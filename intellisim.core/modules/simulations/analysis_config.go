package simulations

import (
	"time"

	"github.com/caarlos0/env/v11"
	"go.uber.org/zap"
)

// AnalysisConfig holds env-driven knobs for the Level-3 analysis pipeline.
type AnalysisConfig struct {
	// Timeout is the per-LLM-call ceiling when the target carries no TimeoutMs.
	TimeoutSeconds int `env:"SIMULATIONS_ANALYSIS_TIMEOUT" envDefault:"300"`
	// MaxSeriesPoints caps points per time-series in the evidence pack (down-sampled to fit).
	MaxSeriesPoints int `env:"SIMULATIONS_ANALYSIS_MAX_POINTS" envDefault:"60"`
	// MaxRetries is the per-section LLM retry budget on transport/JSON failure.
	MaxRetries int `env:"SIMULATIONS_ANALYSIS_MAX_RETRIES" envDefault:"1"`
}

func (c AnalysisConfig) Timeout() time.Duration { return time.Duration(c.TimeoutSeconds) * time.Second }

// NewAnalysisConfig parses AnalysisConfig from the environment (FX provider).
func NewAnalysisConfig(logger *zap.Logger) AnalysisConfig {
	cfg := AnalysisConfig{}
	if err := env.Parse(&cfg); err != nil {
		logger.Named("simulations").Warn("failed to parse analysis config; using defaults", zap.Error(err))
		cfg = AnalysisConfig{TimeoutSeconds: 300, MaxSeriesPoints: 60, MaxRetries: 1}
	}
	return cfg
}
