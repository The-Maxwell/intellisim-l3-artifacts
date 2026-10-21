package simulations

import (
	"encoding/json"
	"strings"
	"testing"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// extractJSONExample pulls the balanced { ... } block that follows the
// "Example:" marker in a system prompt and returns it. The schema line itself
// carries <placeholder> tokens (not valid JSON); the Example: line is the
// concrete, parseable sample, so the test confirms THAT one round-trips.
func extractJSONExample(t *testing.T, s string) string {
	t.Helper()
	marker := strings.LastIndex(s, "Example:")
	if marker < 0 {
		t.Fatalf("no Example: marker found in:\n%s", s)
	}
	s = s[marker:]
	start := strings.IndexByte(s, '{')
	if start < 0 {
		t.Fatalf("no JSON object found after Example: in:\n%s", s)
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	t.Fatalf("unbalanced JSON object in:\n%s", s)
	return ""
}

func assertParses(t *testing.T, jsonStr string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(jsonStr), into); err != nil {
		t.Fatalf("embedded JSON example failed to parse: %v\njson: %s", err, jsonStr)
	}
}

// promptModel builds a small spatial model with a recognizable prompt-context
// chain so tests can assert guidance injection.
func promptModel() datamodels.Model {
	return datamodels.Model{
		Name:        "Predator-Prey",
		Description: "A classic ecology model.",
		IsSpatial:   true,
		LLMConfig: datamodels.LLMConfig{
			PromptContext: "MODEL_CTX_MARKER", // model-generation context — must NOT leak into sim-generation prompts
		},
		SimulationConfig: datamodels.SimulationConfig{
			LLMSimulationGenerationConfig: datamodels.LLMConfig{
				PromptContext: "SIMGEN_CTX_MARKER",
			},
		},
		Environment: &datamodels.Environment{
			Width:            100,
			Height:           80,
			LLMPromptContext: "ENV_CTX_MARKER",
		},
	}
}

func promptAgent() datamodels.Agent {
	return datamodels.Agent{
		UID:         "wolf",
		Name:        "Wolf",
		Description: "A hungry predator.",
		Properties: []datamodels.AgentProperty{
			{UID: "energy", Name: "Energy", Type: datamodels.PropertyTypeNumber, MinValue: f64ptr(0), MaxValue: f64ptr(100), LLMPromptContext: "ENERGY_CTX_MARKER"},
		},
		Initialization: datamodels.AgentInitialization{
			PopulationCount: 20,
			MinPopulation:   u64ptr(5),
			MaxPopulation:   u64ptr(50),
			Placement:       &datamodels.Placement{Type: datamodels.PlacementLLM, PromptContext: "PLACEMENT_CTX_MARKER"},
		},
		LLMConfig: &datamodels.LLMAgentConfig{
			LLMConfig: datamodels.LLMConfig{PromptContext: "AGENT_CTX_MARKER"},
		},
	}
}

const strictJSONHeader = "Output STRICT JSON only"

func TestBuildCountPrompt(t *testing.T) {
	m := promptModel()
	agent := promptAgent()
	sys, user := buildCountPrompt(m, agent, map[string]uint64{"sheep": 100})

	if !strings.Contains(sys, strictJSONHeader) {
		t.Errorf("count system prompt missing strict-JSON header")
	}
	if !strings.Contains(sys, `"count"`) {
		t.Errorf("count system prompt missing schema marker \"count\"")
	}
	// Population/count guidance now reuses the initialization placement prompt context.
	if !strings.Contains(user, "PLACEMENT_CTX_MARKER") {
		t.Errorf("count user message missing placement (population) prompt context")
	}
	if !strings.Contains(user, "SIMGEN_CTX_MARKER") {
		t.Errorf("user message missing sim-generation prompt context")
	}
	if strings.Contains(user, "MODEL_CTX_MARKER") {
		t.Errorf("model-generation prompt context leaked into sim-generation prompt")
	}
	if !strings.Contains(user, "sheep") {
		t.Errorf("count user message missing other-archetype resolved count")
	}
	if !strings.Contains(user, "capacity") {
		t.Errorf("count user message missing environment-capacity nudge")
	}

	var resp LLMCountResponse
	assertParses(t, extractJSONExample(t, sys), &resp)
}

func TestBuildGlobalValuesPrompt(t *testing.T) {
	m := promptModel()
	props := []datamodels.AgentProperty{
		{UID: "temperature", Name: "Temperature", Type: datamodels.PropertyTypeNumber, MinValue: f64ptr(-10), MaxValue: f64ptr(40), LLMPromptContext: "TEMP_CTX_MARKER"},
		{UID: "season", Type: datamodels.PropertyTypeEnum, AllowedValues: []interface{}{"summer", "winter"}},
	}
	sys, user := buildGlobalValuesPrompt(m, props)

	if !strings.Contains(sys, strictJSONHeader) {
		t.Errorf("globals system prompt missing strict-JSON header")
	}
	if !strings.Contains(sys, `"values"`) {
		t.Errorf("globals system prompt missing schema marker \"values\"")
	}
	if !strings.Contains(user, "TEMP_CTX_MARKER") {
		t.Errorf("globals user message missing per-property guidance")
	}
	if !strings.Contains(user, "temperature") {
		t.Errorf("globals user message missing property uid")
	}
	if !strings.Contains(user, "SIMGEN_CTX_MARKER") {
		t.Errorf("user message missing sim-generation prompt context")
	}
	if strings.Contains(user, "MODEL_CTX_MARKER") {
		t.Errorf("model-generation prompt context leaked into sim-generation prompt")
	}

	var resp LLMGlobalValuesResponse
	assertParses(t, extractJSONExample(t, sys), &resp)
}

func TestBuildAgentBatchPrompt(t *testing.T) {
	m := promptModel()
	agent := promptAgent()
	llmProps := []datamodels.AgentProperty{
		{UID: "mood", Type: datamodels.PropertyTypeEnum, AllowedValues: []interface{}{"calm", "angry"}, LLMPromptContext: "MOOD_CTX_MARKER"},
	}
	sys, user := buildAgentBatchPrompt(m, agent, llmProps, 0, 8, "CTX_SUMMARY_MARKER")

	if !strings.Contains(sys, strictJSONHeader) {
		t.Errorf("batch system prompt missing strict-JSON header")
	}
	if !strings.Contains(sys, `"instances"`) {
		t.Errorf("batch system prompt missing schema marker \"instances\"")
	}
	if !strings.Contains(user, "MOOD_CTX_MARKER") {
		t.Errorf("batch user message missing property guidance")
	}
	if !strings.Contains(user, "CTX_SUMMARY_MARKER") {
		t.Errorf("batch user message missing ctx summary")
	}
	if !strings.Contains(user, "internally consistent") {
		t.Errorf("batch user message missing per-instance consistency nudge")
	}
	if !strings.Contains(user, "SIMGEN_CTX_MARKER") {
		t.Errorf("user message missing sim-generation prompt context")
	}
	if strings.Contains(user, "MODEL_CTX_MARKER") {
		t.Errorf("model-generation prompt context leaked into sim-generation prompt")
	}

	var resp LLMAgentBatchResponse
	assertParses(t, extractJSONExample(t, sys), &resp)
}

func TestBuildPlacementPrompt(t *testing.T) {
	m := promptModel()
	agent := promptAgent()
	env := &datasims.SimulationEnvironment{
		Width:  100,
		Height: 80,
		Torus:  true,
		Shapes: []datasims.SimulationShape{
			{ShapeUID: "lake", Name: "Lake", Type: datamodels.ShapeTypeCircle, Position: common.Point{X: 50, Y: 40}, Radius: f64ptr(10)},
		},
	}
	sys, user := buildPlacementPrompt(m, agent, 30, env, "CTX_SUMMARY_MARKER")

	if !strings.Contains(sys, strictJSONHeader) {
		t.Errorf("placement system prompt missing strict-JSON header")
	}
	if !strings.Contains(sys, `"directives"`) {
		t.Errorf("placement system prompt missing schema marker \"directives\"")
	}
	if !strings.Contains(user, "PLACEMENT_CTX_MARKER") {
		t.Errorf("placement user message missing placement prompt context")
	}
	if !strings.Contains(user, "Lake") {
		t.Errorf("placement user message missing shape summary")
	}
	if !strings.Contains(user, "CTX_SUMMARY_MARKER") {
		t.Errorf("placement user message missing ctx summary")
	}
	if !strings.Contains(user, "SIMGEN_CTX_MARKER") {
		t.Errorf("user message missing sim-generation prompt context")
	}
	if strings.Contains(user, "MODEL_CTX_MARKER") {
		t.Errorf("model-generation prompt context leaked into sim-generation prompt")
	}

	var resp LLMPlacementResponse
	assertParses(t, extractJSONExample(t, sys), &resp)
}

// TestBuildPrompts_NilSafe asserts the builders never panic on nil LLMConfig
// agents, non-spatial models, and missing environments/placements.
func TestBuildPrompts_NilSafe(t *testing.T) {
	m := datamodels.Model{Name: "Bare", IsSpatial: false}
	agent := datamodels.Agent{UID: "a", Name: "A"} // nil LLMConfig, nil Placement

	// Should not panic.
	buildCountPrompt(m, agent, nil)
	buildGlobalValuesPrompt(m, nil)
	buildAgentBatchPrompt(m, agent, nil, 0, 1, "")
	buildPlacementPrompt(m, agent, 0, nil, "")
}
