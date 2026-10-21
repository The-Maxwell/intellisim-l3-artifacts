package simulations

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/image"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/extension"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/core/entity"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"intelli.sim.core/assets"
	datasims "intelli.sim.core/data/simulations"
)

const pdfFontFamily = "dejavu"

// renderAnalysisPDF builds the comprehensive analysis report PDF and returns the
// bytes plus a suggested filename. Embeds a Cyrillic-capable TTF so uk reports render.
// Charts that cannot be produced are reported as chart_skipped warnings instead of
// being dropped silently.
func renderAnalysisPDF(a *datasims.Analysis, pack EvidencePack, report datasims.AnalysisReport) ([]byte, string, []datasims.AnalysisWarning, error) {
	var warns []datasims.AnalysisWarning
	chartWarn := func(what, reason string) {
		warns = append(warns, datasims.AnalysisWarning{
			Stage: "render", Code: "chart_skipped",
			Message: what + " omitted: " + reason,
		})
	}
	cfg := config.NewBuilder().
		WithCustomFonts([]*entity.CustomFont{
			{Family: pdfFontFamily, Style: fontstyle.Normal, Bytes: assets.DejaVuSans},
			{Family: pdfFontFamily, Style: fontstyle.Bold, Bytes: assets.DejaVuSansBold},
		}).
		WithDefaultFont(&props.Font{Family: pdfFontFamily}).
		Build()

	m := maroto.New(cfg)

	// --- Title page ---
	m.AddRows(
		text.NewRow(16, "IntelliSim — Simulation Analysis Report", props.Text{Size: 20, Style: fontstyle.Bold, Align: align.Center}),
		text.NewRow(8, pack.Run.Name, props.Text{Size: 14, Align: align.Center}),
	)
	m.AddRows(metaRows(a, pack)...)

	// --- Executive summary ---
	if report.ExecutiveSummary != "" {
		m.AddRows(heading("Executive Summary"))
		m.AddRows(paragraphs(report.ExecutiveSummary)...)
	}

	// --- Model & run overview ---
	m.AddRows(heading("Simulation & Model Overview"))
	m.AddRows(paragraphs(modelOverviewText(pack))...)

	// --- Quantitative metrics ---
	m.AddRows(heading("Quantitative Metrics"))
	for _, r := range statisticsRows(pack) {
		m.AddRows(r)
	}

	// --- Charts ---
	archNames := archetypeNames(pack)
	if png, reason := renderPopulationChart(pack.Population, archNames); png != nil {
		m.AddRows(heading("Population Over Time"))
		m.AddRows(image.NewFromBytesRow(80, png, extension.Png, props.Rect{Center: true, Percent: 100}))
	} else {
		chartWarn("population chart", reason)
	}
	for _, ps := range pack.PropertySeries {
		if png, reason := renderPropertyChart(ps); png != nil {
			m.AddRows(image.NewFromBytesRow(70, png, extension.Png, props.Rect{Center: true, Percent: 100}))
		} else {
			chartWarn("chart for property \""+ps.Name+"\"", reason)
		}
	}
	for _, sps := range pack.ShapePropertySeries {
		ps := PropertySeries{PropertyUID: sps.PropertyUID, Name: sps.ShapeName + ": " + sps.PropertyName, Points: sps.Points}
		if png, reason := renderPropertyChart(ps); png != nil {
			m.AddRows(image.NewFromBytesRow(70, png, extension.Png, props.Rect{Center: true, Percent: 100}))
		} else {
			chartWarn("chart for shape property \""+ps.Name+"\"", reason)
		}
	}

	// --- Sections ---
	for _, s := range report.Sections {
		m.AddRows(heading(s.Title))
		if s.Narrative != "" {
			m.AddRows(paragraphs(s.Narrative)...)
		}
		if len(s.KeyFindings) > 0 {
			m.AddRows(subheading("Key findings"))
			for _, f := range s.KeyFindings {
				m.AddRows(bullet(f))
			}
		}
		if len(s.CausalLinks) > 0 {
			m.AddRows(subheading("Causal links"))
			for _, c := range s.CausalLinks {
				m.AddRows(bullet(c))
			}
		}
		if len(s.EmergentPhenomena) > 0 {
			m.AddRows(subheading("Emergent phenomena"))
			for _, ep := range s.EmergentPhenomena {
				m.AddRows(bullet(ep))
			}
		}
	}

	// --- Conclusions & recommendations ---
	if len(report.Conclusions) > 0 {
		m.AddRows(heading("Conclusions"))
		for _, c := range report.Conclusions {
			m.AddRows(bullet(c))
		}
	}
	if len(report.Recommendations) > 0 {
		m.AddRows(heading("Recommendations"))
		for _, r := range report.Recommendations {
			m.AddRows(bullet(r))
		}
	}

	doc, err := m.Generate()
	if err != nil {
		return nil, "", warns, fmt.Errorf("generate pdf: %w", err)
	}
	name := sanitizeFilename(a.Name) + ".pdf"
	return doc.GetBytes(), name, warns, nil
}

// archetypeNames maps archetype UIDs to display names for chart legends and
// statistics rows (the evidence keys population data by UID).
func archetypeNames(pack EvidencePack) map[string]string {
	names := make(map[string]string, len(pack.Model.Archetypes))
	for _, ar := range pack.Model.Archetypes {
		names[ar.UID] = ar.Name
	}
	return names
}

func heading(s string) core.Row {
	// Headings wrap at ~55 chars (size 15 bold); a fixed height makes multi-line
	// headings overlap the text below them.
	lines := (utf8.RuneCountInString(s)-1)/55 + 1
	h := 12 + float64(lines-1)*7
	return text.NewRow(h, s, props.Text{Size: 15, Style: fontstyle.Bold, Top: 4})
}
func subheading(s string) core.Row {
	return text.NewRow(8, s, props.Text{Size: 12, Style: fontstyle.Bold, Top: 2})
}
func paragraph(s string) core.Row {
	return text.NewRow(rowHeightFor(s), s, props.Text{Size: 10, Align: align.Left})
}

// paragraphs splits multi-line text into one paragraph row per line. Maroto does
// not interpret newline characters — they would render as tofu glyph boxes.
func paragraphs(s string) []core.Row {
	var rows []core.Row
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		rows = append(rows, paragraph(line))
	}
	if len(rows) == 0 {
		rows = append(rows, paragraph(""))
	}
	return rows
}

func bullet(s string) core.Row {
	return text.NewRow(rowHeightFor(s), "•  "+flattenText(s), props.Text{Size: 10, Align: align.Left})
}

// flattenText collapses newlines into spaces for single-row contexts.
func flattenText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// rowHeightFor estimates a row height from text length (maroto rows are fixed-height).
// Rune-based so Cyrillic text (2 bytes per char) doesn't double the estimate.
func rowHeightFor(s string) float64 {
	lines := utf8.RuneCountInString(s)/90 + 1
	h := float64(lines) * 5.0
	if h < 6 {
		h = 6
	}
	return h
}

func metaRows(a *datasims.Analysis, pack EvidencePack) []core.Row {
	target := a.LLMTarget.ProviderUID + " · " + a.LLMTarget.ModelUID
	if a.LLMTarget.Effort != "" {
		target += " (" + string(a.LLMTarget.Effort) + ")"
	}
	lines := []string{
		"Model: " + pack.Model.Name,
		"Analysis language: " + string(a.Language),
		"LLM: " + target,
		"Simulation status at analysis: " + string(a.SimulationStatusAtAnalysis),
		fmt.Sprintf("Steps covered: %d–%d (sampling %d, %d points)", a.StepsCovered.From, a.StepsCovered.To, a.StepsCovered.Sampling, a.StepsCovered.Count),
	}
	if strings.TrimSpace(a.ContextMessage) != "" {
		lines = append(lines, "Context: "+flattenText(a.ContextMessage))
	}
	rows := make([]core.Row, 0, len(lines))
	for _, l := range lines {
		rows = append(rows, text.NewRow(rowHeightFor(l), l, props.Text{Size: 9, Top: 1}))
	}
	return rows
}

func modelOverviewText(pack EvidencePack) string {
	m := pack.Model
	var b strings.Builder
	fmt.Fprintf(&b, "Spatial: %v. Update mode: %s. Archetypes: %d. Rules: %d. Interactions: %d. Events: %d. ",
		m.IsSpatial, m.UpdateMode, len(m.Archetypes), m.RuleCount, m.Interactions, m.Events)
	if m.Environment != nil {
		fmt.Fprintf(&b, "Environment: %dx%d (torus=%v, shapes=%d). ", m.Environment.Width, m.Environment.Height, m.Environment.Torus, m.Environment.ShapeCount)
	}
	names := make([]string, 0, len(m.Archetypes))
	for _, a := range m.Archetypes {
		names = append(names, a.Name)
	}
	if len(names) > 0 {
		fmt.Fprintf(&b, "Archetypes: %s.", strings.Join(names, ", "))
	}
	return b.String()
}

func statisticsRows(pack EvidencePack) []core.Row {
	if pack.Statistics == nil {
		return []core.Row{paragraph("No aggregate statistics were recorded.")}
	}
	st := pack.Statistics
	rows := []core.Row{
		paragraph(fmt.Sprintf("Steps completed: %d. Total agents (final): %d. Spawned: %d. Removed: %d. Interactions: %d. Events: %d. Rule executions: %d.",
			st.StepsCompleted, st.TotalAgents, st.AgentsSpawned, st.AgentsRemoved, st.Interactions, st.Events, st.RuleExecutions)),
	}
	names := archetypeNames(pack)
	for _, p := range st.Population {
		label := names[p.AgentUID]
		if label == "" {
			label = p.AgentUID
		}
		rows = append(rows, paragraph(fmt.Sprintf("Archetype %s — current %d, peak %d, min %d, spawned %d, removed %d.",
			label, p.Current, p.Peak, p.Min, p.Spawned, p.Removed)))
	}
	return rows
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "analysis"
	}
	repl := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return repl.Replace(name)
}
