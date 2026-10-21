package chats

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// ModuleConfig is the chats module's environment configuration.
type ModuleConfig struct {
	// GenerationTimeout bounds one background LLM generation.
	GenerationTimeout time.Duration `env:"CHATS_GENERATION_TIMEOUT" envDefault:"180s"`
	// HistoryLimit caps how many recent messages are sent as LLM context
	// (the ContextFitter then trims further to the model window).
	HistoryLimit int `env:"CHATS_HISTORY_LIMIT" envDefault:"50"`
}

func NewModuleConfig() (ModuleConfig, error) {
	cfg, err := env.ParseAs[ModuleConfig]()
	if err != nil {
		return ModuleConfig{}, fmt.Errorf("parse chats config: %w", err)
	}
	return cfg, nil
}
