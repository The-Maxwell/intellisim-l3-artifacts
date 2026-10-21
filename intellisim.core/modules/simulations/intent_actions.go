package simulations

import (
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// ApplyIntentActions applies the full-control LLM intents (Actions + Properties +
// Emits + LLMNote) that the agent's strategy owns onto ra.
//
// Suppression rule (complementary to ApplyRules):
//
//	The rule engine already suppresses rule actions for purposes the agent's LLM
//	strategy owns (ownedPurposesFor → actionPurpose gating in ApplyRules).  To
//	avoid the inverse overlap — where intents would re-apply actions the rule engine
//	is responsible for — ApplyIntentActions applies an intent action ONLY if the
//	action's purpose IS owned by the agent's strategy.  The purpose-to-action
//	mapping is identical to the one in actionPurpose:
//
//	  set_property / modify_property → purposeProperties
//	  transition                     → purposeDecision
//	  interact                       → purposeInteraction
//	  move / move_toward / move_away / teleport → purposeMovement (ALWAYS skipped
//	      here; movement is already handled by applySpatial via intent.Movement)
//
//	call_llm has NO purpose (it is a rule-engine-only trigger; validateRawAction
//	never admits it from the LLM), so the no-purpose skip below drops it.
//
//	Because move-type actions are handled exclusively in the spatial phase via
//	intent.Movement, they are never applied here even if purposeMovement is owned —
//	doing so would double-integrate the velocity.
//
//	in.Properties (direct assignment map) is applied only when purposeProperties is
//	owned; immutable properties are always skipped (writeProperty handles this).
//
// LLM-emitted events (Task 17):
//
//	in.Emits carries zero or more EmitIntent entries requested by the LLM during
//	the gather phase. Each is applied via applyEmitEvent(..., EventProducerLLM) so
//	that authorization + stats attribution use the LLM producer kind, not the
//	rule-engine agent producer kind. Emits are applied AFTER the Actions loop so
//	they are always queued in the sequential mutation sub-phase — they are never
//	dispatched during the (potentially parallel) LLM gather phase.
//
//	Authorization is double-checked inside applyEmitEvent (via authorizeEmit):
//	validateRawAction already enforced it at decode time, but the secondary check
//	is free and guards against any future path that bypasses validation.
//
// LLMNote flush (absorbs Task 7 temporary loops):
//
//	At the end, if in.LLMNote != "", it is written to AgentMemoryLLM on ra so the
//	agent's recent LLM context feeds back into future prompts.  In synchronous mode
//	this runs on the working copy w; the write is committed by commitRuntimeAgent.
func ApplyIntentActions(rs *RuntimeState, ra *RuntimeAgent, in *AgentIntent) {
	if in == nil || ra == nil || ra.A == nil || ra.removed {
		return
	}

	owned := ownedPurposesFor(ra)

	// Apply in.Actions — only for purposes the agent's strategy owns.
	// Move-type actions are excluded here unconditionally: they are already handled
	// by the spatial phase via intent.Movement, and applying them again would
	// double-integrate the velocity.
	for _, ai := range in.Actions {
		p, hasPurpose := actionPurpose(ai.Type)
		if !hasPurpose {
			// Actions with no purpose (e.g. spawn/remove/emit) are engine/rule
			// concerns; full-control LLM is not permitted to emit them via the Actions
			// path (MapFullActions / validateRawAction routes authorized emit_event
			// actions into in.Emits, not in.Actions), so skip silently.
			continue
		}
		if p == purposeMovement {
			// Movement is always resolved by applySpatial via intent.Movement.
			// Never re-apply here to avoid double-integration.
			continue
		}
		if !owned.has(p) {
			// This purpose belongs to the rule engine for this agent; intent actions
			// that cross the boundary are silently suppressed.
			continue
		}
		// Translate ActionIntent → models.Action and apply via the shared handler.
		action := models.Action{
			Type:        ai.Type,
			PropertyUID: ai.PropertyUID,
			Value:       ai.Value,
			Modifier:    ai.Modifier,
			TargetUID:   ai.TargetUID,
			Speed:       ai.Speed,
			Direction:   ai.Direction,
			NewState:    ai.NewState,
		}
		evts := ApplyAction(rs, ra, action)
		for _, e := range evts {
			AppendMemory(ra, rs.Step, e.Type, e.Summary, e.Details, ra.memoryCap(), nowFn())
		}
		if ra.removed {
			break
		}
	}

	// Apply in.Properties (direct map assignment) only when purposeProperties is owned.
	// writeProperty honours the Immutable flag on each property descriptor; no extra
	// guard is needed here beyond the owned-purpose check.
	if len(in.Properties) > 0 && owned.has(purposeProperties) {
		for uid, val := range in.Properties {
			writeProperty(ra, uid, val)
		}
	}

	// Drain LLM-emit intents (Task 17). Each EmitIntent was authorized at decode
	// time by validateRawAction; applyEmitEvent re-checks authorization and
	// attributes the emit to EventProducerLLM (not EventProducerAgent), ensuring
	// stats + scope delivery correctly reflect the LLM as the event source.
	// Emits are drained here (sequential mutation sub-phase) — never during the
	// parallel LLM gather phase — so rs.events appends are single-threaded.
	if !ra.removed {
		for _, e := range in.Emits {
			evts := applyEmitEvent(rs, ra,
				models.Action{Type: models.ActionTypeEmitEvent, EventUID: e.EventUID, EventData: e.Data},
				models.EventProducerLLM,
			)
			for _, ev := range evts {
				AppendMemory(ra, rs.Step, ev.Type, ev.Summary, ev.Details, ra.memoryCap(), nowFn())
			}
			if ra.removed {
				break
			}
		}
	}

	// Flush in.LLMNote → AgentMemoryLLM.  This absorbs the temporary per-agent flush
	// loops that Task 7 added to both mutation paths in step.go.  In synchronous mode
	// ra is the working copy w; the write commits via commitRuntimeAgent at tick end.
	if in.LLMNote != "" {
		AppendMemory(ra, rs.Step, datasims.AgentMemoryLLM, in.LLMNote, nil, ra.memoryCap(), nowFn())
	}
}
