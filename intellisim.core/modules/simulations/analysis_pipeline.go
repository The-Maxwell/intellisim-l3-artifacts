package simulations

import (
	"context"

	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
)

// pdfRenderer renders the final PDF. Injectable so the pipeline is testable
// without the maroto dependency / embedded fonts. warns carries non-fatal
// render degradations (e.g. skipped charts) even when err is nil.
type pdfRenderer func(a *datasims.Analysis, pack EvidencePack, report datasims.AnalysisReport) (data []byte, filename string, warns []datasims.AnalysisWarning, err error)

type analysisPipeline struct {
	steps  datasims.SimulationStepsRepository
	caller *analysisCaller
	cfg    AnalysisConfig
	logger *zap.Logger
	render pdfRenderer
}

func newAnalysisPipeline(steps datasims.SimulationStepsRepository, inv analysisInvoker, cfg AnalysisConfig, logger *zap.Logger) *analysisPipeline {
	return &analysisPipeline{
		steps:  steps,
		caller: newAnalysisCaller(inv, cfg, logger),
		cfg:    cfg,
		logger: logger.Named("analysis-pipeline"),
		render: renderAnalysisPDF,
	}
}

type pipelineResult struct {
	Report       datasims.AnalysisReport
	StepsCovered datasims.AnalysisStepsCovered
	Usage        *common.TokenUsageSummary
	Requests     uint
	Warnings     []datasims.AnalysisWarning
	PDF          []byte
	FileName     string
}

// run executes the full analysis pipeline. progress(stage, percent) is called
// between stages. It returns an error only on unrecoverable failure (no data, or
// every LLM call failing AND no usable report); otherwise it degrades fail-soft.
func (p *analysisPipeline) run(ctx context.Context, sim *datasims.Simulation, a *datasims.Analysis, sel datasims.AnalysisSelection, progress func(stage string, percent uint8)) (*pipelineResult, error) {
	usage := &common.TokenUsageSummary{}
	res := &pipelineResult{Usage: usage}

	progress("Assembling evidence", 10)
	pack, covered, err := buildEvidence(ctx, p.steps, sim, sel, p.cfg)
	if err != nil {
		return nil, err
	}
	// No data in the selected window — abort rather than produce an empty,
	// hallucination-prone report.
	if len(pack.Population) == 0 && len(pack.PropertySeries) == 0 && len(pack.SpatialSnapshots) == 0 {
		return nil, ErrNoResults
	}
	res.StepsCovered = covered

	sections := p.plannedSections(sim, sel)
	res.Warnings = append(res.Warnings, p.skippedSections(sel, sections)...)
	report := datasims.AnalysisReport{}
	produced := 0
	base := uint8(15)
	span := uint8(60) // 15..75 for sections
	for i, k := range sections {
		title, _ := sectionGuide(k)
		pct := base + uint8(float64(span)*float64(i)/float64(len(sections)))
		progress("Analyzing: "+title, pct)
		sys, user := buildSectionPrompts(k, pack, a.ContextMessage, a.Language)
		var out llmSectionResponse
		res.Requests++
		if p.caller.callJSON(ctx, a.LLMTarget, sys, user, usage, &out) {
			report.Sections = append(report.Sections, datasims.AnalysisSection{
				Key: string(k), Title: firstNonEmpty(out.Title, title),
				Narrative: out.Narrative, KeyFindings: out.KeyFindings, CausalLinks: out.CausalLinks,
				EmergentPhenomena: out.EmergentPhenomena,
			})
			produced++
		} else {
			res.Warnings = append(res.Warnings, datasims.AnalysisWarning{
				Stage: "section:" + string(k), Code: "section_failed",
				Message: "the LLM did not return a valid analysis for this section",
			})
			report.Sections = append(report.Sections, datasims.AnalysisSection{
				Key: string(k), Title: title, Narrative: "This section could not be generated.",
			})
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	progress("Synthesizing conclusions", 80)
	res.Requests++
	var syn llmSynthesisResponse
	sysP, userP := buildSynthesisPrompts(report.Sections, pack, a.ContextMessage, a.Language)
	if p.caller.callJSON(ctx, a.LLMTarget, sysP, userP, usage, &syn) {
		report.ExecutiveSummary = syn.ExecutiveSummary
		report.Conclusions = syn.Conclusions
		report.Recommendations = syn.Recommendations
		produced++
	} else {
		res.Warnings = append(res.Warnings, datasims.AnalysisWarning{
			Stage: "synthesis", Code: "synthesis_failed",
			Message: "the LLM did not return a valid synthesis",
		})
	}

	if produced == 0 {
		return nil, ErrAllLLMCallsFailed
	}
	res.Report = report

	progress("Rendering PDF", 90)
	// The renderer reads selection/target/report/steps-covered off the analysis
	// record; stamp the freshly-computed values BEFORE rendering, otherwise the
	// PDF header prints the pre-run zero StepsCovered (the service only copies
	// res.StepsCovered onto the record after run() returns).
	a.Report = &report
	a.StepsCovered = covered
	pdf, name, renderWarns, err := p.render(a, pack, report)
	res.Warnings = append(res.Warnings, renderWarns...)
	if err != nil {
		return nil, err
	}
	res.PDF = pdf
	res.FileName = name
	progress("Completed", 100)
	return res, nil
}

// plannedSections decides which sections to run from the model + selection focus.
func (p *analysisPipeline) plannedSections(sim *datasims.Simulation, sel datasims.AnalysisSelection) []sectionKey {
	var out []sectionKey
	if focusEnabled(sel, datasims.FocusPopulation) || focusEnabled(sel, datasims.FocusProperties) {
		out = append(out, secDynamics)
	}
	spatial := sim.ModelSnapshot != nil && sim.ModelSnapshot.IsSpatial
	if spatial && (focusEnabled(sel, datasims.FocusSpatial) || focusEnabled(sel, datasims.FocusInteractions)) {
		out = append(out, secSpatial)
	}
	// secEvents and secRules are only added when EXPLICITLY selected (len>0 guard keeps
	// the no-focus default unchanged) AND the corresponding statistics slice is non-empty.
	// Warnings for the "selected but no data" case are emitted by run() via skippedSections.
	if len(sel.FocusAreas) > 0 && containsFocus(sel.FocusAreas, datasims.FocusEvents) {
		stats := sim.Statistics
		if stats != nil && len(stats.ByEvent) > 0 {
			out = append(out, secEvents)
		}
	}
	if len(sel.FocusAreas) > 0 && containsFocus(sel.FocusAreas, datasims.FocusRules) {
		stats := sim.Statistics
		if stats != nil && len(stats.ByRule) > 0 {
			out = append(out, secRules)
		}
	}
	out = append(out, secEmergence) // always
	return out
}

// skippedSections returns warnings for explicitly-focused sections that have no data
// and therefore were not planned. Called by run() after plannedSections.
func (p *analysisPipeline) skippedSections(sel datasims.AnalysisSelection, planned []sectionKey) []datasims.AnalysisWarning {
	var warns []datasims.AnalysisWarning
	plannedSet := make(map[sectionKey]bool, len(planned))
	for _, k := range planned {
		plannedSet[k] = true
	}
	if len(sel.FocusAreas) > 0 && containsFocus(sel.FocusAreas, datasims.FocusEvents) && !plannedSet[secEvents] {
		warns = append(warns, datasims.AnalysisWarning{
			Stage:   "section:" + string(secEvents),
			Code:    "events_no_data",
			Message: "events focus area selected but no event statistics (ByEvent) are recorded for this simulation; section skipped",
		})
	}
	if len(sel.FocusAreas) > 0 && containsFocus(sel.FocusAreas, datasims.FocusRules) && !plannedSet[secRules] {
		warns = append(warns, datasims.AnalysisWarning{
			Stage:   "section:" + string(secRules),
			Code:    "rules_no_data",
			Message: "rules focus area selected but no rule statistics (ByRule) are recorded for this simulation; section skipped",
		})
	}
	return warns
}

// containsFocus reports whether areas contains the given focus area.
func containsFocus(areas []datasims.AnalysisFocusArea, area datasims.AnalysisFocusArea) bool {
	for _, a := range areas {
		if a == area {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
