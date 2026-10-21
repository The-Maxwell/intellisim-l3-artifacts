package simulations

import (
	"fmt"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// ValidateRunnable reports whether a simulation may start a run, returning nil
// when runnable or an ErrNotRunnable-wrapped error (carrying a human-readable
// detail) otherwise. It checks, in order:
//
//   - status is ready (any run/terminal status locks the record);
//   - the simulation has at least one agent and Steps ≥ 1;
//   - state integrity via the create/edit ValidateInstance (no blocking errors:
//     unknown archetypes/properties, bad states, type mismatches, etc.);
//   - every declared archetype whose resolved strategy is a decision strategy
//     using decisions references a non-empty decision menu (the LLM has labels to
//     pick from);
//   - every LLM-enabled archetype (resolved Type != none with any UseFor* flag)
//     that resolves to a concrete LLM target passes inv.ValidateTarget for the
//     capability its strategy requires (full → simulation_execution; decision /
//     the LLM half of hybrid → simulation_decisions).
//
// An LLM-enabled archetype that resolves to NO target is NOT a run-start failure:
// the engine degrades that purpose to rule-based at runtime (resolveTarget ok=false),
// so blocking here would be wrong. inv.ValidateTarget is therefore consulted only
// when a target actually resolves.
func ValidateRunnable(sim *datasims.Simulation, inv llmInvoker) error {
	if sim == nil {
		return fmt.Errorf("%w: simulation is nil", ErrNotRunnable)
	}
	if sim.Status != datasims.SimulationStatusReady {
		return fmt.Errorf("%w: status is %q, must be %q", ErrNotRunnable, sim.Status, datasims.SimulationStatusReady)
	}
	if len(sim.Agents) < 1 {
		return fmt.Errorf("%w: simulation has no agents", ErrNotRunnable)
	}
	if sim.Steps < 1 {
		return fmt.Errorf("%w: steps must be at least 1", ErrNotRunnable)
	}

	model := sim.ModelSnapshot
	if model == nil {
		return fmt.Errorf("%w: missing model snapshot", ErrNotRunnable)
	}

	// A spatial run needs a non-degenerate world. There is no panic risk (the
	// wrap/clamp math guards dims <= 0), but a zero-dimension world silently
	// collapses every position to (0,0). The models-side env_dims check is
	// advisory-only, so this is the single hard stop.
	if model.IsSpatial && (sim.Environment == nil || sim.Environment.Width == 0 || sim.Environment.Height == 0) {
		return fmt.Errorf("%w: spatial simulation requires an environment with width and height > 0", ErrNotRunnable)
	}

	// State integrity — reuse the create/edit validator. Only blocking Errors
	// gate a run; advisory Warnings (e.g. population bounds, out-of-bounds
	// positions) do not.
	if res := ValidateInstance(*sim); len(res.Errors) > 0 {
		return fmt.Errorf("%w: %s", ErrNotRunnable, res.Errors[0].Reason)
	}

	// Per-archetype LLM checks.
	for i := range model.Agents {
		arch := &model.Agents[i]
		strat := resolveArchetypeStrategy(model, arch)
		if strat.Type == models.LLMStrategyNone {
			continue // pure rule-based archetype: no LLM checks.
		}

		// Decision-strategy archetypes that drive decisions need a non-empty menu.
		if strat.Type == models.LLMStrategyDecision && strat.UseForDecisions {
			if len(DecisionMenu(model, arch.UID)) == 0 {
				return fmt.Errorf("%w: archetype %q uses a decision strategy but no rule declares a decision label (empty decision menu)",
					ErrNotRunnable, arch.UID)
			}
		}

		if !strategyUsesLLM(strat) {
			continue // strategy type set but no purpose enabled: nothing to validate.
		}

		// Resolve the archetype's effective target. No target ⇒ the purpose
		// degrades to rules at runtime; that is not a run-start failure.
		target, ok := resolveTarget(model, sim, arch)
		if !ok {
			continue
		}

		capability := capabilityForStrategy(strat.Type)
		if err := inv.ValidateTarget(target, capability); err != nil {
			return fmt.Errorf("%w: archetype %q LLM target %s/%s invalid for %s: %v",
				ErrNotRunnable, arch.UID, target.ProviderUID, target.ModelUID, capability, err)
		}
	}

	return nil
}

// strategyUsesLLM reports whether a resolved strategy actually invokes the LLM for
// any per-step purpose (movement, properties, decisions, interactions). A strategy
// with a set Type but no enabled purpose runs entirely on rules.
func strategyUsesLLM(s models.LLMStrategy) bool {
	if s.Type == models.LLMStrategyNone || s.Type == "" {
		return false
	}
	return s.UseForMovement || s.UseForProperties || s.UseForDecisions || s.UseForInteraction
}

// capabilityForStrategy maps a resolved (non-hybrid) strategy type to the LLM
// capability its calls require: full control needs simulation_execution; decision
// control needs simulation_decisions. resolveArchetypeStrategy never returns
// hybrid (it resolves per-archetype), but hybrid/unknown defaults to the decision
// capability — the narrower of the two.
func capabilityForStrategy(t models.LLMStrategyType) common.ModelCapability {
	if t == models.LLMStrategyFull {
		return common.ModelCapabilitySimulationExecution
	}
	return common.ModelCapabilitySimulationDecisions
}

// ValidateControl reports whether a run-control action is legal for the current
// simulation status, returning nil when legal, ErrNotRunning when the action is a
// known control verb illegal for the status, or ErrNotRunnable for an unknown
// action verb. Legality:
//
//	pause      → only while running
//	resume     → only while paused
//	stop       → while running or paused
//	set_speed  → while running or paused (dedicated speed-only action, ISS-09:
//	             the UI speed slider must work on a live run, not just piggyback
//	             on pause/resume)
func ValidateControl(action string, status datasims.SimulationStatus) error {
	switch action {
	case "pause":
		if status != datasims.SimulationStatusRunning {
			return fmt.Errorf("%w: cannot pause a simulation in status %q", ErrNotRunning, status)
		}
	case "resume":
		if status != datasims.SimulationStatusPaused {
			return fmt.Errorf("%w: cannot resume a simulation in status %q", ErrNotRunning, status)
		}
	case "stop":
		if status != datasims.SimulationStatusRunning && status != datasims.SimulationStatusPaused {
			return fmt.Errorf("%w: cannot stop a simulation in status %q", ErrNotRunning, status)
		}
	case "set_speed":
		if status != datasims.SimulationStatusRunning && status != datasims.SimulationStatusPaused {
			return fmt.Errorf("%w: cannot set speed on a simulation in status %q", ErrNotRunning, status)
		}
	default:
		return fmt.Errorf("%w: unknown control action %q", ErrNotRunnable, action)
	}
	return nil
}
