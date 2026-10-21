package simulations

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
)

func TestRenderAnalysisPDF_SmokeCyrillic(t *testing.T) {
	a := &datasims.Analysis{
		Name: "Аналіз Хижак-Жертва", Language: datasims.AnalysisLanguageUK,
		LLMTarget:                  common.LLMTarget{ProviderUID: "openai", ModelUID: "o3"},
		SimulationStatusAtAnalysis: datasims.SimulationStatusFinished,
		StepsCovered:               datasims.AnalysisStepsCovered{From: 0, To: 100, Sampling: 1, Count: 101},
	}
	pack := EvidencePack{
		Model:      ModelSummary{Name: "Predator-Prey", IsSpatial: true, Archetypes: []ArchetypeSummary{{UID: "prey", Name: "Жертва"}}},
		Run:        RunSummary{Name: "Run A", StepsCompleted: 100},
		Statistics: &datasims.SimulationStatistics{StepsCompleted: 100, TotalAgents: 7},
		Population: []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"prey": 50}}, {Step: 100, Count: map[string]int{"prey": 5}}},
	}
	report := datasims.AnalysisReport{
		ExecutiveSummary: "Популяція жертв скоротилася.",
		Sections:         []datasims.AnalysisSection{{Key: "emergence", Title: "Емерджентність", Narrative: "Спостерігалися коливання.", KeyFindings: []string{"колапс"}}},
		Conclusions:      []string{"Система нестабільна."},
		Recommendations:  []string{"Зменшити швидкість розмноження."},
	}
	data, name, warns, err := renderAnalysisPDF(a, pack, report)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	require.True(t, bytes.HasPrefix(data, []byte("%PDF")), "output must be a PDF")
	require.Contains(t, name, ".pdf")
	require.Empty(t, warns, "both-point population series must chart without warnings")
}

// A report whose only chartable series are too short must still render, but
// each dropped chart must surface as a chart_skipped warning (previously charts
// were dropped silently — the "empty PDF charts" bug's visibility half).
func TestRenderAnalysisPDF_ChartSkippedWarnings(t *testing.T) {
	a := &datasims.Analysis{Name: "One Point", Language: datasims.AnalysisLanguageEN}
	pack := EvidencePack{
		Model:      ModelSummary{Name: "M", Archetypes: []ArchetypeSummary{{UID: "a1", Name: "Agent"}}},
		Run:        RunSummary{Name: "Run"},
		Population: []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"a1": 3}}},
		PropertySeries: []PropertySeries{
			{PropertyUID: "energy", Name: "energy", Points: []datasims.TimeSeriesPoint{{Step: 0, Value: 1.0}}},
		},
	}
	data, _, warns, err := renderAnalysisPDF(a, pack, datasims.AnalysisReport{})
	require.NoError(t, err)
	require.NotEmpty(t, data)
	require.Len(t, warns, 2)
	for _, w := range warns {
		require.Equal(t, "render", w.Stage)
		require.Equal(t, "chart_skipped", w.Code)
	}
}

// Multi-line narratives must not leak raw newline characters into single rows
// (maroto renders \n as tofu boxes).
func TestParagraphsSplitsNewlines(t *testing.T) {
	rows := paragraphs("First paragraph.\n\nSecond paragraph.\r\nThird.")
	require.Len(t, rows, 3)
	require.Equal(t, "no newlines", flattenText("no\nnewlines"))
}
