package simulations

import (
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// analyzableStatus reports whether a simulation may be analyzed (finished or paused).
func analyzableStatus(s datasims.SimulationStatus) bool {
	return s == datasims.SimulationStatusFinished || s == datasims.SimulationStatusPaused
}

// validateAnalysisSelection checks the selection against the model snapshot,
// the recorded properties, and the recorded step range [min,max]. Returns a
// *ValidationError (422) or nil.
func validateAnalysisSelection(sel datasims.AnalysisSelection, model *datamodels.Model, recorded []datasims.PropertyInfo, min, max int64) *ValidationError {
	var res ValidationResult
	add := func(path, reason string) {
		res.Errors = append(res.Errors, ValidationIssue{Path: path, Reason: reason})
	}

	if len(sel.AgentUIDs) > 0 && model != nil {
		known := make(map[string]bool, len(model.Agents))
		for _, a := range model.Agents {
			known[a.UID] = true
		}
		for _, uid := range sel.AgentUIDs {
			if !known[uid] {
				add("selection.agent_uids", "unknown agent uid: "+uid)
			}
		}
	}

	if len(sel.PropertyUIDs) > 0 {
		known := make(map[string]bool, len(recorded))
		for _, p := range recorded {
			known[p.UID] = true
		}
		for _, uid := range sel.PropertyUIDs {
			if !known[uid] {
				add("selection.property_uids", "unknown property uid: "+uid)
			}
		}
	}

	if sel.StepFrom != nil && sel.StepTo != nil && *sel.StepTo < *sel.StepFrom {
		add("selection.step_to", "step_to must be >= step_from")
	}
	if sel.StepFrom != nil && int64(*sel.StepFrom) > max {
		add("selection.step_from", "step_from is beyond the recorded step range")
	}
	if sel.StepTo != nil && int64(*sel.StepTo) < min {
		add("selection.step_to", "step_to is before the recorded step range")
	}

	if len(res.Errors) > 0 {
		return &ValidationError{Result: res}
	}
	return nil
}
