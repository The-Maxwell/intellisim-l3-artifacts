package simulations

import (
	"encoding/json"
	"fmt"
	"strings"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// individualFullStrategy is the Strategy-2 (full × agent) execution strategy: for
// each enabled purpose it asks the LLM to choose concrete actions PER AGENT,
// optionally batching ≤ Strategy.BatchSize agents into one call. Full control maps
// the returned actions onto the engine's ActionIntent vocabulary (validated via
// MapFullActions). Capability: ModelCapabilitySimulationExecution.
type individualFullStrategy struct{}

func (individualFullStrategy) Name() string { return "individual_full" }

func (individualFullStrategy) Plan(sc *StepContext, group *AgentGroup) (PlanResult, error) {
	res := PlanResult{Intents: map[uint64]*AgentIntent{}}
	if sc == nil || sc.RS == nil || group == nil || sc.LLM == nil {
		return res, nil
	}

	purposes := enabledPurposes(group.Strategy)
	if len(purposes) == 0 {
		return res, nil
	}

	cap := strategyCapability(group.Strategy)
	for _, p := range purposes {
		members := group.Members
		if p == purposeDecision {
			// The decision purpose honors on-demand call_llm requests: off-schedule
			// steps include only the pending members (flags consumed either way).
			members = decisionDueMembers(sc.RS.Step, group.Strategy.DecisionFrequency, group.Members)
		} else if !purposeDue(sc.RS.Step, group.Strategy.DecisionFrequency) {
			// Skip this purpose this step; the prior step's intents are reused by
			// the engine between scheduled calls (handled at the run-loop level).
			continue
		}
		if len(members) == 0 {
			continue
		}
		planFullPurpose(sc, group, p, cap, members, &res)
	}
	return res, nil
}

// strategyCapability maps the strategy TYPE to the provider-validated model
// capability used for its calls (the run-start validation pre-checks the same
// capability). full → SimulationExecution; decision/hybrid-LLM →
// SimulationDecisions. This is intentionally SEPARATE from the per-purpose token
// attribution (movement/properties/decision/interaction).
func strategyCapability(s models.LLMStrategy) common.ModelCapability {
	if s.Type == models.LLMStrategyFull {
		return common.ModelCapabilitySimulationExecution
	}
	return common.ModelCapabilitySimulationDecisions
}

// planFullPurpose runs one purpose across the given members (normally the whole
// group; a pending-only subset for off-schedule call_llm decisions) in
// BatchSize-sized batches, demuxing the full-control / property / movement
// response by instance index into per-agent intents. capability is the
// provider-validated capability for the calls (kept separate from the
// per-purpose token attribution).
func planFullPurpose(sc *StepContext, group *AgentGroup, p purpose, capability common.ModelCapability, members []*RuntimeAgent, res *PlanResult) {
	rs := sc.RS
	for _, batch := range chunkMembers(members, group.Strategy.BatchSize) {
		system, user := buildIndividualPrompt(rs, group, p, batch)
		fb := resolveModelFallback(rs.Model, group.Archetype)

		switch p {
		case purposeDecision:
			var out fullDecisionResp
			if !sc.LLM.callJSON(sc.Ctx, group.Target, capability,
				datasims.LLMUsageSimulationDecision, group.ArchetypeUID, system, user, fb, &out) {
				continue // rules cover this purpose for the batch
			}
			demuxFullActions(rs, batch, out.Agents, res)
		case purposeProperties:
			var out propsResp
			if !sc.LLM.callJSON(sc.Ctx, group.Target, capability,
				datasims.LLMUsageSimulationProperties, group.ArchetypeUID, system, user, fb, &out) {
				continue
			}
			demuxProperties(batch, out, res)
		case purposeMovement:
			var out movementResp
			if !sc.LLM.callJSON(sc.Ctx, group.Target, capability,
				datasims.LLMUsageSimulationMovement, group.ArchetypeUID, system, user, fb, &out) {
				continue
			}
			demuxMovement(batch, out, res)
		case purposeInteraction:
			// Interaction participation is mediated in the interactions phase
			// (MediateInteraction); the strategy issues no separate batch call.
			continue
		}
	}
}

// fullDecisionResp is the full-control response: per-instance concrete actions.
type fullDecisionResp struct {
	Agents []struct {
		Instance int         `json:"instance"`
		Actions  []rawAction `json:"actions"`
	} `json:"agents"`
}

// demuxFullActions maps each instance's raw full-control actions onto validated
// ActionIntents, merged into the batch member at that index. It also sets
// intent.LLMNote to a compact summary of the action types chosen (e.g.
// "actions: set_property,move") so the mutation phase can flush it to
// AgentMemoryLLM for future prompt feedback.
//
// Task 17: emit_event ActionIntents (validated + authorized by MapFullActions /
// validateRawAction) are extracted from the mapped list and appended to
// intent.Emits rather than intent.Actions. This ensures ApplyIntentActions routes
// them through applyEmitEvent(..., EventProducerLLM) rather than the rule-engine
// applyEmitEvent(..., EventProducerAgent) path.
func demuxFullActions(rs *RuntimeState, batch []*RuntimeAgent, agents []struct {
	Instance int         `json:"instance"`
	Actions  []rawAction `json:"actions"`
}, res *PlanResult) {
	for _, a := range agents {
		ra := memberAt(batch, a.Instance)
		if ra == nil {
			continue
		}
		intent := ensureIntent(res, ra.A.ID)
		mapped := MapFullActions(rs, ra, a.Actions)
		// Partition: emit_event → intent.Emits; everything else → intent.Actions.
		for _, ai := range mapped {
			if ai.Type == models.ActionTypeEmitEvent {
				intent.Emits = append(intent.Emits, EmitIntent{EventUID: ai.EventUID, Data: ai.EventData})
			} else {
				intent.Actions = append(intent.Actions, ai)
			}
		}
		// Derive a compact LLM-response note from the decoded (validated) actions +
		// emits so the mutation phase can write it to AgentMemoryLLM. We never thread raw text.
		allMapped := mapped
		if len(allMapped) > 0 {
			types := make([]string, 0, len(allMapped))
			seen := map[string]bool{}
			for _, ai := range allMapped {
				s := string(ai.Type)
				if !seen[s] {
					seen[s] = true
					types = append(types, s)
				}
			}
			intent.LLMNote = "actions: " + strings.Join(types, ",")
		}
	}
}

// demuxProperties merges per-instance property directives into the batch intents.
// Each values map is filtered to the agent's DECLARED, MUTABLE property UIDs first
// (the prompt's "unknown or immutable UIDs are dropped" promise, enforced at the
// LLM seam); agents whose map filters to nothing get no properties intent.
func demuxProperties(batch []*RuntimeAgent, out propsResp, res *PlanResult) {
	for _, a := range out.Agents {
		ra := memberAt(batch, a.Instance)
		if ra == nil || len(a.Values) == 0 {
			continue
		}
		vals := filterSettableValues(ra.Archetype, a.Values)
		if len(vals) == 0 {
			continue
		}
		intent := ensureIntent(res, ra.A.ID)
		if intent.Properties == nil {
			intent.Properties = make(map[string]any, len(vals))
		}
		for k, v := range vals {
			intent.Properties[k] = v
		}
	}
}

// demuxMovement merges per-instance movement directives into the batch intents.
func demuxMovement(batch []*RuntimeAgent, out movementResp, res *PlanResult) {
	for _, a := range out.Agents {
		ra := memberAt(batch, a.Instance)
		if ra == nil {
			continue
		}
		intent := ensureIntent(res, ra.A.ID)
		intent.Movement = &MovementIntent{Intent: a.Intent, TargetRef: a.TargetRef, Speed: a.Speed}
	}
}

// --- shared individual-strategy helpers (used by both individual strategies) ---

// enabledPurposes returns the per-step purposes enabled by the strategy flags, in
// a fixed order (decision, properties, movement, interaction). UseForDecisions
// gates the decision purpose, etc.
func enabledPurposes(s models.LLMStrategy) []purpose {
	var out []purpose
	if s.UseForDecisions {
		out = append(out, purposeDecision)
	}
	if s.UseForProperties {
		out = append(out, purposeProperties)
	}
	if s.UseForMovement {
		out = append(out, purposeMovement)
	}
	if s.UseForInteraction {
		out = append(out, purposeInteraction)
	}
	return out
}

// purposeDue reports whether a purpose runs on this step given the strategy's
// DecisionFrequency: run when step % max(1,freq) == 0, else skip (reuse prior).
func purposeDue(step uint64, freq int) bool {
	f := freq
	if f < 1 {
		f = 1
	}
	return step%uint64(f) == 0
}

// decisionDueMembers returns the members participating in this step's
// decision-purpose call and CONSUMES their pending call_llm flags:
//   - frequency due  → all members (a scheduled call satisfies pending requests);
//   - frequency idle → only members with a pending on-demand request.
//
// Flags are cleared for every returned member regardless of the eventual call
// outcome (consume-once, fail-soft — LastDecision replay / rules cover a failed
// call). Mutating members here is safe under the parallel LLM gather for the
// same reason ra.LastDecision writes are: group member sets are disjoint and the
// results merge is serial.
func decisionDueMembers(step uint64, freq int, members []*RuntimeAgent) []*RuntimeAgent {
	if purposeDue(step, freq) {
		for _, ra := range members {
			if ra != nil {
				ra.llmCallPending = false
			}
		}
		return members
	}
	var pending []*RuntimeAgent
	for _, ra := range members {
		if ra != nil && ra.llmCallPending {
			ra.llmCallPending = false
			pending = append(pending, ra)
		}
	}
	return pending
}

// chunkMembers splits members into batches of at most max(1,size). Removed
// members are skipped so a batch never carries a stale agent.
func chunkMembers(members []*RuntimeAgent, size int) [][]*RuntimeAgent {
	n := size
	if n < 1 {
		n = 1
	}
	live := make([]*RuntimeAgent, 0, len(members))
	for _, ra := range members {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		live = append(live, ra)
	}
	var out [][]*RuntimeAgent
	for i := 0; i < len(live); i += n {
		end := i + n
		if end > len(live) {
			end = len(live)
		}
		out = append(out, live[i:end])
	}
	return out
}

// memberAt returns the batch member at the given instance index, or nil when out
// of range. The instance index is the agent's position WITHIN the batch (the
// contract the LLM is told to use).
func memberAt(batch []*RuntimeAgent, instance int) *RuntimeAgent {
	if instance < 0 || instance >= len(batch) {
		return nil
	}
	return batch[instance]
}

// ensureIntent returns the (created-on-demand) AgentIntent for an agent id in res.
func ensureIntent(res *PlanResult, id uint64) *AgentIntent {
	if in, ok := res.Intents[id]; ok {
		return in
	}
	in := &AgentIntent{}
	res.Intents[id] = in
	return in
}

// buildIndividualPrompt assembles the system + user messages for one individual
// batch call. The user message enumerates each batch member by its 0-based
// instance index (the demux key) along with its state/position/properties, then
// appends the shared mandatory-hint context + memory summaries.
func buildIndividualPrompt(rs *RuntimeState, group *AgentGroup, p purpose, batch []*RuntimeAgent) (string, string) {
	var menu []string
	if p == purposeDecision && group.Strategy.Type == models.LLMStrategyDecision {
		menu = DecisionMenuWithHints(rs.Model, group.ArchetypeUID)
	}
	var emitLabels []string
	if p == purposeDecision {
		emitLabels = llmEmittableEventLabels(rs.Model, group.ArchetypeUID)
	}

	var sys strings.Builder
	sys.WriteString(runSystemHeader)
	sys.WriteString(purposeSystemSpec(p, group.Strategy, menu, emitLabels))

	var user strings.Builder
	user.WriteString(buildSharedRuntimeBlocks(rs))
	fmt.Fprintf(&user, "Archetype %q has %d agent(s) acting this step:\n", group.ArchetypeUID, len(batch))
	for i, ra := range batch {
		writeAgentLine(&user, i, ra)
	}

	if wantsPropertySchema(p, group.Strategy) {
		if lines := settablePropertyLines(group.Archetype); lines != "" {
			user.WriteString("Properties (settable; values are clamped to bounds):\n")
			user.WriteString(lines)
		}
	}

	if ctxText := buildContext(rs, group, batch); ctxText != "" {
		user.WriteString(ctxText)
		user.WriteByte('\n')
	}
	return sys.String(), user.String()
}

// writeAgentLine appends one agent's compact state to the prompt under its
// instance index.
func writeAgentLine(b *strings.Builder, instance int, ra *RuntimeAgent) {
	if ra == nil || ra.A == nil {
		return
	}
	fmt.Fprintf(b, "- instance %d (id %d) state=%q", instance, ra.A.ID, ra.A.State)
	if ra.A.Position != nil {
		fmt.Fprintf(b, " pos=(%d,%d)", ra.A.Position.X, ra.A.Position.Y)
	}
	if len(ra.A.Properties) > 0 {
		// json.Marshal sorts map keys — canonical, unambiguous, byte-stable.
		if props, err := json.Marshal(ra.A.Properties); err == nil {
			fmt.Fprintf(b, " properties=%s", props)
		}
	}
	b.WriteByte('\n')
}

// settablePropertyLines renders one propSchemaLine per DECLARED mutable
// property of the archetype (immutable properties are excluded — the engine
// drops writes to them anyway). Returns "" when nothing is settable.
func settablePropertyLines(arch *models.Agent) string {
	if arch == nil {
		return ""
	}
	var b strings.Builder
	for _, p := range arch.Properties {
		if p.Immutable {
			continue
		}
		fmt.Fprintf(&b, "- %s\n", propSchemaLine(p))
	}
	return b.String()
}

// filterSettableValues restricts an LLM-returned property-values map to the
// archetype's DECLARED, MUTABLE property UIDs — the runtime property prompts
// promise "unknown or immutable UIDs are dropped", and this enforces it at the
// LLM seam (rule-engine property writes keep their own semantics untouched).
// Iteration follows the declared property order (deterministic); returns nil
// when nothing survives so callers can skip empty directives.
func filterSettableValues(arch *models.Agent, values map[string]any) map[string]any {
	if arch == nil || len(values) == 0 {
		return nil
	}
	var out map[string]any
	for i := range arch.Properties {
		p := &arch.Properties[i]
		if p.Immutable {
			continue
		}
		v, ok := values[p.UID]
		if !ok {
			continue
		}
		if out == nil {
			out = make(map[string]any, len(values))
		}
		out[p.UID] = v
	}
	return out
}

// wantsPropertySchema reports whether a purpose's prompt should carry the
// settable-property schema: the properties purpose writes values directly, and
// the FULL-kind decision purpose may emit set_property/modify_property actions.
func wantsPropertySchema(p purpose, strat models.LLMStrategy) bool {
	if p == purposeProperties {
		return true
	}
	return p == purposeDecision && strat.Type == models.LLMStrategyFull
}

// purposeSystemSpec returns the per-purpose system-prompt fragment (role + strict
// JSON output contract). The full strategies request a concrete-action contract
// for the decision purpose; decision strategies request a label from the menu.
// emitLabels (may be nil) advertises the LLM-emittable events — rendered by
// llmEmittableEventLabels (UID + optional prompt context) — on both the decision
// and full-control decision specs.
func purposeSystemSpec(p purpose, strat models.LLMStrategy, menu []string, emitLabels []string) string {
	switch p {
	case purposeProperties:
		return "\nTask: update agent property values.\n" +
			"Only the DECLARED settable property UIDs are accepted; values are clamped to the property's bounds; unknown or immutable UIDs are dropped.\n" +
			"Schema: {\"agents\":[{\"instance\":<idx>,\"values\":{\"<propUID>\":<value>}}]}\n" +
			"Example: {\"agents\":[{\"instance\":0,\"values\":{\"energy\":42}}]}\n"
	case purposeMovement:
		return "\nTask: choose each agent's movement.\n" +
			"target_ref: an archetype or shape uid (each agent resolves the NEAREST matching instance), a literal \"x,y\" point, or empty (nearest agent of any archetype).\n" +
			"speed is 0..1 (fraction of the agent's max per-step distance). intent hold = stay; wander = random direction.\n" +
			"Schema: {\"agents\":[{\"instance\":<idx>,\"intent\":\"toward|away|wander|hold\",\"target_ref\":\"<archetypeUID|shapeUID|x,y>\",\"speed\":<0..1>}]}\n" +
			"Example: {\"agents\":[{\"instance\":0,\"intent\":\"toward\",\"target_ref\":\"lake\",\"speed\":0.8}]}\n"
	case purposeDecision:
		if strat.Type == models.LLMStrategyDecision {
			spec := "\nTask: choose one decision label per agent."
			if len(menu) > 0 {
				spec += " Allowed labels: " + strings.Join(menu, ", ") + "."
			} else {
				spec += " You may return a free label."
			}
			spec += "\nSchema: {\"agents\":[{\"instance\":<idx>,\"label\":\"<label>\",\"params\":{}}]}\n"
			if len(emitLabels) > 0 {
				spec += "Optionally add \"emit_uid\":\"<uid>\" (+ \"emit_data\":<payload>) to an agent entry to emit one of: " +
					strings.Join(emitLabels, ", ") + ".\n"
			}
			spec += "Example: {\"agents\":[{\"instance\":0,\"label\":\"forage\",\"params\":{}}]}\n"
			return spec
		}
		// full × agent: concrete actions per agent, including LLM-emitted events.
		// emit_event is only valid when the EventDefinition's Producers include "llm"
		// and (if ProducerUIDs is non-empty) the agent's archetype UID is listed.
		spec := "\nTask: choose concrete actions per agent.\n" +
			"Allowed action types: set_property, modify_property, move, move_toward, move_away, interact, transition, emit_event.\n" +
			"For emit_event: {\"type\":\"emit_event\",\"event_uid\":\"<uid>\",\"event_data\":<payload|null>}\n" +
			"emit_event is only valid for events whose Producers include \"llm\"; unauthorized emits are silently dropped.\n"
		if len(emitLabels) > 0 {
			spec += "Emittable events: " + strings.Join(emitLabels, ", ") + ".\n"
		}
		spec += "set_property/modify_property may target a DECLARED environment global property by adding \"target_uid\":\"environment\".\n" +
			"move_toward/move_away target_uid follows the movement target_ref semantics (archetype/shape uid → nearest instance).\n" +
			"Schema: {\"agents\":[{\"instance\":<idx>,\"actions\":[{\"type\":\"<type>\",\"property_uid\":\"...\",\"value\":...,\"modifier\":\"set|add|subtract|multiply|divide|increment|decrement|toggle\",\"target_uid\":\"...\",\"speed\":<0..1>,\"new_state\":\"...\",\"event_uid\":\"...\",\"event_data\":...}]}]}\n" +
			"Example: {\"agents\":[{\"instance\":0,\"actions\":[{\"type\":\"move_toward\",\"target_uid\":\"lake\",\"speed\":0.7},{\"type\":\"modify_property\",\"property_uid\":\"energy\",\"modifier\":\"subtract\",\"value\":2}]}]}\n"
		return spec
	default:
		return ""
	}
}
