package simulations

import (
	"time"

	"intelli.sim.core/data/common"
)

// AnalysisStatus is the lifecycle of a Level-3 results analysis.
type AnalysisStatus string

const (
	AnalysisStatusPending   AnalysisStatus = "pending"
	AnalysisStatusRunning   AnalysisStatus = "running"
	AnalysisStatusCompleted AnalysisStatus = "completed"
	AnalysisStatusFailed    AnalysisStatus = "failed"
)

// AnalysisLanguage is the output language of the report.
type AnalysisLanguage string

const (
	AnalysisLanguageEN AnalysisLanguage = "en"
	AnalysisLanguageUK AnalysisLanguage = "uk"
)

// AnalysisFocusArea is an analytical aspect the report may emphasize.
type AnalysisFocusArea string

const (
	FocusPopulation   AnalysisFocusArea = "population"
	FocusProperties   AnalysisFocusArea = "properties"
	FocusSpatial      AnalysisFocusArea = "spatial"
	FocusInteractions AnalysisFocusArea = "interactions"
	FocusEvents       AnalysisFocusArea = "events"
	FocusRules        AnalysisFocusArea = "rules"
)

// AnalysisSelection scopes which recorded results feed the analysis. All-zero
// (no range, no agents/properties, no focus areas) means "whole run, all data".
type AnalysisSelection struct {
	StepFrom     *uint64
	StepTo       *uint64
	Sampling     *uint64 // keep every Nth step; nil/0/1 = no down-sampling
	AgentUIDs    []string
	PropertyUIDs []string
	FocusAreas   []AnalysisFocusArea
}

// AnalysisSection is one LLM-produced report section.
type AnalysisSection struct {
	Key               string
	Title             string
	Narrative         string
	KeyFindings       []string
	CausalLinks       []string
	EmergentPhenomena []string
}

// AnalysisReport is the assembled analytical content (sections + synthesis).
type AnalysisReport struct {
	Sections         []AnalysisSection
	ExecutiveSummary string
	Conclusions      []string
	Recommendations  []string
}

// AnalysisProgress is the live pipeline progress snapshot.
type AnalysisProgress struct {
	Stage   string
	Percent uint8
}

// AnalysisStepsCovered records the effective window the analysis actually used.
type AnalysisStepsCovered struct {
	From     uint64
	To       uint64
	Sampling uint64
	Count    uint64
}

// AnalysisWarning is a non-fatal degradation note (e.g. a section LLM call failed).
type AnalysisWarning struct {
	Stage   string
	Code    string
	Message string
}

// Analysis is a single Level-3 results-analysis record for a simulation.
type Analysis struct {
	ID                         string
	SimulationID               string
	Name                       string
	ContextMessage             string
	Language                   AnalysisLanguage
	LLMTarget                  common.LLMTarget
	Selection                  AnalysisSelection
	Status                     AnalysisStatus
	StatusText                 string
	Progress                   AnalysisProgress
	SimulationStatusAtAnalysis SimulationStatus
	StepsCovered               AnalysisStepsCovered
	Report                     *AnalysisReport
	File                       *common.File
	TokenUsage                 *common.TokenUsageSummary
	Requests                   uint
	Warnings                   []AnalysisWarning
	Error                      string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
	CreatedBy                  string
	StartedAt                  *time.Time
	FinishedAt                 *time.Time
}
