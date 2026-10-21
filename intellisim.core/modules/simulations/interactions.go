package simulations

import (
	"context"
	"fmt"

	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// ProcessInteractions evaluates every enabled Interaction in the model for this
// step:
//   - agent_agent: for each initiator (UID in InitiatorUIDs), find targets (UID in
//     TargetUIDs) within Radius/InteractionRadius via the index; eval Conditions;
//     apply InitiatorActions to initiator + TargetActions to target (+ reverse if
//     Bidirectional).
//   - agent_shape: targets are SimulationShapes; membership via ShapeContains;
//     InitiatorActions mutate the initiating agent (TargetActions are recorded as
//     intent — shapes carry no agent runtime state to mutate here).
//   - agent_global: initiator vs global properties; InitiatorActions applied to the
//     initiator when its Conditions hold against the environment globals.
//
// UseLLM interactions call llmRT.MediateInteraction(...) to decide whether/how the
// interaction fires (and may supply mediation params). A nil llmRT degrades UseLLM
// interactions to fire when their declared Conditions hold. Every fired interaction
// records an AgentMemoryInteraction event on the agents it mutates.
//
// intents carries the current LLM intents per agent (consumed by ConditionTypeDecision
// inside an interaction's Conditions); it may be nil.
func ProcessInteractions(rs *RuntimeState, llmRT *llmRuntime, intents map[uint64]*AgentIntent) {
	if rs == nil || rs.Model == nil {
		return
	}
	for i := range rs.Model.Interactions {
		in := rs.Model.Interactions[i]
		if !in.Enabled {
			continue
		}
		switch in.Type {
		case models.InteractionTypeAgentAgent:
			processAgentAgent(rs, llmRT, intents, in)
		case models.InteractionTypeAgentShape:
			processAgentShape(rs, llmRT, intents, in)
		case models.InteractionTypeAgentGlobal:
			processAgentGlobal(rs, llmRT, intents, in)
		}
	}

	// Force-fired interactions (interact rule actions / LLM interact intents)
	// drain AFTER the regular pass so legacy ordering is untouched.
	drainForcedInteractions(rs)
}

// drainForcedInteractions consumes rs.forced, force-firing each queued
// interaction for its initiator. Conditions and UseLLM mediation are SKIPPED
// (the firing rule/LLM already made that decision); geometric eligibility is
// kept: agent_agent needs an eligible target within the resolved radius
// (nearest by (BodyDistance, agent id) — an explicit tie-break, the spatial
// index candidate order is not relied upon), agent_shape needs an overlapping
// target shape (first in Env.Shapes order), agent_global always fires.
//
// The loop is index-based so entries appended DURING the drain (an interaction
// whose own action is interact) are also processed; a per-drain dedup set keyed
// initiator|interaction guarantees each pair fires at most once per step, so
// cycles terminate. The queue is reset afterwards.
func drainForcedInteractions(rs *RuntimeState) {
	if len(rs.forced) == 0 {
		return
	}
	fired := make(map[string]struct{}, len(rs.forced))
	for i := 0; i < len(rs.forced); i++ {
		req := rs.forced[i]
		key := fmt.Sprintf("%d|%s", req.initiatorID, req.interactionID)
		if _, dup := fired[key]; dup {
			continue
		}
		fired[key] = struct{}{}

		inp := interactionByID(rs, req.interactionID)
		if inp == nil || !inp.Enabled {
			continue // validated at queue time; defensive
		}
		in := *inp
		initiator := rs.ByID[req.initiatorID]
		if initiator == nil || initiator.A == nil || initiator.removed {
			continue
		}

		switch in.Type {
		case models.InteractionTypeAgentAgent:
			target := nearestForcedTarget(rs, initiator, in)
			if target == nil {
				warnOnce(rs, "interact action: no eligible target in range for interaction '"+in.ID+"'")
				continue
			}
			fireInteractionPair(rs, in, initiator, target)
		case models.InteractionTypeAgentShape:
			sh := firstOverlappingTargetShape(rs, initiator, in)
			if sh == nil {
				warnOnce(rs, "interact action: no overlapping target shape for interaction '"+in.ID+"'")
				continue
			}
			applyInteractionActions(rs, initiator, in.InitiatorActions, in)
			applyShapeActions(sh, in.TargetActions)
		case models.InteractionTypeAgentGlobal:
			applyInteractionActions(rs, initiator, in.InitiatorActions, in)
		default:
			continue
		}
		if rs.stats != nil {
			rs.stats.Interactions++
			rs.stats.ByInteraction[in.ID]++
		}
	}
	rs.forced = rs.forced[:0]
}

// nearestForcedTarget picks the closest live target eligible for the interaction
// (TargetUIDs membership, within the resolved radius), tie-broken by ascending
// agent id for determinism.
func nearestForcedTarget(rs *RuntimeState, initiator *RuntimeAgent, in models.Interaction) *RuntimeAgent {
	if initiator.A.Position == nil {
		return nil
	}
	w, h, torus := envDims(rs)
	qBody := bodyForAgent(initiator)
	var best *RuntimeAgent
	bestDist := 0.0
	for _, cand := range neighborsWithin(rs, initiator, interactionRadius(initiator, in), in.TargetUIDs) {
		if cand == nil || cand.A == nil || cand.removed || cand.A.ID == initiator.A.ID {
			continue
		}
		d := BodyDistance(qBody, bodyForAgent(cand), w, h, torus)
		if best == nil || d < bestDist || (d == bestDist && cand.A.ID < best.A.ID) {
			best = cand
			bestDist = d
		}
	}
	return best
}

// firstOverlappingTargetShape returns the first shape (Env.Shapes order) that
// matches the interaction's TargetUIDs and overlaps the initiator's footprint.
func firstOverlappingTargetShape(rs *RuntimeState, initiator *RuntimeAgent, in models.Interaction) *datasims.SimulationShape {
	if rs.Env == nil || initiator.A.Position == nil {
		return nil
	}
	w, h, torus := envDims(rs)
	for si := range rs.Env.Shapes {
		sh := &rs.Env.Shapes[si]
		if !matchesUIDSet(sh.ShapeUID, in.TargetUIDs) {
			continue
		}
		if BodyOverlapsShape(initiator, sh, w, h, torus) {
			return sh
		}
	}
	return nil
}

// interactionInitiators returns the live runtime agents eligible to initiate the
// interaction (AgentUID in InitiatorUIDs; an empty set matches any agent).
func interactionInitiators(rs *RuntimeState, in models.Interaction) []*RuntimeAgent {
	var out []*RuntimeAgent
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		if matchesUIDSet(ra.A.AgentUID, in.InitiatorUIDs) {
			out = append(out, ra)
		}
	}
	return out
}

// matchesUIDSet reports whether uid is in the set; an empty set matches anything.
func matchesUIDSet(uid string, set []string) bool {
	if len(set) == 0 {
		return true
	}
	for _, u := range set {
		if u == uid {
			return true
		}
	}
	return false
}

// interactionRadius resolves the effective discovery radius: the interaction's
// explicit Radius takes precedence, falling back to the initiator archetype's
// configured InteractionRadius, else 0 (same-position contact only).
func interactionRadius(initiator *RuntimeAgent, in models.Interaction) float64 {
	if in.Radius != nil {
		return *in.Radius
	}
	if initiator.Archetype != nil && initiator.Archetype.LLMConfig != nil &&
		initiator.Archetype.LLMConfig.SimulationConfig != nil {
		return initiator.Archetype.LLMConfig.SimulationConfig.InteractionRadius
	}
	return 0
}

// processAgentAgent handles agent_agent interactions: for each initiator, every
// matching target within the resolved radius that satisfies the conditions fires
// the interaction (initiator + target actions, plus the reverse pairing when
// Bidirectional).
func processAgentAgent(rs *RuntimeState, llmRT *llmRuntime, intents map[uint64]*AgentIntent, in models.Interaction) {
	for _, initiator := range interactionInitiators(rs, in) {
		if initiator.A.Position == nil {
			continue
		}
		r := interactionRadius(initiator, in)
		for _, target := range neighborsWithin(rs, initiator, r, in.TargetUIDs) {
			if target == nil || target.removed {
				continue
			}
			if !interactionConditionsHold(rs, initiator, intents, in) {
				continue
			}
			if !mediateFire(rs, llmRT, in, initiator, target) {
				continue
			}
			fireInteractionPair(rs, in, initiator, target)
			if rs.stats != nil {
				rs.stats.Interactions++
				rs.stats.ByInteraction[in.ID]++
			}
			if initiator.removed {
				break
			}
		}
	}
}

// processAgentShape handles agent_shape interactions: any initiator standing
// inside a matching target shape fires its InitiatorActions.
func processAgentShape(rs *RuntimeState, llmRT *llmRuntime, intents map[uint64]*AgentIntent, in models.Interaction) {
	if rs.Env == nil {
		return
	}
	w, h, torus := envDims(rs)
	for _, initiator := range interactionInitiators(rs, in) {
		if initiator.A.Position == nil {
			continue
		}
		for si := range rs.Env.Shapes {
			sh := &rs.Env.Shapes[si]
			if !matchesUIDSet(sh.ShapeUID, in.TargetUIDs) {
				continue
			}
			if !BodyOverlapsShape(initiator, sh, w, h, torus) {
				continue
			}
			if !interactionConditionsHold(rs, initiator, intents, in) {
				continue
			}
			if !mediateFire(rs, llmRT, in, initiator, nil) {
				continue
			}
			applyInteractionActions(rs, initiator, in.InitiatorActions, in)
			applyShapeActions(sh, in.TargetActions)
			if rs.stats != nil {
				rs.stats.Interactions++
				rs.stats.ByInteraction[in.ID]++
			}
			if initiator.removed {
				break
			}
		}
	}
}

// processAgentGlobal handles agent_global interactions: each initiator whose
// conditions hold against the environment globals fires its InitiatorActions.
func processAgentGlobal(rs *RuntimeState, llmRT *llmRuntime, intents map[uint64]*AgentIntent, in models.Interaction) {
	for _, initiator := range interactionInitiators(rs, in) {
		if !interactionConditionsHold(rs, initiator, intents, in) {
			continue
		}
		if !mediateFire(rs, llmRT, in, initiator, nil) {
			continue
		}
		applyInteractionActions(rs, initiator, in.InitiatorActions, in)
		if rs.stats != nil {
			rs.stats.Interactions++
			rs.stats.ByInteraction[in.ID]++
		}
	}
}

// interactionConditionsHold reports whether every condition of the interaction is
// satisfied for the initiator (AND-combined, identical to rule condition logic).
func interactionConditionsHold(rs *RuntimeState, initiator *RuntimeAgent, intents map[uint64]*AgentIntent, in models.Interaction) bool {
	if len(in.Conditions) == 0 {
		return true
	}
	var intent *AgentIntent
	if intents != nil {
		intent = intents[initiator.A.ID]
	}
	return allConditionsTrue(rs, initiator, intent, in.Conditions)
}

// mediateFire consults the LLM for UseLLM interactions (whether to fire). Non-LLM
// interactions always fire once their conditions hold. A nil llmRT degrades a
// UseLLM interaction to fire (the conditions already gated it).
func mediateFire(rs *RuntimeState, llmRT *llmRuntime, in models.Interaction, initiator, target *RuntimeAgent) bool {
	if !in.UseLLM || llmRT == nil {
		return true
	}
	fire, _ := llmRT.MediateInteraction(context.Background(), rs, in, initiator, target)
	return fire
}

// fireInteractionPair applies the initiator/target action sets for an agent_agent
// interaction, plus the reverse pairing when Bidirectional.
func fireInteractionPair(rs *RuntimeState, in models.Interaction, initiator, target *RuntimeAgent) {
	applyInteractionActions(rs, initiator, in.InitiatorActions, in)
	if !initiator.removed {
		applyInteractionActions(rs, target, in.TargetActions, in)
	}
	if in.Bidirectional {
		// Reverse pairing: the target now plays the initiator role.
		if !target.removed {
			applyInteractionActions(rs, target, in.InitiatorActions, in)
		}
		if !target.removed {
			applyInteractionActions(rs, initiator, in.TargetActions, in)
		}
	}
}

// applyInteractionActions applies a list of actions to one agent and records an
// AgentMemoryInteraction event when any action ran. Action-level memory events
// produced by ApplyAction are folded under the interaction memory entry.
func applyInteractionActions(rs *RuntimeState, ra *RuntimeAgent, actions []models.Action, in models.Interaction) {
	if ra == nil || ra.A == nil || ra.removed || len(actions) == 0 {
		return
	}
	applied := false
	for _, a := range actions {
		evts := ApplyAction(rs, ra, a)
		if len(evts) > 0 {
			applied = true
		}
		if ra.removed {
			break
		}
	}
	if applied {
		AppendMemory(ra, rs.Step, datasims.AgentMemoryInteraction, interactionSummary(in), nil, ra.memoryCap(), nowFn())
	}
}

// applyShapeActions mutates a target shape's properties for the set/modify subset
// of an agent_shape interaction's TargetActions. Shapes carry no behavioural state,
// so only direct property writes apply.
func applyShapeActions(sh *datasims.SimulationShape, actions []models.Action) {
	if sh == nil || len(actions) == 0 {
		return
	}
	for _, a := range actions {
		if a.PropertyUID == "" {
			continue
		}
		switch a.Type {
		case models.ActionTypeSetProperty:
			if a.Value == nil {
				continue
			}
			if sh.Properties == nil {
				sh.Properties = make(map[string]interface{})
			}
			sh.Properties[a.PropertyUID] = a.Value
		case models.ActionTypeModifyProperty:
			applyShapePropertyModifier(sh, a)
		}
	}
}

// applyShapePropertyModifier applies a numeric/boolean property modifier to a shape
// property in place.
func applyShapePropertyModifier(sh *datasims.SimulationShape, a models.Action) {
	if sh.Properties == nil {
		return
	}
	cur, ok := sh.Properties[a.PropertyUID]
	if !ok {
		return
	}
	if a.Modifier == models.PropertyModifierToggle {
		b, _ := cur.(bool)
		sh.Properties[a.PropertyUID] = !b
		return
	}
	curF, ok := coerceFloat(cur)
	if !ok {
		return
	}
	delta, _ := coerceFloat(a.Value)
	switch a.Modifier {
	case models.PropertyModifierIncrement:
		sh.Properties[a.PropertyUID] = curF + 1
	case models.PropertyModifierDecrement:
		sh.Properties[a.PropertyUID] = curF - 1
	case models.PropertyModifierSet:
		sh.Properties[a.PropertyUID] = delta
	case models.PropertyModifierAdd:
		sh.Properties[a.PropertyUID] = curF + delta
	case models.PropertyModifierSubtract:
		sh.Properties[a.PropertyUID] = curF - delta
	case models.PropertyModifierMultiply:
		sh.Properties[a.PropertyUID] = curF * delta
	case models.PropertyModifierDivide:
		if delta != 0 {
			sh.Properties[a.PropertyUID] = curF / delta
		}
	}
}

// interactionSummary is the memory-log summary for a fired interaction.
func interactionSummary(in models.Interaction) string {
	if in.Name != "" {
		return "interaction: " + in.Name
	}
	if in.ID != "" {
		return "interaction: " + in.ID
	}
	return "interaction"
}
