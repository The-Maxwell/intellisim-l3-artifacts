package simulations

import (
	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// individualDecisionStrategy is the Strategy-4 (decision × agent) execution
// strategy: for the decision purpose it asks the LLM to choose ONE high-level
// label per agent (from the archetype's DecisionMenu); the rule engine translates
// those labels into concrete actions. Other enabled purposes (properties /
// movement) reuse the per-agent property/movement contracts. Optionally batches ≤
// Strategy.BatchSize agents per call. Capability for the decision purpose:
// ModelCapabilitySimulationDecisions.
type individualDecisionStrategy struct{}

func (individualDecisionStrategy) Name() string { return "individual_decision" }

func (individualDecisionStrategy) Plan(sc *StepContext, group *AgentGroup) (PlanResult, error) {
	res := PlanResult{Intents: map[uint64]*AgentIntent{}}
	if sc == nil || sc.RS == nil || group == nil || sc.LLM == nil {
		return res, nil
	}

	purposes := enabledPurposes(group.Strategy)
	if len(purposes) == 0 {
		return res, nil
	}

	for _, p := range purposes {
		switch p {
		case purposeDecision:
			// The decision purpose honors on-demand call_llm requests: off-schedule
			// steps include only the pending members (flags consumed either way).
			members := decisionDueMembers(sc.RS.Step, group.Strategy.DecisionFrequency, group.Members)
			if len(members) > 0 {
				planDecisionPurpose(sc, group, members, &res)
			}
		case purposeProperties, purposeMovement:
			if !purposeDue(sc.RS.Step, group.Strategy.DecisionFrequency) {
				continue
			}
			// Non-decision purposes share the per-agent property/movement
			// contracts (demuxed by the full strategy's helpers) but are validated
			// under the decisions capability — a decision-strategy agent's target
			// is capability-validated for SimulationDecisions at run start.
			planFullPurpose(sc, group, p, common.ModelCapabilitySimulationDecisions, group.Members, &res)
		case purposeInteraction:
			continue
		}
	}
	return res, nil
}

// planDecisionPurpose runs the decision purpose across the given members
// (normally the whole group; a pending-only subset for off-schedule call_llm
// requests) in BatchSize-sized batches, demuxing the label response by instance
// index into a per-agent DecisionIntent and recording ra.LastDecision (for the
// last_decision fallback). On callJSON ok=false, the batch's agents fall back:
// an agent with a prior LastDecision reuses it (last_decision behaviour);
// otherwise rules cover it.
func planDecisionPurpose(sc *StepContext, group *AgentGroup, members []*RuntimeAgent, res *PlanResult) {
	rs := sc.RS
	for _, batch := range chunkMembers(members, group.Strategy.BatchSize) {
		system, user := buildIndividualPrompt(rs, group, purposeDecision, batch)
		fb := resolveModelFallback(rs.Model, group.Archetype)

		var out decisionResp
		if !sc.LLM.callJSON(sc.Ctx, group.Target, common.ModelCapabilitySimulationDecisions,
			datasims.LLMUsageSimulationDecision, group.ArchetypeUID, system, user, fb, &out) {
			// LLM gave no fresh decision for this batch: replay each agent's last
			// remembered decision when present; otherwise leave it to the rules.
			for _, ra := range batch {
				if ra.LastDecision != nil {
					intent := ensureIntent(res, ra.A.ID)
					intent.Decisions = append(intent.Decisions, *ra.LastDecision)
				}
			}
			continue
		}
		demuxDecisionsWithState(rs, batch, out, res)
	}
}

// demuxDecisionsWithState maps each instance's label onto a per-agent
// DecisionIntent and records it as the agent's LastDecision for the
// last_decision fallback. It also sets intent.LLMNote to "chose: <label>" so the
// mutation phase can flush it to AgentMemoryLLM for future prompt feedback.
//
// Task 17: if the response entry includes a non-empty emit_uid, it is resolved
// against the model's EventDefinitions and authorized via authorizeEmit(def,
// EventProducerLLM, archetypeUID). Authorized emits are appended to intent.Emits;
// unauthorized ones (no "llm" in Producers, or ProducerUIDs mismatch) are silently
// dropped. rs is used for eventDefByUID; when rs is nil, emits are skipped.
func demuxDecisionsWithState(rs *RuntimeState, batch []*RuntimeAgent, out decisionResp, res *PlanResult) {
	for _, a := range out.Agents {
		ra := memberAt(batch, a.Instance)
		if ra == nil || a.Label == "" {
			continue
		}
		dec := DecisionIntent{Label: a.Label, Params: a.Params}
		intent := ensureIntent(res, ra.A.ID)
		intent.Decisions = append(intent.Decisions, dec)
		intent.LLMNote = "chose: " + a.Label
		// SET LastDecision (copy) so the last_decision fallback can replay it.
		last := dec
		ra.LastDecision = &last
		// Task 17: translate optional emit into AgentIntent.Emits.
		if a.EmitUID != "" && rs != nil {
			archetypeUID := ""
			if ra.A != nil {
				archetypeUID = ra.A.AgentUID
			}
			def := eventDefByUID(rs, a.EmitUID)
			if authorizeEmit(def, models.EventProducerLLM, archetypeUID) {
				intent.Emits = append(intent.Emits, EmitIntent{EventUID: a.EmitUID, Data: a.EmitData})
			}
		}
	}
}
