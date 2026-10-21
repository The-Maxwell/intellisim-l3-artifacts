package models

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// ModuleConfig holds models-module runtime knobs (env-driven).
type ModuleConfig struct {
	// GenerationTimeout bounds one background model-generation LLM call.
	GenerationTimeout time.Duration `env:"MODELS_GENERATION_TIMEOUT" envDefault:"300s"`
	// HistoryLimit caps how many recent model-chat messages are sent as LLM context.
	HistoryLimit int `env:"MODELS_HISTORY_LIMIT" envDefault:"30"`
	// MaxImageBytes caps the decoded size of a single uploaded reference image BEFORE
	// compression (default 6 MiB). Oversized uploads are rejected before imageutil runs.
	MaxImageBytes int `env:"MODELS_MAX_IMAGE_BYTES" envDefault:"6291456"`
	// MaxModelImages caps how many environment-generation images a model may hold
	// (managed library; default 5). Adding past this ⇒ 422 too_many_images.
	MaxModelImages int `env:"MODELS_MAX_MODEL_IMAGES" envDefault:"5"`
	// ModelImageMaxDim is the max pixel dimension a stored generation image is
	// downscaled to on upload (default 1536 — ample for vision analysis).
	ModelImageMaxDim int `env:"MODELS_MODEL_IMAGE_MAX_DIM" envDefault:"1536"`
	// ModelImageMaxBytes caps the compressed generation image (base64-decoded)
	// kept on the model document (default 1.5 MiB). 5 × 1.5 MiB stays under Mongo's
	// 16 MiB document limit. Over this after compression ⇒ 422 image_too_large.
	ModelImageMaxBytes int `env:"MODELS_MODEL_IMAGE_MAX_BYTES" envDefault:"1572864"`
	// DisplayImageMaxDim is the max pixel dimension an entity display image is downscaled to
	// before storage (default 256).
	DisplayImageMaxDim int `env:"MODELS_DISPLAY_IMAGE_MAX_DIM" envDefault:"256"`
	// DisplayImageMaxBytes caps the compressed display image (base64-decoded) accepted for
	// storage (default 256 KiB). Over this after compression ⇒ 422.
	DisplayImageMaxBytes int `env:"MODELS_DISPLAY_IMAGE_MAX_BYTES" envDefault:"262144"`
}

func NewModuleConfig() (ModuleConfig, error) {
	cfg, err := env.ParseAs[ModuleConfig]()
	if err != nil {
		return ModuleConfig{}, fmt.Errorf("parse models config: %w", err)
	}
	return cfg, nil
}
