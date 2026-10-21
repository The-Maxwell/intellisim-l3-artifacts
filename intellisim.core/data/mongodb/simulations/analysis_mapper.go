package simulations

import (
	mongocommon "intelli.sim.core/data/mongodb/common"
	"intelli.sim.core/data/simulations"
)

// AnalysisToDocument converts a domain Analysis to its BSON document. A non-empty
// id (update path) is parsed onto _id; SimulationID must be a valid hex id.
func AnalysisToDocument(a simulations.Analysis) (AnalysisDocument, error) {
	simOID, err := mongocommon.ParseID(a.SimulationID)
	if err != nil {
		return AnalysisDocument{}, err
	}
	doc := AnalysisDocument{
		SimulationID:               simOID,
		Name:                       a.Name,
		ContextMessage:             a.ContextMessage,
		Language:                   string(a.Language),
		LLMTarget:                  mongocommon.LLMTargetToDocument(a.LLMTarget),
		Selection:                  selectionToDocument(a.Selection),
		Status:                     string(a.Status),
		StatusText:                 a.StatusText,
		Progress:                   AnalysisProgressDocument{Stage: a.Progress.Stage, Percent: a.Progress.Percent},
		SimulationStatusAtAnalysis: string(a.SimulationStatusAtAnalysis),
		StepsCovered: AnalysisStepsCoveredDocument{
			From: a.StepsCovered.From, To: a.StepsCovered.To,
			Sampling: a.StepsCovered.Sampling, Count: a.StepsCovered.Count,
		},
		Requests:   a.Requests,
		Error:      a.Error,
		CreatedAt:  a.CreatedAt,
		UpdatedAt:  a.UpdatedAt,
		CreatedBy:  a.CreatedBy,
		StartedAt:  a.StartedAt,
		FinishedAt: a.FinishedAt,
	}
	if a.ID != "" {
		oid, err := mongocommon.ParseID(a.ID)
		if err != nil {
			return AnalysisDocument{}, err
		}
		doc.ID = oid
	}
	if a.Report != nil {
		doc.Report = reportToDocument(*a.Report)
	}
	if a.File != nil {
		fd, err := mongocommon.FileToDocument(*a.File)
		if err != nil {
			return AnalysisDocument{}, err
		}
		doc.File = &fd
	}
	if a.TokenUsage != nil {
		td := mongocommon.TokenUsageSummaryToDocument(*a.TokenUsage)
		doc.TokenUsage = &td
	}
	for _, w := range a.Warnings {
		doc.Warnings = append(doc.Warnings, AnalysisWarningDocument{Stage: w.Stage, Code: w.Code, Message: w.Message})
	}
	return doc, nil
}

// AnalysisFromDocument converts a BSON document back to the domain entity.
func AnalysisFromDocument(doc AnalysisDocument) simulations.Analysis {
	a := simulations.Analysis{
		ID:                         doc.ID.Hex(),
		SimulationID:               doc.SimulationID.Hex(),
		Name:                       doc.Name,
		ContextMessage:             doc.ContextMessage,
		Language:                   simulations.AnalysisLanguage(doc.Language),
		LLMTarget:                  mongocommon.LLMTargetFromDocument(doc.LLMTarget),
		Selection:                  selectionFromDocument(doc.Selection),
		Status:                     simulations.AnalysisStatus(doc.Status),
		StatusText:                 doc.StatusText,
		Progress:                   simulations.AnalysisProgress{Stage: doc.Progress.Stage, Percent: doc.Progress.Percent},
		SimulationStatusAtAnalysis: simulations.SimulationStatus(doc.SimulationStatusAtAnalysis),
		StepsCovered: simulations.AnalysisStepsCovered{
			From: doc.StepsCovered.From, To: doc.StepsCovered.To,
			Sampling: doc.StepsCovered.Sampling, Count: doc.StepsCovered.Count,
		},
		Requests:   doc.Requests,
		Error:      doc.Error,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
		CreatedBy:  doc.CreatedBy,
		StartedAt:  doc.StartedAt,
		FinishedAt: doc.FinishedAt,
	}
	if doc.Report != nil {
		r := reportFromDocument(*doc.Report)
		a.Report = &r
	}
	if doc.File != nil {
		f := mongocommon.FileFromDocument(*doc.File)
		a.File = &f
	}
	if doc.TokenUsage != nil {
		t := mongocommon.TokenUsageSummaryFromDocument(*doc.TokenUsage)
		a.TokenUsage = &t
	}
	for _, w := range doc.Warnings {
		a.Warnings = append(a.Warnings, simulations.AnalysisWarning{Stage: w.Stage, Code: w.Code, Message: w.Message})
	}
	return a
}

func selectionToDocument(s simulations.AnalysisSelection) AnalysisSelectionDocument {
	d := AnalysisSelectionDocument{
		StepFrom: s.StepFrom, StepTo: s.StepTo, Sampling: s.Sampling,
		AgentUIDs: s.AgentUIDs, PropertyUIDs: s.PropertyUIDs,
	}
	for _, f := range s.FocusAreas {
		d.FocusAreas = append(d.FocusAreas, string(f))
	}
	return d
}

func selectionFromDocument(d AnalysisSelectionDocument) simulations.AnalysisSelection {
	s := simulations.AnalysisSelection{
		StepFrom: d.StepFrom, StepTo: d.StepTo, Sampling: d.Sampling,
		AgentUIDs: d.AgentUIDs, PropertyUIDs: d.PropertyUIDs,
	}
	for _, f := range d.FocusAreas {
		s.FocusAreas = append(s.FocusAreas, simulations.AnalysisFocusArea(f))
	}
	return s
}

func reportToDocument(r simulations.AnalysisReport) *AnalysisReportDocument {
	d := &AnalysisReportDocument{
		ExecutiveSummary: r.ExecutiveSummary,
		Conclusions:      r.Conclusions,
		Recommendations:  r.Recommendations,
	}
	for _, s := range r.Sections {
		d.Sections = append(d.Sections, AnalysisSectionDocument{
			Key: s.Key, Title: s.Title, Narrative: s.Narrative,
			KeyFindings: s.KeyFindings, CausalLinks: s.CausalLinks,
			EmergentPhenomena: s.EmergentPhenomena,
		})
	}
	return d
}

func reportFromDocument(d AnalysisReportDocument) simulations.AnalysisReport {
	r := simulations.AnalysisReport{
		ExecutiveSummary: d.ExecutiveSummary,
		Conclusions:      d.Conclusions,
		Recommendations:  d.Recommendations,
	}
	for _, s := range d.Sections {
		r.Sections = append(r.Sections, simulations.AnalysisSection{
			Key: s.Key, Title: s.Title, Narrative: s.Narrative,
			KeyFindings: s.KeyFindings, CausalLinks: s.CausalLinks,
			EmergentPhenomena: s.EmergentPhenomena,
		})
	}
	return r
}
