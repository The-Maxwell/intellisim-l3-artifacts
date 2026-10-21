package simulations

import (
	"fmt"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// ValidateInstance validates a simulation instance against its ModelSnapshot.
// ModelSnapshot is the authority for all static definitions. The result contains
// blocking Errors and advisory Warnings. StartReady is true when there are no
// errors AND the simulation has at least one agent.
func ValidateInstance(sim datasims.Simulation) ValidationResult {
	var res ValidationResult
	addErr := func(path, reason string) {
		res.Errors = append(res.Errors, ValidationIssue{Path: path, Reason: reason})
	}
	addWarn := func(path, reason string) {
		res.Warnings = append(res.Warnings, ValidationIssue{Path: path, Reason: reason})
	}

	if sim.ModelSnapshot == nil {
		addErr("model_snapshot", "missing model snapshot")
		res.StartReady = false
		return res
	}
	snap := sim.ModelSnapshot

	// U2: the optional termination condition must stay within the restricted
	// subset and be enforceable (operator + value present, etc.).
	if sim.Termination != nil {
		validateTerminationCondition(addErr, "termination", *sim.Termination)
	}

	// Build lookup maps from the snapshot once.
	agentsByUID := make(map[string]datamodels.Agent, len(snap.Agents))
	propsByAgentUID := make(map[string]map[string]datamodels.AgentProperty)
	for _, a := range snap.Agents {
		agentsByUID[a.UID] = a
		pm := make(map[string]datamodels.AgentProperty, len(a.Properties))
		for _, pr := range a.Properties {
			pm[pr.UID] = pr
		}
		propsByAgentUID[a.UID] = pm
	}

	shapesByUID := make(map[string]datamodels.Shape)
	if snap.Environment != nil {
		for _, sh := range snap.Environment.Shapes {
			shapesByUID[sh.UID] = sh
		}
	}

	globalPropsByUID := make(map[string]datamodels.AgentProperty)
	if snap.Environment != nil {
		for _, pr := range snap.Environment.GlobalProperties {
			globalPropsByUID[pr.UID] = pr
		}
	}

	// ── Steps ──────────────────────────────────────────────────────────────
	if sim.Steps < 1 {
		addErr("steps", "steps must be at least 1")
	}

	// ── Agents ─────────────────────────────────────────────────────────────
	seenIDs := make(map[uint64]int) // instanceID → first index
	countByUID := make(map[string]int)
	unplaced := 0 // spatial agents awaiting manual placement (nil position)

	for i, ag := range sim.Agents {
		p := fmt.Sprintf("agents[%d]", i)

		// A spatial agent with no position awaits manual placement — the run is
		// blocked until the user places it in the world (see the aggregate error
		// after the loop).
		if snap.IsSpatial && ag.Position == nil {
			unplaced++
		}

		// Unknown archetype
		archetype, known := agentsByUID[ag.AgentUID]
		if !known {
			addErr(p+".agent_uid", fmt.Sprintf("unknown agent uid %q", ag.AgentUID))
		}

		// Duplicate instance ID
		if firstIdx, seen := seenIDs[ag.ID]; seen {
			addErr(p+".id", fmt.Sprintf("duplicate instance id %d (first seen at agents[%d])", ag.ID, firstIdx))
		} else {
			seenIDs[ag.ID] = i
		}

		// Count per archetype (for population bound warning later)
		countByUID[ag.AgentUID]++

		if !known {
			// Can't check state or properties without the archetype; skip.
			// Position warnings are still relevant.
			validateAgentPositionWarnings(addWarn, i, ag, snap, sim.Environment)
			continue
		}

		// State check
		if ag.State != "" && len(archetype.States) > 0 && !containsStr(archetype.States, ag.State) {
			addErr(p+".state", fmt.Sprintf("state %q not in agent states", ag.State))
		}

		// Property type checks
		props := propsByAgentUID[ag.AgentUID]
		for uid, val := range ag.Properties {
			propPath := p + ".properties." + uid
			pr, exists := props[uid]
			if !exists {
				addErr(propPath, fmt.Sprintf("unknown property %q", uid))
				continue
			}
			if typeErr := checkPropertyType(pr, val); typeErr != "" {
				addErr(propPath, typeErr)
				continue
			}
			// Numeric range warning
			if pr.Type == datamodels.PropertyTypeNumber {
				if fv, ok := coerceFloat(val); ok {
					if (pr.MinValue != nil && fv < *pr.MinValue) || (pr.MaxValue != nil && fv > *pr.MaxValue) {
						min := "−∞"
						max := "+∞"
						if pr.MinValue != nil {
							min = formatFloat(*pr.MinValue)
						}
						if pr.MaxValue != nil {
							max = formatFloat(*pr.MaxValue)
						}
						addWarn(propPath, fmt.Sprintf("value %s outside [%s,%s]", formatFloat(fv), min, max))
					}
				}
			}
		}

		// Position warnings
		validateAgentPositionWarnings(addWarn, i, ag, snap, sim.Environment)
	}

	// Unplaced agents block the run: manual-placement instances must be positioned
	// in the world (simulation editor) before starting.
	if unplaced > 0 {
		addErr("agents", fmt.Sprintf(
			"%d agent(s) have no position — place them in the world before running", unplaced))
	}

	// Population count-bound warnings (per archetype).
	// Only warn when at least one bound is set AND the count is outside it.
	for _, a := range snap.Agents {
		cnt := countByUID[a.UID]
		init := a.Initialization
		belowMin := init.MinPopulation != nil && int64(cnt) < int64(*init.MinPopulation)
		aboveMax := init.MaxPopulation != nil && int64(cnt) > int64(*init.MaxPopulation)
		if belowMin || aboveMax {
			minStr := "0"
			maxStr := "+∞"
			if init.MinPopulation != nil {
				minStr = fmt.Sprintf("%d", *init.MinPopulation)
			}
			if init.MaxPopulation != nil {
				maxStr = fmt.Sprintf("%d", *init.MaxPopulation)
			}
			addWarn("agents", fmt.Sprintf("archetype %q has %d instances, outside [%s,%s]",
				a.UID, cnt, minStr, maxStr))
		}
	}

	// ── Environment shapes ─────────────────────────────────────────────────
	if sim.Environment != nil {
		for i, sh := range sim.Environment.Shapes {
			p := fmt.Sprintf("environment.shapes[%d]", i)

			if _, known := shapesByUID[sh.ShapeUID]; !known {
				addErr(p+".shape_uid", fmt.Sprintf("unknown shape uid %q", sh.ShapeUID))
				continue
			}

			// Geometry coherence
			switch sh.Type {
			case datamodels.ShapeTypeCircle:
				if sh.Radius == nil {
					addErr(p, "circle requires radius")
				}
			case datamodels.ShapeTypeRectangle:
				if sh.Width == nil || sh.Height == nil {
					addErr(p, "rectangle requires width and height")
				}
			case datamodels.ShapeTypePolygon:
				if len(sh.Points) < 3 {
					addErr(p, "polygon requires at least 3 points")
				}
			case datamodels.ShapeTypeLine:
				if len(sh.Points) < 2 {
					addErr(p, "line requires at least 2 points")
				}
			}
		}
	}

	// ── Widgets ────────────────────────────────────────────────────────────
	if len(sim.Widgets) == 0 {
		addWarn("widgets", "no widgets configured")
	} else {
		// Build agentProps map (agentUID → set of property UIDs) for referential checks.
		agentProps := make(map[string]map[string]bool, len(snap.Agents))
		for _, a := range snap.Agents {
			set := make(map[string]bool, len(a.Properties))
			for _, pr := range a.Properties {
				set[pr.UID] = true
			}
			agentProps[a.UID] = set
		}
		globalProps := make(map[string]bool, len(globalPropsByUID))
		for uid := range globalPropsByUID {
			globalProps[uid] = true
		}

		for i, w := range sim.Widgets {
			wp := fmt.Sprintf("widgets[%d]", i)

			// Structural checks via common.ValidateWidget.
			for _, wi := range common.ValidateWidget(i, w) {
				res.Errors = append(res.Errors, ValidationIssue{Path: wi.Path, Reason: wi.Reason})
			}

			// Referential checks on source and series sources.
			if w.Source != nil {
				validateSimWidgetSourceRefs(&res, wp+".source", *w.Source, agentProps, globalProps)
			}
			for j, s := range w.Series {
				validateSimWidgetSourceRefs(&res, fmt.Sprintf("%s.series[%d].source", wp, j), s.Source, agentProps, globalProps)
			}

			// agent_table column validation.
			if w.Type == common.WidgetAgentTable && w.Source != nil && len(w.Source.AgentUIDs) == 1 {
				if set, ok := agentProps[w.Source.AgentUIDs[0]]; ok {
					for j, col := range w.Columns {
						if !set[col] {
							res.Errors = append(res.Errors, ValidationIssue{
								Path:   fmt.Sprintf("%s.columns[%d]", wp, j),
								Reason: fmt.Sprintf("column %q is not a property uid of agent %q", col, w.Source.AgentUIDs[0]),
							})
						}
					}
				}
			}
		}
	}

	res.StartReady = len(res.Errors) == 0 && len(sim.Agents) > 0
	return res
}

// validateAgentPositionWarnings emits position-related warnings for agent i.
// The unplaced (nil position) case is handled as a blocking error in
// ValidateInstance (a null-position spatial agent awaits manual placement), so
// this only flags out-of-bounds positions.
func validateAgentPositionWarnings(
	addWarn func(path, reason string),
	i int,
	ag datasims.SimulationAgent,
	snap *datamodels.Model,
	env *datasims.SimulationEnvironment,
) {
	p := fmt.Sprintf("agents[%d]", i)

	if ag.Position != nil && env != nil {
		if ag.Position.X >= env.Width || ag.Position.Y >= env.Height {
			addWarn(p+".position", "position outside world bounds")
		}
	}
}

// checkPropertyType validates a runtime value against an archetype property's
// declared type. Returns an error reason string, or "" if the value is valid.
func checkPropertyType(pr datamodels.AgentProperty, val interface{}) string {
	switch pr.Type {
	case datamodels.PropertyTypeNumber:
		if _, ok := coerceFloat(val); !ok {
			return "expected number"
		}
	case datamodels.PropertyTypeBoolean:
		if _, ok := val.(bool); !ok {
			return "expected boolean"
		}
	case datamodels.PropertyTypeString:
		if _, ok := val.(string); !ok {
			return "expected string"
		}
	case datamodels.PropertyTypeColor:
		s, ok := val.(string)
		if !ok {
			return "expected string"
		}
		if !datamodels.IsHexColor(s) {
			return "expected hex colour like #RRGGBB"
		}
	case datamodels.PropertyTypeEnum:
		s, ok := val.(string)
		if !ok {
			return "expected enum string"
		}
		if len(pr.AllowedValues) > 0 && !enumValueAllowed(s, pr.AllowedValues) {
			return fmt.Sprintf("value %q not an allowed enum value", s)
		}
	}
	return ""
}

// enumValueAllowed checks if s is present in allowed using strict string
// comparison first, then a numeric fallback for JSON/BSON round-trips. It
// validates already-stored, type-coerced values — unlike enumAllowed in
// llmmapper.go, which loosely accepts any fmt.Sprintf representation to handle
// raw LLM JSON output where the caller cannot guarantee the value type.
func enumValueAllowed(s string, allowed []interface{}) bool {
	for _, av := range allowed {
		if str, ok := av.(string); ok && str == s {
			return true
		}
		// Numeric tolerance for JSON/BSON round-trips (unlikely for enum but safe).
		if fa, ok := coerceFloat(av); ok {
			if fv, ok2 := coerceFloat(s); ok2 && fa == fv {
				return true
			}
		}
	}
	return false
}

// validateSimWidgetSourceRefs checks one widget source against snapshot archetypes
// and properties. path already points at the source (e.g. "widgets[1].source").
// Mirrors models.validateWidgetSourceRefs but reads from the snapshot's lookup maps.
func validateSimWidgetSourceRefs(
	res *ValidationResult,
	path string,
	src common.WidgetSource,
	agentProps map[string]map[string]bool,
	globalProps map[string]bool,
) {
	addErr := func(suffix, reason string) {
		res.Errors = append(res.Errors, ValidationIssue{Path: path + suffix, Reason: reason})
	}

	for j, uid := range src.AgentUIDs {
		if _, ok := agentProps[uid]; !ok {
			addErr(fmt.Sprintf(".agent_uids[%d]", j), fmt.Sprintf("unknown agent uid %q", uid))
		}
	}

	switch src.Type {
	case common.WidgetSourceAgentProperty:
		if src.PropertyUID != "" && !simWidgetPropertyResolvable(src.PropertyUID, src.AgentUIDs, agentProps) {
			addErr(".property_uid", fmt.Sprintf("property %q does not exist on the referenced agents", src.PropertyUID))
		}
	case common.WidgetSourceGlobalProperty:
		if src.PropertyUID != "" && !globalProps[src.PropertyUID] {
			addErr(".property_uid", fmt.Sprintf("global property %q does not exist in environment.global_properties", src.PropertyUID))
		}
	// sim_metric intentionally has no referential checks — it reads built-in
	// engine run counters, not snapshot archetypes/properties.
	}
}

// simWidgetPropertyResolvable reports whether propUID exists on at least one of
// the referenced agents (or on ANY agent when agentUIDs is empty). When every
// referenced uid is unknown the function returns true — the unknown uids are
// already flagged and a property issue would only cascade noise.
func simWidgetPropertyResolvable(propUID string, agentUIDs []string, agentProps map[string]map[string]bool) bool {
	if len(agentUIDs) == 0 {
		for _, set := range agentProps {
			if set[propUID] {
				return true
			}
		}
		return false
	}
	anyKnown := false
	for _, uid := range agentUIDs {
		set, ok := agentProps[uid]
		if !ok {
			continue
		}
		anyKnown = true
		if set[propUID] {
			return true
		}
	}
	return !anyKnown
}

// containsStr reports whether target is present in ss.
func containsStr(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

// formatFloat formats a float64 without unnecessary trailing zeros.
func formatFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}
