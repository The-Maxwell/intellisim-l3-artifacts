package simulations

import (
	"time"

	"intelli.sim.core/data/common"
)

// AnalyzeRequest is the body of POST /:id/analyses.
type AnalyzeRequest struct {
	Name           string               `json:"name" validate:"omitempty,max=120"`
	ContextMessage string               `json:"context_message" validate:"required,notblank,max=4000"`
	Language       string               `json:"language" validate:"required,oneof=en uk"`
	LLMTarget      LLMTargetDTO         `json:"llm_target" validate:"required"`
	Selection      AnalysisSelectionDTO `json:"selection"`
}

// AnalysisSelectionDTO scopes which results to analyze (all-empty = whole run).
type AnalysisSelectionDTO struct {
	StepFrom     *uint64  `json:"step_from,omitempty"`
	StepTo       *uint64  `json:"step_to,omitempty"`
	Sampling     *uint64  `json:"sampling,omitempty" validate:"omitempty,min=1"`
	AgentUIDs    []string `json:"agent_uids,omitempty" validate:"omitempty,dive,max=100"`
	PropertyUIDs []string `json:"property_uids,omitempty" validate:"omitempty,dive,max=100"`
	FocusAreas   []string `json:"focus_areas,omitempty" validate:"omitempty,dive,oneof=population properties spatial interactions events rules"`
}

// AnalysisProgressDTO mirrors AnalysisProgress.
type AnalysisProgressDTO struct {
	Stage   string `json:"stage"`
	Percent uint8  `json:"percent"`
}

type StepsCoveredDTO struct {
	From     uint64 `json:"from"`
	To       uint64 `json:"to"`
	Sampling uint64 `json:"sampling"`
	Count    uint64 `json:"count"`
}

type AnalysisFileDTO struct {
	UID       string    `json:"uid"`
	Name      string    `json:"name"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

type AnalysisSectionDTO struct {
	Key               string   `json:"key"`
	Title             string   `json:"title"`
	Narrative         string   `json:"narrative"`
	KeyFindings       []string `json:"key_findings,omitempty"`
	CausalLinks       []string `json:"causal_links,omitempty"`
	EmergentPhenomena []string `json:"emergent_phenomena,omitempty"`
}

type AnalysisReportDTO struct {
	Sections         []AnalysisSectionDTO `json:"sections,omitempty"`
	ExecutiveSummary string               `json:"executive_summary"`
	Conclusions      []string             `json:"conclusions,omitempty"`
	Recommendations  []string             `json:"recommendations,omitempty"`
}

type AnalysisWarningDTO struct {
	Stage   string `json:"stage"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// AnalysisResponse is the full GET/mutation response.
type AnalysisResponse struct {
	ID                         string                    `json:"id"`
	SimulationID               string                    `json:"simulation_id"`
	Name                       string                    `json:"name"`
	ContextMessage             string                    `json:"context_message"`
	Language                   string                    `json:"language"`
	LLMTarget                  LLMTargetDTO              `json:"llm_target"`
	Selection                  AnalysisSelectionDTO      `json:"selection"`
	Status                     string                    `json:"status"`
	StatusText                 string                    `json:"status_text"`
	Progress                   AnalysisProgressDTO       `json:"progress"`
	SimulationStatusAtAnalysis string                    `json:"simulation_status_at_analysis"`
	StepsCovered               StepsCoveredDTO           `json:"steps_covered"`
	Report                     *AnalysisReportDTO        `json:"report,omitempty"`
	File                       *AnalysisFileDTO          `json:"file,omitempty"`
	TokenUsage                 *common.TokenUsageSummary `json:"token_usage,omitempty"`
	Requests                   uint                      `json:"requests"`
	Warnings                   []AnalysisWarningDTO      `json:"warnings,omitempty"`
	Error                      string                    `json:"error,omitempty"`
	CreatedAt                  time.Time                 `json:"created_at"`
	CreatedBy                  string                    `json:"created_by"`
	StartedAt                  *time.Time                `json:"started_at,omitempty"`
	FinishedAt                 *time.Time                `json:"finished_at,omitempty"`
}

// AnalysisSummaryDTO is the list projection.
type AnalysisSummaryDTO struct {
	ID                         string    `json:"id"`
	Name                       string    `json:"name"`
	Status                     string    `json:"status"`
	Language                   string    `json:"language"`
	ModelLabel                 string    `json:"model_label"`
	SimulationStatusAtAnalysis string    `json:"simulation_status_at_analysis"`
	HasFile                    bool      `json:"has_file"`
	CreatedAt                  time.Time `json:"created_at"`
}

type ListAnalysesResponse struct {
	Items []AnalysisSummaryDTO `json:"items"`
}

// --- results-query DTOs ---

type ResultsSummaryDTO struct {
	StepRange  ResultsStepRangeDTO   `json:"step_range"`
	AgentTypes []ResultsAgentTypeDTO `json:"agent_types"`
	Properties []ResultsPropertyDTO  `json:"properties"`
	FocusAreas []string              `json:"focus_areas"`
	HasResults bool                  `json:"has_results"`
}

type ResultsStepRangeDTO struct {
	Min uint64 `json:"min"`
	Max uint64 `json:"max"`
}

type ResultsAgentTypeDTO struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type ResultsPropertyDTO struct {
	UID        string `json:"uid"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	EntityType string `json:"entity_type"`
}

type ResultsSeriesPointDTO struct {
	Step  int64       `json:"step"`
	Value interface{} `json:"value"`
}

type ResultsSeriesDTO struct {
	PropertyUID string                  `json:"property_uid,omitempty"`
	Population  []PopulationCountDTO    `json:"population,omitempty"`
	Series      []ResultsSeriesPointDTO `json:"series,omitempty"`
}

type PopulationCountDTO struct {
	Step  int64          `json:"step"`
	Count map[string]int `json:"count"`
}
