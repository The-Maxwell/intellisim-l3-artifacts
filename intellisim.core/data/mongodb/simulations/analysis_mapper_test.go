package simulations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.mongodb.org/mongo-driver/v2/bson"
	"intelli.sim.core/data/common"
	"intelli.sim.core/data/simulations"
)

func TestAnalysisMapperRoundTrip(t *testing.T) {
	from := uint64(0)
	to := uint64(100)
	a := simulations.Analysis{
		ID:             bson.NewObjectID().Hex(),
		SimulationID:   bson.NewObjectID().Hex(),
		Name:           "Predator-Prey Analysis",
		ContextMessage: "Focus on population collapse",
		Language:       simulations.AnalysisLanguageEN,
		LLMTarget:      common.LLMTarget{ProviderUID: "openai", ModelUID: "o3", Effort: common.LLMEffort("medium")},
		Selection: simulations.AnalysisSelection{
			StepFrom: &from, StepTo: &to,
			AgentUIDs:  []string{"predator"},
			FocusAreas: []simulations.AnalysisFocusArea{simulations.FocusPopulation},
		},
		Status:    simulations.AnalysisStatusCompleted,
		Report:    &simulations.AnalysisReport{ExecutiveSummary: "summary", Conclusions: []string{"c1"}, Sections: []simulations.AnalysisSection{{Key: "emergence", Title: "Emergence", Narrative: "n"}}},
		File:      &common.File{UID: "f1", Name: "a.pdf", Type: common.SimulationFileAnalysisReport, MimeType: "application/pdf", Size: 123, GridFSID: bson.NewObjectID().Hex()},
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Millisecond),
		CreatedBy: "user-1",
	}
	doc, err := AnalysisToDocument(a)
	require.NoError(t, err)
	got := AnalysisFromDocument(doc)
	require.Equal(t, a.Name, got.Name)
	require.Equal(t, a.Language, got.Language)
	require.Equal(t, a.LLMTarget.ModelUID, got.LLMTarget.ModelUID)
	require.Equal(t, a.Selection.AgentUIDs, got.Selection.AgentUIDs)
	require.Equal(t, a.Selection.FocusAreas, got.Selection.FocusAreas)
	require.Equal(t, a.Status, got.Status)
	require.NotNil(t, got.Report)
	require.Equal(t, "summary", got.Report.ExecutiveSummary)
	require.NotNil(t, got.File)
	require.Equal(t, a.File.GridFSID, got.File.GridFSID)
}

func TestAnalysisMapperRoundTrip_EmergentPhenomena(t *testing.T) {
	// Verify EmergentPhenomena survives a full entity→document→entity round-trip.
	phenomena := []string{"self-organisation into clusters", "oscillatory population boom-bust"}
	a := simulations.Analysis{
		ID:           bson.NewObjectID().Hex(),
		SimulationID: bson.NewObjectID().Hex(),
		Name:         "Emergence Round-Trip",
		Language:     simulations.AnalysisLanguageEN,
		LLMTarget:    common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
		Status:       simulations.AnalysisStatusCompleted,
		Report: &simulations.AnalysisReport{
			ExecutiveSummary: "exec",
			Sections: []simulations.AnalysisSection{
				{
					Key:                "emergence",
					Title:              "Emergent Behavior",
					Narrative:          "System exhibits self-organisation.",
					KeyFindings:        []string{"clusters form"},
					CausalLinks:        []string{"density -> clustering"},
					EmergentPhenomena:  phenomena,
				},
			},
		},
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Millisecond),
		CreatedBy: "user-1",
	}
	doc, err := AnalysisToDocument(a)
	require.NoError(t, err)

	// Document must carry emergent_phenomena.
	require.Len(t, doc.Report.Sections, 1)
	require.Equal(t, phenomena, doc.Report.Sections[0].EmergentPhenomena)

	got := AnalysisFromDocument(doc)
	require.NotNil(t, got.Report)
	require.Len(t, got.Report.Sections, 1)
	require.Equal(t, phenomena, got.Report.Sections[0].EmergentPhenomena, "EmergentPhenomena must survive entity→document→entity round-trip")
}

func TestAnalysisMapperRoundTrip_EmergentPhenomenaEmpty(t *testing.T) {
	// Old docs (no emergent_phenomena field) must deserialize to nil — backward compat.
	a := simulations.Analysis{
		ID:           bson.NewObjectID().Hex(),
		SimulationID: bson.NewObjectID().Hex(),
		Name:         "Old Analysis",
		Language:     simulations.AnalysisLanguageEN,
		LLMTarget:    common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
		Status:       simulations.AnalysisStatusCompleted,
		Report: &simulations.AnalysisReport{
			Sections: []simulations.AnalysisSection{
				{Key: "dynamics", Title: "Dynamics", Narrative: "stable"},
				// EmergentPhenomena intentionally omitted (nil)
			},
		},
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Millisecond),
		CreatedBy: "user-2",
	}
	doc, err := AnalysisToDocument(a)
	require.NoError(t, err)

	got := AnalysisFromDocument(doc)
	require.NotNil(t, got.Report)
	require.Nil(t, got.Report.Sections[0].EmergentPhenomena, "nil EmergentPhenomena must stay nil (backward compat)")
}
