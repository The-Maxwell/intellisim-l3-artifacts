package models

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
)

func TestBuildSystemPrompt_ScopeAndLocks(t *testing.T) {
	m := datamodels.Model{Agents: []datamodels.Agent{
		{UID: "p", Name: "Pred", Locked: true},
		{UID: "q", Name: "Quarry"},
	}}

	p := buildSystemPrompt(m, EditScope{Section: "agents"})

	require.Contains(t, p, "agents")  // the agents section schema is present
	require.Contains(t, p, "FROZEN")  // locked-entity block header
	require.Contains(t, p, "p")       // locked uid listed
	require.Contains(t, p, "agent p") // locked summary line for the locked agent

	// Out-of-scope schema sections are trimmed away.
	require.NotContains(t, p, "interactions[]:")
	require.NotContains(t, p, "rules[]:")
	require.NotContains(t, p, "events[]:")
	require.NotContains(t, p, "widgets[]:")
	require.NotContains(t, p, "environment: {")

	// The unlocked agent ("q") must NOT appear in the FROZEN list.
	frozen := p[strings.Index(p, "FROZEN"):]
	require.NotContains(t, frozen, "agent q")

	// The current model slice is embedded as valid JSON containing the agents.
	require.Contains(t, p, "CURRENT MODEL")
	require.Contains(t, p, "\"uid\":\"p\"")
}

func TestBuildSystemPrompt_WholeModelIncludesAllSchema(t *testing.T) {
	p := buildSystemPrompt(datamodels.Model{}, EditScope{})
	require.Contains(t, p, "agents[]:")
	require.Contains(t, p, "rules[]:")
	require.Contains(t, p, "interactions[]:")
	require.Contains(t, p, "events[]:")
	require.Contains(t, p, "widgets[]:")
	require.Contains(t, p, "display_message")
}

func TestBuildSystemPrompt_AgentsSchemaIncludesGenerationAndCountMode(t *testing.T) {
	p := buildSystemPrompt(datamodels.Model{}, EditScope{Section: "agents"})
	require.Contains(t, p, "generation?")
	require.Contains(t, p, "count_mode?")
}

func TestBuildSystemPrompt_WidgetsScope(t *testing.T) {
	m := datamodels.Model{Widgets: []common.WidgetConfig{{
		ID: "pop_chart", Type: common.WidgetLineChart, Title: "Population",
		Series: []common.WidgetSeries{{
			Label:  "wolves",
			Source: common.WidgetSource{Type: common.WidgetSourcePopulation, AgentUIDs: []string{"wolf"}},
		}},
		Position: common.WidgetPosition{X: 0, Y: 0, W: 4, H: 2},
	}}}

	p := buildSystemPrompt(m, EditScope{Section: "widgets"})

	// The widgets section schema (incl. the source matrix) is present...
	require.Contains(t, p, "widgets[]:")
	require.Contains(t, p, "source matrix")
	// ...and out-of-scope sections are trimmed away.
	require.NotContains(t, p, "agents[]:")
	require.NotContains(t, p, "rules[]:")

	// The current widget slice is embedded for the LLM to edit against.
	require.Contains(t, p, "CURRENT MODEL")
	require.Contains(t, p, "\"pop_chart\"")
}

func TestCurrentSliceJSON_IncludesGenerationAndCountMode(t *testing.T) {
	mean := 50.0
	std := 10.0
	m := datamodels.Model{Agents: []datamodels.Agent{{
		UID: "wolf", Type: datamodels.AgentTypePlain,
		Properties: []datamodels.AgentProperty{{
			UID: "energy", Type: datamodels.PropertyTypeNumber,
			Generation: &datamodels.PropertyGeneration{
				Mode: datamodels.GenModeRandom, Distribution: datamodels.DistNormal,
				Mean: &mean, StdDev: &std,
			},
		}},
		Initialization: datamodels.AgentInitialization{PopulationCount: 5, CountMode: datamodels.CountLLM},
	}}}

	slice := currentSliceJSON(m, EditScope{Section: "agents"})

	require.Contains(t, slice, "\"generation\"")
	require.Contains(t, slice, "\"std_dev\":10")
	require.Contains(t, slice, "\"count_mode\":\"llm\"")
}

func TestBuildSystemPrompt_EntityScopeNarrowsCurrentSlice(t *testing.T) {
	m := datamodels.Model{Agents: []datamodels.Agent{
		{UID: "wolf", Name: "Wolf"},
		{UID: "sheep", Name: "Sheep"},
	}}
	p := buildSystemPrompt(m, EditScope{Section: "agents", UID: "wolf"})
	require.Contains(t, p, "\"uid\":\"wolf\"")
	require.NotContains(t, p, "\"uid\":\"sheep\"")
	require.Contains(t, p, "\"wolf\"") // scope instruction names the entity
}

func TestBuildSystemPrompt_SemanticsLegendAndLanguage(t *testing.T) {
	sys := buildSystemPrompt(datamodels.Model{}, EditScope{})
	if !strings.Contains(sys, "## SEMANTICS") {
		t.Errorf("system prompt missing semantics legend")
	}
	if !strings.Contains(sys, "researcher's language") {
		t.Errorf("system prompt missing display_message language instruction")
	}
	// Widgets-only scope: rules/interactions absent → legend omitted.
	sys = buildSystemPrompt(datamodels.Model{}, EditScope{Section: "widgets"})
	if strings.Contains(sys, "## SEMANTICS") {
		t.Errorf("widgets-only scope must not carry the rules/actions legend")
	}
}

func TestBuildSystemPrompt_SchemaNudges(t *testing.T) {
	p := buildSystemPrompt(datamodels.Model{}, EditScope{})

	// Geometry scalar contract is explicit (bare "radius": 5 kills the envelope).
	require.Contains(t, p, "geometry width/height/radius are OBJECTS")
	require.Contains(t, p, "never a bare number")

	// Interaction action-side semantics (predation nudge).
	require.Contains(t, p, "initiator_actions apply to the INITIATOR, target_actions to the TARGET")
	require.Contains(t, p, "remove_agent belongs in target_actions")
}

func TestBuildSystemPrompt_ResearcherGuidance(t *testing.T) {
	m := datamodels.Model{}
	m.LLMConfig.PromptContext = "  Prefer predator-prey dynamics with realistic ENERGY_MARKER budgets.  "

	p := buildSystemPrompt(m, EditScope{Section: "agents"})

	// The guidance heading and the (trimmed) context are injected.
	require.Contains(t, p, "## RESEARCHER GUIDANCE")
	require.Contains(t, p, "ENERGY_MARKER")
	// Trimmed: the leading/trailing whitespace of the raw context is stripped.
	require.Contains(t, p, "Prefer predator-prey dynamics with realistic ENERGY_MARKER budgets.\n")
	// The guidance appears before the scope instruction.
	require.Less(t, strings.Index(p, "## RESEARCHER GUIDANCE"), strings.Index(p, "## SCOPE"))
}

func TestBuildSystemPrompt_ResearcherGuidanceAbsentWhenEmpty(t *testing.T) {
	// Empty context → no guidance section.
	p := buildSystemPrompt(datamodels.Model{}, EditScope{})
	require.NotContains(t, p, "## RESEARCHER GUIDANCE")

	// Whitespace-only context → still no guidance section.
	m := datamodels.Model{}
	m.LLMConfig.PromptContext = "   \n\t  "
	p = buildSystemPrompt(m, EditScope{})
	require.NotContains(t, p, "## RESEARCHER GUIDANCE")
}

func TestBuildVisionSystemPrompt_ResearcherGuidance(t *testing.T) {
	env := datamodels.Environment{Width: 10, Height: 10}

	// Empty context → no guidance section.
	require.NotContains(t, buildVisionSystemPrompt(env, ""), "## RESEARCHER GUIDANCE")

	// Non-empty (untrimmed) context → trimmed guidance section injected.
	p := buildVisionSystemPrompt(env, "  Treat dark cells as OBSTACLE_MARKER walls.  ")
	require.Contains(t, p, "## RESEARCHER GUIDANCE")
	require.Contains(t, p, "Treat dark cells as OBSTACLE_MARKER walls.\n")
}

func TestCurrentSliceJSON_IsValidJSON(t *testing.T) {
	m := datamodels.Model{Agents: []datamodels.Agent{{UID: "a", Name: "A", Type: datamodels.AgentTypePlain}}}
	slice := currentSliceJSON(m, EditScope{})
	require.NotEmpty(t, slice)

	var decoded LLMModel
	require.NoError(t, json.Unmarshal([]byte(slice), &decoded))
	require.Len(t, decoded.Agents, 1)
	require.Equal(t, "a", decoded.Agents[0].UID)
}
