package simulations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
)

func TestPipeline_RunProducesReportAndPDF(t *testing.T) {
	repo := &fakeStepsRepo{
		min: 0, max: 50,
		props:  []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		pop:    []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"prey": 50}}, {Step: 50, Count: map[string]int{"prey": 3}}},
		series: []datasims.TimeSeriesPoint{{Step: 0, Value: 9.0}, {Step: 50, Value: 1.0}},
	}
	inv := &fakeAnalysisInvoker{reasoning: true,
		analysis: `{"title":"Sec","narrative":"the prey collapsed","key_findings":["collapse"],"causal_links":["overgrazing -> collapse"]}`}
	caller := newAnalysisCaller(inv, AnalysisConfig{MaxRetries: 0, TimeoutSeconds: 30}, zapNop())
	p := &analysisPipeline{steps: repo, caller: caller, cfg: AnalysisConfig{MaxSeriesPoints: 60},
		logger: zapNop(),
		render: func(_ *datasims.Analysis, _ EvidencePack, r datasims.AnalysisReport) ([]byte, string, []datasims.AnalysisWarning, error) {
			return []byte("%PDF-1.7 fake"), "report.pdf", nil, nil
		},
	}
	sim := &datasims.Simulation{ID: "sim1", Name: "Run A", ModelSnapshot: testModel(),
		Steps: 50, Status: datasims.SimulationStatusFinished,
		Statistics: &datasims.SimulationStatistics{StepsCompleted: 50}}
	a := &datasims.Analysis{Language: datasims.AnalysisLanguageEN, ContextMessage: "why collapse?",
		LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "o3"}}

	stages := []string{}
	res, err := p.run(context.Background(), sim, a, datasims.AnalysisSelection{}, func(stage string, pct uint8) { stages = append(stages, stage) })
	require.NoError(t, err)
	require.NotEmpty(t, res.Report.Sections)
	require.NotEmpty(t, res.PDF)
	require.Equal(t, "report.pdf", res.FileName)
	require.NotNil(t, res.Usage)
	require.Greater(t, res.Requests, uint(0))
	require.NotEmpty(t, stages)
}

// TestPipeline_RendererSeesStepsCovered reproduces the "Steps covered: 0–0
// (sampling 0, 0 points)" PDF header bug: the renderer reads StepsCovered off the
// analysis record, but the pipeline only returned it on the pipelineResult (the
// service stamps a.StepsCovered AFTER run() — and thus after the PDF is rendered).
// The analysis handed to the renderer must already carry the effective window.
func TestPipeline_RendererSeesStepsCovered(t *testing.T) {
	// Recorded range 0–5 → 6 population points, sampling 1 (mirrors the E2E run).
	pop := make([]datasims.PopulationPoint, 0, 6)
	for s := int64(0); s <= 5; s++ {
		pop = append(pop, datasims.PopulationPoint{Step: s, Count: map[string]int{"prey": 10}})
	}
	repo := &fakeStepsRepo{
		min: 0, max: 5,
		props: []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		pop:   pop,
	}
	inv := &fakeAnalysisInvoker{reasoning: true,
		analysis: `{"title":"Sec","narrative":"steady","key_findings":["stable"],"causal_links":[]}`}
	caller := newAnalysisCaller(inv, AnalysisConfig{MaxRetries: 0, TimeoutSeconds: 30}, zapNop())

	var atRender datasims.AnalysisStepsCovered
	p := &analysisPipeline{steps: repo, caller: caller, cfg: AnalysisConfig{MaxSeriesPoints: 60},
		logger: zapNop(),
		render: func(a *datasims.Analysis, _ EvidencePack, _ datasims.AnalysisReport) ([]byte, string, []datasims.AnalysisWarning, error) {
			atRender = a.StepsCovered // what metaRows/the PDF header would print
			return []byte("%PDF-1.7 fake"), "report.pdf", nil, nil
		},
	}
	sim := &datasims.Simulation{ID: "sim-cov", Name: "Coverage Run", ModelSnapshot: testModel(),
		Steps: 5, Status: datasims.SimulationStatusFinished,
		Statistics: &datasims.SimulationStatistics{StepsCompleted: 5}}
	a := &datasims.Analysis{Language: datasims.AnalysisLanguageEN,
		LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "o3"}}

	res, err := p.run(context.Background(), sim, a, datasims.AnalysisSelection{}, func(string, uint8) {})
	require.NoError(t, err)

	want := datasims.AnalysisStepsCovered{From: 0, To: 5, Sampling: 1, Count: 6}
	require.Equal(t, want, res.StepsCovered, "pipeline result must carry the effective window")
	require.Equal(t, want, atRender,
		"the analysis passed to the PDF renderer must already carry StepsCovered (otherwise the header prints zeros)")
	require.Equal(t, want, a.StepsCovered, "the analysis record must be stamped by the pipeline")
}

// TestPlannedSections_EventsFocusWithData verifies that selecting FocusEvents with
// non-empty Statistics.ByEvent causes secEvents to be included in the planned sections.
func TestPlannedSections_EventsFocusWithData(t *testing.T) {
	p := &analysisPipeline{logger: zapNop()}
	sim := &datasims.Simulation{
		ModelSnapshot: testModel(),
		Statistics: &datasims.SimulationStatistics{
			ByEvent: []datasims.EventStatistic{{EventUID: "ev1", Emitted: 42}},
		},
	}
	sel := datasims.AnalysisSelection{
		FocusAreas: []datasims.AnalysisFocusArea{datasims.FocusEvents},
	}
	sections := p.plannedSections(sim, sel)

	found := false
	for _, k := range sections {
		if k == secEvents {
			found = true
		}
	}
	require.True(t, found, "secEvents must be planned when FocusEvents is selected and ByEvent is non-empty")
}

// TestPlannedSections_EventsFocusEmptyData verifies that selecting FocusEvents but having
// empty Statistics.ByEvent causes secEvents to be skipped and a warning to be produced
// by the pipeline run (not plannedSections directly — warn is appended in run).
func TestPlannedSections_EventsFocusEmptyData(t *testing.T) {
	p := &analysisPipeline{logger: zapNop()}
	sim := &datasims.Simulation{
		ModelSnapshot: testModel(),
		Statistics:    &datasims.SimulationStatistics{ByEvent: nil},
	}
	sel := datasims.AnalysisSelection{
		FocusAreas: []datasims.AnalysisFocusArea{datasims.FocusEvents},
	}
	sections := p.plannedSections(sim, sel)

	for _, k := range sections {
		require.NotEqual(t, secEvents, k, "secEvents must NOT be planned when ByEvent is empty")
	}
}

// TestPlannedSections_NoFocusDoesNotAddEventsRules verifies that an empty FocusAreas
// (no explicit focus = "all") does NOT auto-add secEvents or secRules.
func TestPlannedSections_NoFocusDoesNotAddEventsRules(t *testing.T) {
	p := &analysisPipeline{logger: zapNop()}
	sim := &datasims.Simulation{
		ModelSnapshot: testModel(),
		Statistics: &datasims.SimulationStatistics{
			ByEvent: []datasims.EventStatistic{{EventUID: "ev1", Emitted: 10}},
			ByRule:  []datasims.RuleStatistic{{RuleID: "r1", Executions: 5}},
		},
	}
	// empty FocusAreas = no explicit focus
	sel := datasims.AnalysisSelection{}
	sections := p.plannedSections(sim, sel)

	for _, k := range sections {
		require.NotEqual(t, secEvents, k, "secEvents must NOT be auto-added when FocusAreas is empty")
		require.NotEqual(t, secRules, k, "secRules must NOT be auto-added when FocusAreas is empty")
	}
}

// TestPlannedSections_RulesFocusWithData verifies that selecting FocusRules with
// non-empty Statistics.ByRule causes secRules to be included in the planned sections.
func TestPlannedSections_RulesFocusWithData(t *testing.T) {
	p := &analysisPipeline{logger: zapNop()}
	sim := &datasims.Simulation{
		ModelSnapshot: testModel(),
		Statistics: &datasims.SimulationStatistics{
			ByRule: []datasims.RuleStatistic{{RuleID: "r1", Executions: 99}},
		},
	}
	sel := datasims.AnalysisSelection{
		FocusAreas: []datasims.AnalysisFocusArea{datasims.FocusRules},
	}
	sections := p.plannedSections(sim, sel)

	found := false
	for _, k := range sections {
		if k == secRules {
			found = true
		}
	}
	require.True(t, found, "secRules must be planned when FocusRules is selected and ByRule is non-empty")
}

// TestPlannedSections_RulesFocusEmptyData verifies that FocusRules with empty ByRule
// does not add secRules to the planned sections.
func TestPlannedSections_RulesFocusEmptyData(t *testing.T) {
	p := &analysisPipeline{logger: zapNop()}
	sim := &datasims.Simulation{
		ModelSnapshot: testModel(),
		Statistics:    &datasims.SimulationStatistics{ByRule: nil},
	}
	sel := datasims.AnalysisSelection{
		FocusAreas: []datasims.AnalysisFocusArea{datasims.FocusRules},
	}
	sections := p.plannedSections(sim, sel)

	for _, k := range sections {
		require.NotEqual(t, secRules, k, "secRules must NOT be planned when ByRule is empty")
	}
}

// TestPipeline_EventsFocusEmptyData_ProducesWarning verifies that the full pipeline
// run with FocusEvents selected but empty ByEvent produces a skip warning.
func TestPipeline_EventsFocusEmptyData_ProducesWarning(t *testing.T) {
	repo := &fakeStepsRepo{
		min:   0,
		max:   10,
		props: []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		pop:   []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"agent": 5}}, {Step: 10, Count: map[string]int{"agent": 3}}},
	}
	inv := &fakeAnalysisInvoker{reasoning: true,
		analysis: `{"title":"Sec","narrative":"ok","key_findings":["f"],"causal_links":[]}`}
	caller := newAnalysisCaller(inv, AnalysisConfig{MaxRetries: 0, TimeoutSeconds: 30}, zapNop())
	p := &analysisPipeline{steps: repo, caller: caller, cfg: AnalysisConfig{MaxSeriesPoints: 60},
		logger: zapNop(),
		render: func(_ *datasims.Analysis, _ EvidencePack, r datasims.AnalysisReport) ([]byte, string, []datasims.AnalysisWarning, error) {
			return []byte("%PDF-1.7 fake"), "report.pdf", nil, nil
		},
	}
	sim := &datasims.Simulation{
		ID: "sim-ev-warn", Name: "EV Warn", ModelSnapshot: testModel(),
		Steps: 10, Status: datasims.SimulationStatusFinished,
		Statistics: &datasims.SimulationStatistics{StepsCompleted: 10, ByEvent: nil},
	}
	a := &datasims.Analysis{Language: datasims.AnalysisLanguageEN,
		LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "o3"}}

	sel := datasims.AnalysisSelection{
		FocusAreas: []datasims.AnalysisFocusArea{datasims.FocusEvents},
	}
	res, err := p.run(context.Background(), sim, a, sel, func(string, uint8) {})
	require.NoError(t, err)

	// A warning about the skipped events section must be present.
	found := false
	for _, w := range res.Warnings {
		if w.Code == "events_no_data" {
			found = true
		}
	}
	require.True(t, found, "pipeline must emit events_no_data warning when FocusEvents is selected but ByEvent is empty")

	// secEvents must not appear in the report sections.
	for _, s := range res.Report.Sections {
		require.NotEqual(t, string(secEvents), s.Key, "secEvents must not appear in report when ByEvent is empty")
	}
}

func TestPipeline_AssemblesEmergentPhenomena(t *testing.T) {
	// The LLM returns emergent_phenomena — verify the pipeline threads it into AnalysisSection.
	repo := &fakeStepsRepo{
		min: 0, max: 10,
		props: []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		pop:   []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"agent": 20}}, {Step: 10, Count: map[string]int{"agent": 5}}},
	}
	inv := &fakeAnalysisInvoker{reasoning: true,
		analysis: `{"title":"Emergence","narrative":"agents cluster","key_findings":["clusters"],"causal_links":["density->cluster"],"emergent_phenomena":["spontaneous clustering","boom-bust cycle"]}`}
	caller := newAnalysisCaller(inv, AnalysisConfig{MaxRetries: 0, TimeoutSeconds: 30}, zapNop())
	p := &analysisPipeline{steps: repo, caller: caller, cfg: AnalysisConfig{MaxSeriesPoints: 60},
		logger: zapNop(),
		render: func(_ *datasims.Analysis, _ EvidencePack, r datasims.AnalysisReport) ([]byte, string, []datasims.AnalysisWarning, error) {
			return []byte("%PDF-1.7 fake"), "out.pdf", nil, nil
		},
	}
	sim := &datasims.Simulation{ID: "sim2", Name: "Cluster Run", ModelSnapshot: testModel(),
		Steps: 10, Status: datasims.SimulationStatusFinished,
		Statistics: &datasims.SimulationStatistics{StepsCompleted: 10}}
	a := &datasims.Analysis{Language: datasims.AnalysisLanguageEN, ContextMessage: "emergence?",
		LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "o3"}}

	res, err := p.run(context.Background(), sim, a, datasims.AnalysisSelection{}, func(string, uint8) {})
	require.NoError(t, err)
	require.NotEmpty(t, res.Report.Sections)

	// Find the emergence section and verify EmergentPhenomena was assembled.
	var emergenceSection *datasims.AnalysisSection
	for i := range res.Report.Sections {
		if res.Report.Sections[i].Key == string(secEmergence) {
			emergenceSection = &res.Report.Sections[i]
			break
		}
	}
	require.NotNil(t, emergenceSection, "emergence section must exist")
	require.Equal(t, []string{"spontaneous clustering", "boom-bust cycle"}, emergenceSection.EmergentPhenomena,
		"pipeline must thread EmergentPhenomena from llmSectionResponse into AnalysisSection")
}
