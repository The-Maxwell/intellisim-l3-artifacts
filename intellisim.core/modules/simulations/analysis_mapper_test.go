package simulations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	datasims "intelli.sim.core/data/simulations"
)

func TestAnalysisToResponse_EmergentPhenomena(t *testing.T) {
	phenomena := []string{"emergent flocking", "oscillatory crash"}
	a := datasims.Analysis{
		ID:           "aaa",
		SimulationID: "bbb",
		Name:         "Test Analysis",
		Language:     datasims.AnalysisLanguageEN,
		Status:       datasims.AnalysisStatusCompleted,
		CreatedAt:    time.Now().UTC(),
		Report: &datasims.AnalysisReport{
			ExecutiveSummary: "exec",
			Sections: []datasims.AnalysisSection{
				{
					Key:               "emergence",
					Title:             "Emergent Behavior",
					Narrative:         "agents cluster spontaneously",
					KeyFindings:       []string{"clusters form"},
					CausalLinks:       []string{"density -> cluster"},
					EmergentPhenomena: phenomena,
				},
			},
		},
	}

	resp := analysisToResponse(a)
	require.NotNil(t, resp.Report)
	require.Len(t, resp.Report.Sections, 1)
	require.Equal(t, phenomena, resp.Report.Sections[0].EmergentPhenomena,
		"analysisToResponse must carry EmergentPhenomena from entity into DTO")
}

func TestAnalysisToResponse_EmergentPhenomenaOmitted(t *testing.T) {
	// When EmergentPhenomena is nil, it must be nil in the DTO (omitempty).
	a := datasims.Analysis{
		ID:           "ccc",
		SimulationID: "ddd",
		Name:         "No Emergence",
		Language:     datasims.AnalysisLanguageEN,
		Status:       datasims.AnalysisStatusCompleted,
		CreatedAt:    time.Now().UTC(),
		Report: &datasims.AnalysisReport{
			ExecutiveSummary: "exec",
			Sections: []datasims.AnalysisSection{
				{Key: "dynamics", Title: "Dynamics", Narrative: "stable", EmergentPhenomena: nil},
			},
		},
	}

	resp := analysisToResponse(a)
	require.NotNil(t, resp.Report)
	require.Len(t, resp.Report.Sections, 1)
	require.Nil(t, resp.Report.Sections[0].EmergentPhenomena,
		"nil EmergentPhenomena must be nil in DTO (omitempty / backward compat)")
}
