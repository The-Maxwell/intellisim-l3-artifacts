package llm

import (
	"go.uber.org/fx"
)

var Module = fx.Module(
	"llms",
	fx.Provide(
		NewRegistry,
		NewTokenEstimator,
		NewContextFitter,
		NewService,
		NewProviderBridge,
		NewLLMAPI,
	),
)
