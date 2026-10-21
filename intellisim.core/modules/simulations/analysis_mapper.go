package simulations

import (
	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
)

// selectionFromDTO converts an API selection to the domain selection.
func selectionFromDTO(d AnalysisSelectionDTO) datasims.AnalysisSelection {
	s := datasims.AnalysisSelection{
		StepFrom: d.StepFrom, StepTo: d.StepTo, Sampling: d.Sampling,
		AgentUIDs: d.AgentUIDs, PropertyUIDs: d.PropertyUIDs,
	}
	for _, f := range d.FocusAreas {
		s.FocusAreas = append(s.FocusAreas, datasims.AnalysisFocusArea(f))
	}
	return s
}

func selectionToDTO(s datasims.AnalysisSelection) AnalysisSelectionDTO {
	d := AnalysisSelectionDTO{
		StepFrom: s.StepFrom, StepTo: s.StepTo, Sampling: s.Sampling,
		AgentUIDs: s.AgentUIDs, PropertyUIDs: s.PropertyUIDs,
	}
	for _, f := range s.FocusAreas {
		d.FocusAreas = append(d.FocusAreas, string(f))
	}
	return d
}

// analysisToResponse converts a domain Analysis to the full API response.
func analysisToResponse(a datasims.Analysis) AnalysisResponse {
	resp := AnalysisResponse{
		ID:                         a.ID,
		SimulationID:               a.SimulationID,
		Name:                       a.Name,
		ContextMessage:             a.ContextMessage,
		Language:                   string(a.Language),
		LLMTarget:                  llmTargetToDTO(a.LLMTarget),
		Selection:                  selectionToDTO(a.Selection),
		Status:                     string(a.Status),
		StatusText:                 a.StatusText,
		Progress:                   AnalysisProgressDTO{Stage: a.Progress.Stage, Percent: a.Progress.Percent},
		SimulationStatusAtAnalysis: string(a.SimulationStatusAtAnalysis),
		StepsCovered:               StepsCoveredDTO{From: a.StepsCovered.From, To: a.StepsCovered.To, Sampling: a.StepsCovered.Sampling, Count: a.StepsCovered.Count},
		TokenUsage:                 a.TokenUsage,
		Requests:                   a.Requests,
		Error:                      a.Error,
		CreatedAt:                  a.CreatedAt,
		CreatedBy:                  a.CreatedBy,
		StartedAt:                  a.StartedAt,
		FinishedAt:                 a.FinishedAt,
	}
	if a.Report != nil {
		r := AnalysisReportDTO{
			ExecutiveSummary: a.Report.ExecutiveSummary,
			Conclusions:      a.Report.Conclusions,
			Recommendations:  a.Report.Recommendations,
		}
		for _, s := range a.Report.Sections {
			r.Sections = append(r.Sections, AnalysisSectionDTO{
				Key: s.Key, Title: s.Title, Narrative: s.Narrative,
				KeyFindings: s.KeyFindings, CausalLinks: s.CausalLinks,
				EmergentPhenomena: s.EmergentPhenomena,
			})
		}
		resp.Report = &r
	}
	if a.File != nil {
		resp.File = &AnalysisFileDTO{
			UID: a.File.UID, Name: a.File.Name, MimeType: a.File.MimeType,
			Size: a.File.Size, CreatedAt: a.File.CreatedAt,
		}
	}
	for _, w := range a.Warnings {
		resp.Warnings = append(resp.Warnings, AnalysisWarningDTO{Stage: w.Stage, Code: w.Code, Message: w.Message})
	}
	return resp
}

func analysisToSummaryDTO(a datasims.Analysis) AnalysisSummaryDTO {
	label := a.LLMTarget.ProviderUID
	if a.LLMTarget.ModelUID != "" {
		label += " · " + a.LLMTarget.ModelUID
	}
	return AnalysisSummaryDTO{
		ID: a.ID, Name: a.Name, Status: string(a.Status), Language: string(a.Language),
		ModelLabel: label, SimulationStatusAtAnalysis: string(a.SimulationStatusAtAnalysis),
		HasFile: a.File != nil, CreatedAt: a.CreatedAt,
	}
}

// targetFromAnalyzeRequest reuses the module's existing llmTargetFromDTO helper.
func targetFromAnalyzeRequest(req AnalyzeRequest) common.LLMTarget {
	t := llmTargetFromDTO(&req.LLMTarget)
	if t == nil {
		return common.LLMTarget{}
	}
	return *t
}
