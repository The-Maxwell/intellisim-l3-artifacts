package simulations

import (
	"encoding/json"
	"fmt"
	"strings"

	datasims "intelli.sim.core/data/simulations"
)

// sectionKey identifies an analysis section (one focused LLM call each).
type sectionKey string

const (
	secDynamics  sectionKey = "dynamics"  // population & property trends over time
	secSpatial   sectionKey = "spatial"   // spatial distribution & movement
	secEmergence sectionKey = "emergence" // emergent behavior & causal relationships (always)
	secEvents    sectionKey = "events"    // event emission patterns over the run
	secRules     sectionKey = "rules"     // rule firing frequency & dominant behaviors
)

func languageName(l datasims.AnalysisLanguage) string {
	if l == datasims.AnalysisLanguageUK {
		return "Ukrainian"
	}
	return "English"
}

// sectionGuide returns the per-section title + analytical objective injected into the prompt.
func sectionGuide(k sectionKey) (title, objective string) {
	switch k {
	case secDynamics:
		return "Population & Property Dynamics",
			"Analyze how agent populations and key numeric properties evolved over time. " +
				"Identify growth/decline phases, oscillations, equilibria, tipping points, and correlations between series." +
				" If shape_property_series is present in the evidence, treat environment-shape properties (resources, water bodies, zones) as first-class dynamics and relate their trends to agent behavior."
	case secSpatial:
		return "Spatial & Interaction Patterns",
			"Analyze spatial distribution, clustering, movement and agent interactions. " +
				"Identify aggregation/segregation, territory formation, and how spatial structure shaped outcomes."
	case secEvents:
		return "Event Emission Patterns",
			"Analyze the events fired during the run using the per-event emission counts (ByEvent). " +
				"Identify which events dominated, temporal clustering or bursts, which agent populations triggered them, " +
				"and the downstream consequences of high-frequency versus rare events."
	case secRules:
		return "Rule Firing & Dominant Behaviors",
			"Analyze rule execution frequency using the per-rule counts (ByRule). " +
				"Identify the dominant rules that shaped system behavior, rules that rarely fired (near-dead paths), " +
				"competitive dynamics between rules (priority conflicts, mutual suppression), and correlate rule activations " +
				"with population trends to explain behavioral phases."
	default: // secEmergence
		return "Emergent Behavior & Causal Relationships",
			"Identify EMERGENT phenomena (system-level behavior not reducible to individual rules) and " +
				"establish CAUSE-AND-EFFECT chains between events. Go beyond restating statistics: explain WHY the system behaved as it did. " +
				"Place each distinct emergent phenomenon — self-organisation, phase transitions, oscillations, cascades, etc. — as a concise statement in the emergent_phenomena list." +
				" If shape_property_series is present, consider agent–environment feedback loops (e.g. agents depleting or replenishing shape resources) as candidate causal chains."
	}
}

// buildSectionPrompts returns the (system, user) prompts for one analysis section.
func buildSectionPrompts(k sectionKey, pack EvidencePack, contextMessage string, lang datasims.AnalysisLanguage) (string, string) {
	title, objective := sectionGuide(k)
	lname := languageName(lang)

	var sys strings.Builder
	fmt.Fprintf(&sys, "You are an expert analyst of agent-based modeling (ABM) simulations working inside the IntelliSim meta-system.\n\n")
	sys.WriteString("DOMAIN ONTOLOGY: A model defines agent ARCHETYPES (templates) with typed PROPERTIES (number/string/enum/boolean/color) and a state machine; ")
	sys.WriteString("a RULE ENGINE (condition types incl. property/position/proximity/state/count/decision; action types incl. set/modify property, move, transition, spawn/remove, emit_event, call_llm); ")
	sys.WriteString("INTERACTIONS (agent-agent/shape/global); EVENTS; and a spatial ENVIRONMENT (grid, optional torus, shapes, global properties). ")
	sys.WriteString("A simulation runs in discrete steps; per-step state was recorded to a time series.\n\n")
	fmt.Fprintf(&sys, "YOUR TASK — section \"%s\": %s\n\n", title, objective)
	sys.WriteString("DATA: You are given AGGREGATED time series and statistics (not raw per-agent frames). Ground EVERY claim strictly in the provided evidence; never invent numbers or entities. Cite concrete numbers and step indices/ranges from the evidence when making claims. Prefer causal and emergent insight over restating values. Be concise, specific, and structured.\n\n")
	fmt.Fprintf(&sys, "OUTPUT: Respond with a SINGLE JSON object and nothing else, in this exact shape:\n")
	sys.WriteString(`{"title": string, "narrative": string, "key_findings": [string], "causal_links": [string], "emergent_phenomena": [string]}` + "\n")
	sys.WriteString("- narrative: 2–5 paragraphs of analysis.\n- key_findings: 3–7 short bullet insights.\n- causal_links: cause→effect statements (may be empty if none are evidenced).\n- emergent_phenomena: system-level behaviors not reducible to individual rules; may be empty.\n")
	fmt.Fprintf(&sys, "Write all natural-language content in %s.\n", lname)

	var user strings.Builder
	user.WriteString("=== SIMULATION EVIDENCE (JSON) ===\n")
	user.WriteString(marshalEvidence(pack, k))
	user.WriteString("\n=== END EVIDENCE ===\n")
	if strings.TrimSpace(contextMessage) != "" {
		fmt.Fprintf(&user, "\nRESEARCHER CONTEXT / QUESTIONS:\n%s\n", strings.TrimSpace(contextMessage))
	}
	fmt.Fprintf(&user, "\nProduce the \"%s\" section now as the specified JSON.", title)
	return sys.String(), user.String()
}

// buildSynthesisPrompts returns the (system, user) prompts for the final synthesis call.
func buildSynthesisPrompts(sections []datasims.AnalysisSection, pack EvidencePack, contextMessage string, lang datasims.AnalysisLanguage) (string, string) {
	lname := languageName(lang)
	var sys strings.Builder
	sys.WriteString("You are an expert ABM simulation analyst writing the conclusion of a results-analysis report for a researcher.\n\n")
	sys.WriteString("You are given the section analyses already produced for this simulation plus its headline statistics. ")
	sys.WriteString("Synthesize them into an executive summary, overall conclusions, and actionable recommendations. Ground everything in the provided material; do not introduce unsupported claims.\n\n")
	sys.WriteString("OUTPUT: Respond with a SINGLE JSON object and nothing else, in this exact shape:\n")
	sys.WriteString(`{"executive_summary": string, "conclusions": [string], "recommendations": [string]}` + "\n")
	sys.WriteString("- executive_summary: 1–2 tight paragraphs.\n- conclusions: 3–6 bullets.\n- recommendations: 3–6 actionable bullets (model/parameter/further-investigation suggestions).\n")
	fmt.Fprintf(&sys, "Write all content in %s.\n", lname)

	var user strings.Builder
	fmt.Fprintf(&user, "SIMULATION: %s (steps completed: %d)\n\n", pack.Run.Name, pack.Run.StepsCompleted)
	user.WriteString("=== SECTION ANALYSES ===\n")
	for _, s := range sections {
		fmt.Fprintf(&user, "## %s\n%s\n", s.Title, s.Narrative)
		if len(s.KeyFindings) > 0 {
			fmt.Fprintf(&user, "Key findings: %s\n", strings.Join(s.KeyFindings, "; "))
		}
		if len(s.CausalLinks) > 0 {
			fmt.Fprintf(&user, "Causal links: %s\n", strings.Join(s.CausalLinks, "; "))
		}
		if len(s.EmergentPhenomena) > 0 {
			fmt.Fprintf(&user, "Emergent phenomena: %s\n", strings.Join(s.EmergentPhenomena, "; "))
		}
		user.WriteString("\n")
	}
	user.WriteString("=== HEADLINE STATISTICS (JSON) ===\n")
	user.WriteString(marshalStatistics(pack))
	if strings.TrimSpace(contextMessage) != "" {
		fmt.Fprintf(&user, "\nRESEARCHER CONTEXT / QUESTIONS:\n%s\n", strings.TrimSpace(contextMessage))
	}
	user.WriteString("\nProduce the synthesis now as the specified JSON.")
	return sys.String(), user.String()
}

// marshalEvidence serializes the slice of the evidence pack relevant to a section.
func marshalEvidence(pack EvidencePack, k sectionKey) string {
	payload := map[string]any{
		"model": pack.Model,
		"run":   pack.Run,
	}
	switch k {
	case secDynamics:
		payload["population_over_time"] = pack.Population
		payload["property_series"] = pack.PropertySeries
		payload["statistics"] = pack.Statistics
		if len(pack.ShapePropertySeries) > 0 {
			payload["shape_property_series"] = pack.ShapePropertySeries
		}
	case secSpatial:
		payload["statistics"] = pack.Statistics
		payload["spatial_snapshots"] = pack.SpatialSnapshots
	case secEmergence:
		payload["population_over_time"] = pack.Population
		payload["property_series"] = pack.PropertySeries
		payload["statistics"] = pack.Statistics
		if len(pack.ShapePropertySeries) > 0 {
			payload["shape_property_series"] = pack.ShapePropertySeries
		}
	case secEvents:
		if pack.Statistics != nil {
			payload["by_event"] = pack.Statistics.ByEvent
		}
		payload["population_over_time"] = pack.Population
	case secRules:
		if pack.Statistics != nil {
			payload["by_rule"] = pack.Statistics.ByRule
		}
		payload["population_over_time"] = pack.Population
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func marshalStatistics(pack EvidencePack) string {
	b, err := json.Marshal(map[string]any{"statistics": pack.Statistics, "population_over_time": pack.Population})
	if err != nil {
		return "{}"
	}
	return string(b)
}
