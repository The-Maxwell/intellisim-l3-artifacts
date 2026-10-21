package simulations

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// RunConfig holds the engine run-phase runtime knobs (env-driven). It mirrors
// ModuleConfig's parsing style and is provided separately so the create/edit
// path and the run path can evolve their config independently.
type RunConfig struct {
	// MaxConcurrentRuns caps the number of simultaneous in-memory engine runs.
	MaxConcurrentRuns int `env:"SIMULATIONS_MAX_CONCURRENT_RUNS" envDefault:"4"`
	// StepFlushSize is the number of buffered step documents that triggers a
	// batched timeseries write.
	StepFlushSize int `env:"SIMULATIONS_STEP_FLUSH_SIZE" envDefault:"50"`
	// StepFlushIntervalMs bounds how long buffered steps wait before a flush.
	StepFlushIntervalMs int `env:"SIMULATIONS_STEP_FLUSH_INTERVAL_MS" envDefault:"500"`
	// DefaultStepDelayMs is the default inter-step delay (run speed) when the
	// client does not request one.
	DefaultStepDelayMs int `env:"SIMULATIONS_DEFAULT_STEP_DELAY_MS" envDefault:"100"`
	// StreamTicketTTLSec bounds the lifetime of a single-use WebSocket connect
	// ticket.
	StreamTicketTTLSec int `env:"SIMULATIONS_STREAM_TICKET_TTL_S" envDefault:"30"`
	// DefaultParallelWorkers is the default per-run LLM concurrency when the
	// model/sim config does not specify one.
	DefaultParallelWorkers int `env:"SIMULATIONS_DEFAULT_PARALLEL_WORKERS" envDefault:"4"`
	// WSAllowedOrigin is the allowed Origin for the run WebSocket upgrade.
	WSAllowedOrigin string `env:"WS_ALLOWED_ORIGIN" envDefault:"http://localhost:3000"`
	// GenerationTimeout is the per-LLM-call ceiling applied during a run when
	// neither the resolved target nor the run-level config supplies one. Sourced
	// from the same SIMULATIONS_GENERATION_TIMEOUT env knob as ModuleConfig so
	// the same value governs both generation and run-time LLM calls.
	GenerationTimeout time.Duration `env:"SIMULATIONS_GENERATION_TIMEOUT" envDefault:"300s"`
}

// NewRunConfig parses RunConfig from the environment, mirroring NewModuleConfig.
func NewRunConfig() (RunConfig, error) {
	cfg, err := env.ParseAs[RunConfig]()
	if err != nil {
		return RunConfig{}, fmt.Errorf("parse simulations run config: %w", err)
	}
	return cfg, nil
}
