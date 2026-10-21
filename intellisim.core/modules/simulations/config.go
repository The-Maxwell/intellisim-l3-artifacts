package simulations

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// ModuleConfig holds simulations-module runtime knobs (env-driven).
type ModuleConfig struct {
	// GenerationTimeout bounds one background simulation-generation LLM call.
	// Used as the per-LLM-call ceiling when a target carries no explicit TimeoutMs.
	GenerationTimeout time.Duration `env:"SIMULATIONS_GENERATION_TIMEOUT" envDefault:"300s"`
	// LLMBatchSize controls how many agents are batched per LLM request during
	// generation (population/properties/placement steps).
	LLMBatchSize int `env:"SIMULATIONS_LLM_BATCH_SIZE" envDefault:"40"`
}

func NewModuleConfig() (ModuleConfig, error) {
	cfg, err := env.ParseAs[ModuleConfig]()
	if err != nil {
		return ModuleConfig{}, fmt.Errorf("parse simulations config: %w", err)
	}
	return cfg, nil
}
