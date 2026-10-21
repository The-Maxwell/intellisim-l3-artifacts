package common

// ModelCapability is a model purpose from the closed list shared with LLM
// providers (see the llmbridge proto). Custom values are not allowed.
type ModelCapability string

const (
	ModelCapabilityChat                 ModelCapability = "chat"
	ModelCapabilityModelGeneration      ModelCapability = "model_generation"
	ModelCapabilitySimulationGeneration ModelCapability = "simulation_generation"
	ModelCapabilitySimulationExecution  ModelCapability = "simulation_execution"
	ModelCapabilitySimulationDecisions  ModelCapability = "simulation_decisions"
	ModelCapabilityResultsAnalysis      ModelCapability = "results_analysis"
	ModelCapabilityVisionAnalysis       ModelCapability = "vision_analysis"
)

// AllModelCapabilities lists every valid capability value.
var AllModelCapabilities = []ModelCapability{
	ModelCapabilityChat,
	ModelCapabilityModelGeneration,
	ModelCapabilitySimulationGeneration,
	ModelCapabilitySimulationExecution,
	ModelCapabilitySimulationDecisions,
	ModelCapabilityResultsAnalysis,
	ModelCapabilityVisionAnalysis,
}

// Valid reports whether c is one of the closed-list values.
func (c ModelCapability) Valid() bool {
	for _, v := range AllModelCapabilities {
		if c == v {
			return true
		}
	}
	return false
}
