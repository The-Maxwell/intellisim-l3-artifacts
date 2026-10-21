package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"go.uber.org/zap"
)

type Config struct {
	// Provider identity
	ProviderID string `env:"PROVIDER_ID" envDefault:"openai"`
	Name       string `env:"PROVIDER_NAME" envDefault:"OpenAI"`

	// Core gRPC server (must match core's GRPC_SERVER_PORT)
	CoreHost string `env:"GRPC_HOST" envDefault:"localhost"`
	CorePort int    `env:"GRPC_PORT" envDefault:"50051"`

	// OpenAI API
	OpenAIAPIKey   string `env:"OPENAI_API_KEY,required"`
	OpenAIBaseURL  string `env:"OPENAI_BASE_URL" envDefault:"https://api.openai.com/v1"`
	TimeoutSeconds int    `env:"OPENAI_TIMEOUT_SECONDS" envDefault:"60"`
	MaxRetries     int    `env:"OPENAI_MAX_RETRIES" envDefault:"3"`
}

// CoreAddr returns the host:port of the core gRPC server.
func (c *Config) CoreAddr() string { return fmt.Sprintf("%s:%d", c.CoreHost, c.CorePort) }

func Load(logger *zap.Logger) (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if cfg.ProviderID == "" {
		return nil, fmt.Errorf("PROVIDER_ID is required")
	}
	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 60
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 3
	}
	if cfg.OpenAIBaseURL == "" {
		cfg.OpenAIBaseURL = "https://api.openai.com/v1"
	}

	logger.Info("Config loaded",
		zap.String("provider_id", cfg.ProviderID),
		zap.String("core_addr", cfg.CoreAddr()),
		zap.String("openai_base_url", cfg.OpenAIBaseURL),
	)
	return cfg, nil
}
