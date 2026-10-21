package simulations

// llmSectionResponse is the strict JSON shape returned by a sectioned analysis call.
type llmSectionResponse struct {
	Title             string   `json:"title"`
	Narrative         string   `json:"narrative"`
	KeyFindings       []string `json:"key_findings"`
	CausalLinks       []string `json:"causal_links"`
	EmergentPhenomena []string `json:"emergent_phenomena"`
}

// llmSynthesisResponse is the strict JSON shape returned by the final synthesis call.
type llmSynthesisResponse struct {
	ExecutiveSummary string   `json:"executive_summary"`
	Conclusions      []string `json:"conclusions"`
	Recommendations  []string `json:"recommendations"`
}
