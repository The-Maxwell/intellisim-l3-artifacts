package simulations

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	mongocommon "intelli.sim.core/data/mongodb/common"
)

// AnalysisDocument is the BSON form of simulations.Analysis (collection: simulation_analyses).
type AnalysisDocument struct {
	ID                         bson.ObjectID                          `bson:"_id,omitempty"`
	SimulationID               bson.ObjectID                          `bson:"simulation_id"`
	Name                       string                                 `bson:"name"`
	ContextMessage             string                                 `bson:"context_message"`
	Language                   string                                 `bson:"language"`
	LLMTarget                  mongocommon.LLMTargetDocument          `bson:"llm_target"`
	Selection                  AnalysisSelectionDocument              `bson:"selection"`
	Status                     string                                 `bson:"status"`
	StatusText                 string                                 `bson:"status_text"`
	Progress                   AnalysisProgressDocument               `bson:"progress"`
	SimulationStatusAtAnalysis string                                 `bson:"simulation_status_at_analysis"`
	StepsCovered               AnalysisStepsCoveredDocument           `bson:"steps_covered"`
	Report                     *AnalysisReportDocument                `bson:"report,omitempty"`
	File                       *mongocommon.FileDocument              `bson:"file,omitempty"`
	TokenUsage                 *mongocommon.TokenUsageSummaryDocument `bson:"token_usage,omitempty"`
	Requests                   uint                                   `bson:"requests"`
	Warnings                   []AnalysisWarningDocument              `bson:"warnings,omitempty"`
	Error                      string                                 `bson:"error,omitempty"`
	CreatedAt                  time.Time                              `bson:"created_at"`
	UpdatedAt                  time.Time                              `bson:"updated_at"`
	CreatedBy                  string                                 `bson:"created_by"`
	StartedAt                  *time.Time                             `bson:"started_at,omitempty"`
	FinishedAt                 *time.Time                             `bson:"finished_at,omitempty"`
}

type AnalysisSelectionDocument struct {
	StepFrom     *uint64  `bson:"step_from,omitempty"`
	StepTo       *uint64  `bson:"step_to,omitempty"`
	Sampling     *uint64  `bson:"sampling,omitempty"`
	AgentUIDs    []string `bson:"agent_uids,omitempty"`
	PropertyUIDs []string `bson:"property_uids,omitempty"`
	FocusAreas   []string `bson:"focus_areas,omitempty"`
}

type AnalysisProgressDocument struct {
	Stage   string `bson:"stage"`
	Percent uint8  `bson:"percent"`
}

type AnalysisStepsCoveredDocument struct {
	From     uint64 `bson:"from"`
	To       uint64 `bson:"to"`
	Sampling uint64 `bson:"sampling"`
	Count    uint64 `bson:"count"`
}

type AnalysisSectionDocument struct {
	Key               string   `bson:"key"`
	Title             string   `bson:"title"`
	Narrative         string   `bson:"narrative"`
	KeyFindings       []string `bson:"key_findings,omitempty"`
	CausalLinks       []string `bson:"causal_links,omitempty"`
	EmergentPhenomena []string `bson:"emergent_phenomena,omitempty"`
}

type AnalysisReportDocument struct {
	Sections         []AnalysisSectionDocument `bson:"sections,omitempty"`
	ExecutiveSummary string                    `bson:"executive_summary"`
	Conclusions      []string                  `bson:"conclusions,omitempty"`
	Recommendations  []string                  `bson:"recommendations,omitempty"`
}

type AnalysisWarningDocument struct {
	Stage   string `bson:"stage"`
	Code    string `bson:"code"`
	Message string `bson:"message"`
}
