package simulations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	datasims "intelli.sim.core/data/simulations"
)

func TestBuildSectionPrompts(t *testing.T) {
	pack := EvidencePack{
		Model:      ModelSummary{Name: "Predator-Prey", Archetypes: []ArchetypeSummary{{UID: "prey", Name: "Prey"}}},
		Run:        RunSummary{Name: "Run A", StepsCompleted: 100},
		Population: []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"prey": 50}}},
	}
	sys, user := buildSectionPrompts(secEmergence, pack, "investigate collapse", datasims.AnalysisLanguageEN)
	require.Contains(t, sys, "agent-based")
	require.Contains(t, strings.ToLower(sys), "json")
	require.Contains(t, sys, "English")
	require.Contains(t, user, "Predator-Prey")
	require.Contains(t, user, "investigate collapse")

	sysUK, _ := buildSectionPrompts(secDynamics, pack, "", datasims.AnalysisLanguageUK)
	require.Contains(t, sysUK, "Ukrainian")
}

func TestBuildSynthesisPrompts(t *testing.T) {
	sections := []datasims.AnalysisSection{{Title: "Dynamics", Narrative: "population fell"}}
	pack := EvidencePack{Run: RunSummary{Name: "Run A"}}
	sys, user := buildSynthesisPrompts(sections, pack, "investigate collapse", datasims.AnalysisLanguageEN)
	require.Contains(t, strings.ToLower(sys), "json")
	require.Contains(t, user, "population fell")
}

func TestBuildSynthesisPrompts_ForwardsCausalAndEmergent(t *testing.T) {
	sections := []datasims.AnalysisSection{{
		Title:             "Dynamics",
		Narrative:         "n",
		KeyFindings:       []string{"kf"},
		CausalLinks:       []string{"drought → prey decline"},
		EmergentPhenomena: []string{"predator-prey oscillation"},
	}}
	_, user := buildSynthesisPrompts(sections, EvidencePack{}, "", datasims.AnalysisLanguageEN)
	if !strings.Contains(user, "drought → prey decline") {
		t.Errorf("synthesis user message missing causal links")
	}
	if !strings.Contains(user, "predator-prey oscillation") {
		t.Errorf("synthesis user message missing emergent phenomena")
	}
}

func TestMarshalEvidence_ShapeSeries(t *testing.T) {
	pack := EvidencePack{
		ShapePropertySeries: []ShapePropertySeries{{
			ShapeUID: "lake", ShapeName: "Lake", PropertyUID: "water_level", PropertyName: "Water level",
			Points: []datasims.TimeSeriesPoint{{Step: 0, Value: 50.0}},
		}},
	}
	for _, k := range []sectionKey{secDynamics, secEmergence} {
		if s := marshalEvidence(pack, k); !strings.Contains(s, "shape_property_series") {
			t.Fatalf("section %v evidence missing shape_property_series: %s", k, s)
		}
	}
	if s := marshalEvidence(EvidencePack{}, secDynamics); strings.Contains(s, "shape_property_series") {
		t.Fatalf("empty pack must omit shape_property_series: %s", s)
	}
	if s := marshalEvidence(pack, secSpatial); strings.Contains(s, "shape_property_series") {
		t.Fatalf("spatial section must not carry shape series: %s", s)
	}
}

func TestBuildSectionPrompts_CitationInstruction(t *testing.T) {
	sys, _ := buildSectionPrompts(secDynamics, EvidencePack{}, "", datasims.AnalysisLanguageEN)
	if !strings.Contains(sys, "step indices") {
		t.Errorf("section system prompt missing the citation instruction")
	}
}
