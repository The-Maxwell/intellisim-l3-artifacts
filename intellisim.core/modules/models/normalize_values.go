package models

import (
	"strconv"
	"strings"

	datamodels "intelli.sim.core/data/models"
)

// NormalizeRuleValues coerces numeric-looking string Condition/Action values
// ("1") to float64 on save, mirroring what a numeric API/LLM submission would
// already look like. It exists because modules/simulations' engine coerceFloat
// does not parse strings, so a UI-submitted string on a numeric property
// silently fails numeric compares/writes (ISS-06).
//
// Resolution: a condition/action's referenced property is looked up by
// PropertyUID, scoped to the archetypes the OWNING rule/interaction actually
// applies to (see propertyTypeIndex.scopeArchetypes) — plus the environment's
// global properties (an action whose TargetUID is the reserved EnvironmentUID,
// or a global_property condition, resolves against globals only, regardless of
// scope). Two archetypes may declare the same PropertyUID with different
// types; scoping prevents a rule/interaction from being coerced per an
// archetype it does not actually apply to. When the scoped archetypes
// DISAGREE on the property's declared type, resolution is treated as failed
// and the value is left untouched (never guess). When the property resolves
// (unambiguously) to a declared NUMBER type, a numeric-looking string Value is
// coerced to float64; a non-numeric declared type (string/color/enum/boolean)
// is left untouched. When no property can be resolved at all (e.g.
// random/step conditions have no PropertyUID), only those two
// numeric-semantic condition types are coerced — every other unresolved case
// (decision/state labels, count/proximity/memory values, etc.) is left
// exactly as submitted.
//
// Composite sub-conditions are normalized recursively. Also normalizes
// Model.Interactions (Conditions + InitiatorActions + TargetActions): scope is
// InitiatorUIDs for initiator_actions, TargetUIDs for target_actions, and the
// union of both for conditions (same conflict->skip rule; see
// propertyTypeIndex.interactionConditionScope). Mutates m in place; called at
// every model save site alongside NormalizeLLMStrategies / NormalizeSpatial.
func NormalizeRuleValues(m *datamodels.Model) {
	if m == nil {
		return
	}
	idx := buildPropertyTypeIndex(m)
	for i := range m.Rules {
		scope := idx.scopeArchetypes(m.Rules[i].AppliesTo)
		normalizeConditionValues(m.Rules[i].Conditions, idx, scope)
		normalizeActionValues(m.Rules[i].Actions, idx, scope)
	}
	for i := range m.Interactions {
		in := &m.Interactions[i]
		condScope := idx.interactionConditionScope(*in)
		normalizeConditionValues(in.Conditions, idx, condScope)
		normalizeActionValues(in.InitiatorActions, idx, idx.scopeArchetypes(in.InitiatorUIDs))
		normalizeActionValues(in.TargetActions, idx, idx.scopeArchetypes(in.TargetUIDs))
	}
}

// propertyTypeIndex indexes declared property types per archetype (NOT merged
// globally — two archetypes may declare the same PropertyUID with different
// types) plus a separate global (environment) namespace, since an action/
// condition targeting the environment resolves against globals only.
type propertyTypeIndex struct {
	// perArchetype maps an archetype/agent UID to its own PropertyUID -> type.
	perArchetype map[string]map[string]datamodels.AgentPropertyType
	// global maps environment GlobalProperty UID -> type.
	global map[string]datamodels.AgentPropertyType
	// allArchetypeUIDs lists every agent UID declared in the model, used to
	// resolve an empty applies_to/UIDs scope ("applies to every archetype").
	allArchetypeUIDs []string
}

func buildPropertyTypeIndex(m *datamodels.Model) propertyTypeIndex {
	idx := propertyTypeIndex{
		perArchetype: make(map[string]map[string]datamodels.AgentPropertyType, len(m.Agents)),
		global:       map[string]datamodels.AgentPropertyType{},
	}
	for _, a := range m.Agents {
		props := make(map[string]datamodels.AgentPropertyType, len(a.Properties))
		for _, p := range a.Properties {
			if _, exists := props[p.UID]; !exists {
				props[p.UID] = p.Type
			}
		}
		idx.perArchetype[a.UID] = props
		idx.allArchetypeUIDs = append(idx.allArchetypeUIDs, a.UID)
	}
	if m.Environment != nil {
		for _, p := range m.Environment.GlobalProperties {
			if _, exists := idx.global[p.UID]; !exists {
				idx.global[p.UID] = p.Type
			}
		}
	}
	return idx
}

// scopeArchetypes translates a raw applies_to/InitiatorUIDs/TargetUIDs list
// into the archetype UIDs to resolve non-global PropertyUID types against: the
// list itself (dropping the reserved EnvironmentUID sentinel, which is handled
// separately via the global namespace), or every declared archetype when the
// list is empty — mirroring the engine's "empty applies_to/UID-set matches
// everything" semantics (see ruleAppliesToUID / matchesUIDSet in
// modules/simulations).
func (idx propertyTypeIndex) scopeArchetypes(uids []string) []string {
	if len(uids) == 0 {
		return idx.allArchetypeUIDs
	}
	scope := make([]string, 0, len(uids))
	for _, uid := range uids {
		if uid == datamodels.EnvironmentUID {
			continue
		}
		scope = append(scope, uid)
	}
	return scope
}

// interactionConditionScope computes the archetype scope for an interaction's
// Conditions: the union of its initiator and target archetype scopes. Either
// side being empty (matches "any agent", see matchesUIDSet) makes the whole
// union "any agent" too.
func (idx propertyTypeIndex) interactionConditionScope(in datamodels.Interaction) []string {
	if len(in.InitiatorUIDs) == 0 || len(in.TargetUIDs) == 0 {
		return idx.allArchetypeUIDs
	}
	union := make([]string, 0, len(in.InitiatorUIDs)+len(in.TargetUIDs))
	seen := make(map[string]struct{}, cap(union))
	add := func(uids []string) {
		for _, uid := range uids {
			if uid == datamodels.EnvironmentUID {
				continue
			}
			if _, dup := seen[uid]; dup {
				continue
			}
			seen[uid] = struct{}{}
			union = append(union, uid)
		}
	}
	add(in.InitiatorUIDs)
	add(in.TargetUIDs)
	return union
}

// resolveScoped looks up propertyUID's declared type across only the given
// archetype UIDs. It reports unresolved when no archetype in scope declares
// the property, OR when the archetypes in scope disagree on its type (an
// applies_to/UID-set spanning archetypes whose declared types conflict is
// left uncoerced — safety over guessing).
func (idx propertyTypeIndex) resolveScoped(scope []string, propertyUID string) (datamodels.AgentPropertyType, bool) {
	var found datamodels.AgentPropertyType
	seen := false
	for _, uid := range scope {
		props, ok := idx.perArchetype[uid]
		if !ok {
			continue
		}
		t, ok := props[propertyUID]
		if !ok {
			continue
		}
		if seen && t != found {
			return "", false
		}
		found, seen = t, true
	}
	return found, seen
}

// resolveCondition returns the declared property type a condition references
// (global_property looks up globals only; every other PropertyUID-bearing type
// checks the scoped archetypes, falling back to globals), and whether one
// resolved.
//
// event and memory conditions are deliberately treated as unresolvable: their
// PropertyUID is NOT a property reference, so resolving it against declared
// property types would wrongly coerce their Value. For ConditionTypeEvent,
// PropertyUID is an event-UID fallback (see eventConditionUID in
// modules/simulations/rules.go) and Value is an event payload label compared
// against the event's Data (matchEvent); a payload UID that happens to collide
// with a declared numeric property UID must not turn a string label into a
// float. For ConditionTypeMemory, evalMemory ignores PropertyUID entirely and
// matches Value (a memory summary/label) via MemoryMatch — property resolution
// genuinely does not apply. Their Value types are event/memory-defined and are
// left exactly as submitted.
func (idx propertyTypeIndex) resolveCondition(c datamodels.Condition, scope []string) (datamodels.AgentPropertyType, bool) {
	if c.Type == datamodels.ConditionTypeEvent || c.Type == datamodels.ConditionTypeMemory {
		return "", false
	}
	if c.PropertyUID == "" {
		return "", false
	}
	if c.Type == datamodels.ConditionTypeGlobalProperty {
		t, ok := idx.global[c.PropertyUID]
		return t, ok
	}
	if t, ok := idx.resolveScoped(scope, c.PropertyUID); ok {
		return t, true
	}
	t, ok := idx.global[c.PropertyUID]
	return t, ok
}

// resolveAction returns the declared property type an action references
// (TargetUID == "environment" resolves against globals only), and whether one
// resolved.
func (idx propertyTypeIndex) resolveAction(a datamodels.Action, scope []string) (datamodels.AgentPropertyType, bool) {
	if a.PropertyUID == "" {
		return "", false
	}
	if a.TargetUID == datamodels.EnvironmentUID {
		t, ok := idx.global[a.PropertyUID]
		return t, ok
	}
	if t, ok := idx.resolveScoped(scope, a.PropertyUID); ok {
		return t, true
	}
	t, ok := idx.global[a.PropertyUID]
	return t, ok
}

// numericSemanticConditionType reports whether a condition type's Value is
// always numeric regardless of any resolvable property (used only as the
// no-property-resolved fallback — random/step have no PropertyUID to resolve
// in the first place).
func numericSemanticConditionType(t datamodels.ConditionType) bool {
	switch t {
	case datamodels.ConditionTypeRandom, datamodels.ConditionTypeStep:
		return true
	}
	return false
}

// coerceNumericString parses a numeric-looking string into float64, leaving
// every other value (including a non-numeric string) untouched.
func coerceNumericString(v interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return v
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return v
	}
	return f
}

func normalizeConditionValues(conds []datamodels.Condition, idx propertyTypeIndex, scope []string) {
	for i := range conds {
		normalizeOneCondition(&conds[i], idx, scope)
		if len(conds[i].SubConditions) > 0 {
			normalizeConditionValues(conds[i].SubConditions, idx, scope)
		}
	}
}

func normalizeOneCondition(c *datamodels.Condition, idx propertyTypeIndex, scope []string) {
	if t, resolved := idx.resolveCondition(*c, scope); resolved {
		if t == datamodels.PropertyTypeNumber {
			c.Value = coerceNumericString(c.Value)
		}
		return
	}
	if numericSemanticConditionType(c.Type) {
		c.Value = coerceNumericString(c.Value)
	}
}

func normalizeActionValues(actions []datamodels.Action, idx propertyTypeIndex, scope []string) {
	for i := range actions {
		if t, resolved := idx.resolveAction(actions[i], scope); resolved && t == datamodels.PropertyTypeNumber {
			actions[i].Value = coerceNumericString(actions[i].Value)
		}
	}
}
