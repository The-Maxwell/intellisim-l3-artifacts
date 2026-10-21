// Copyright (c) 2026 Vladyslav Pryshchepa
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// Noncommercial use only. Commercial licensing: vlad.prishhepa@gmail.com

package sdk

import (
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"

	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
)

// Options configures a Provider. CoreAddr, Registration, and Handler are required.
type Options struct {
	CoreAddr     string
	Registration Registration
	Handler      Handler
	Logger       *slog.Logger

	InitialBackoff time.Duration // default 1s — doubled per failed connect, ±20% jitter
	MaxBackoff     time.Duration // default 30s
	DefaultTimeout time.Duration // default 60s — per invocation when core sends no timeout
	DrainTimeout   time.Duration // default 10s — wait for in-flight handlers on shutdown

	// DialOptions fully replaces the SDK's default dial options when non-empty
	// (defaults: insecure transport, keepalive 15s/5s, 32 MB max message sizes).
	DialOptions []grpc.DialOption
}

// Provider maintains the connection to core and dispatches invocations to the handler.
type Provider struct {
	opts    Options
	logger  *slog.Logger
	reg     *llmbridgev1.ConnectRequest
	handler Handler
	chatH   ChatHandler      // nil if the handler does not implement it
	visionH VisionHandler    // nil if not implemented
	reasonH ReasoningHandler // nil if not implemented
}

// New validates the options, the registration, and the capability⇒handler
// requirements, failing fast on any configuration error.
func New(opts Options) (*Provider, error) {
	if opts.CoreAddr == "" {
		return nil, fmt.Errorf("sdk: Options.CoreAddr is required")
	}
	if opts.Handler == nil {
		return nil, fmt.Errorf("sdk: Options.Handler is required")
	}
	if err := validateRegistration(opts.Registration); err != nil {
		return nil, err
	}

	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.InitialBackoff <= 0 {
		opts.InitialBackoff = 1 * time.Second
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 30 * time.Second
	}
	if opts.DefaultTimeout <= 0 {
		opts.DefaultTimeout = 60 * time.Second
	}
	if opts.DrainTimeout <= 0 {
		opts.DrainTimeout = 10 * time.Second
	}

	p := &Provider{
		opts:    opts,
		logger:  opts.Logger.With("component", "llmprovider-sdk", "provider_uid", opts.Registration.UID),
		reg:     registrationToProto(opts.Registration),
		handler: opts.Handler,
	}
	p.chatH, _ = opts.Handler.(ChatHandler)
	p.visionH, _ = opts.Handler.(VisionHandler)
	p.reasonH, _ = opts.Handler.(ReasoningHandler)

	if err := p.validateCapabilityHandlers(); err != nil {
		return nil, err
	}
	return p, nil
}

func validateRegistration(r Registration) error {
	if r.UID == "" {
		return fmt.Errorf("sdk: Registration.UID is required")
	}
	if len(r.Models) == 0 {
		return fmt.Errorf("sdk: registration requires at least one model")
	}
	seen := make(map[string]bool, len(r.Models))
	for i, m := range r.Models {
		if m.UID == "" {
			return fmt.Errorf("sdk: model #%d: model UID is required", i)
		}
		if seen[m.UID] {
			return fmt.Errorf("sdk: duplicate model UID %q", m.UID)
		}
		seen[m.UID] = true
		if len(m.Capabilities) == 0 {
			return fmt.Errorf("sdk: model %q: at least one capability is required", m.UID)
		}
		for _, c := range m.Capabilities {
			if !validCapability(c) {
				return fmt.Errorf("sdk: model %q: unknown capability %q", m.UID, c)
			}
		}
		for _, e := range m.Efforts {
			if !validEffort(e) {
				return fmt.Errorf("sdk: model %q: unknown effort %q", m.UID, e)
			}
		}
		if err := validateConfigParams(m.UID, m.ConfigParams); err != nil {
			return err
		}
	}
	return nil
}

func validateConfigParams(modelUID string, params []ConfigParam) error {
	seen := make(map[string]bool, len(params))
	for _, p := range params {
		if p.Key == "" {
			return fmt.Errorf("sdk: model %q: config param has empty key", modelUID)
		}
		if seen[p.Key] {
			return fmt.Errorf("sdk: model %q: duplicate config param key %q", modelUID, p.Key)
		}
		seen[p.Key] = true
		switch p.Type {
		case ConfigParamDouble, ConfigParamInt, ConfigParamUint, ConfigParamBool,
			ConfigParamString, ConfigParamEnum, ConfigParamStringList:
		default:
			return fmt.Errorf("sdk: model %q: config param %q has unknown type %q", modelUID, p.Key, p.Type)
		}
		if p.Type == ConfigParamEnum && len(p.EnumValues) == 0 {
			return fmt.Errorf("sdk: model %q: enum config param %q has no enum_values", modelUID, p.Key)
		}
	}
	return nil
}

// validateCapabilityHandlers enforces: declared capability ⇒ implemented handler.
func (p *Provider) validateCapabilityHandlers() error {
	for _, m := range p.opts.Registration.Models {
		for _, c := range m.Capabilities {
			switch c {
			case CapabilityChat, CapabilityModelGeneration:
				if p.chatH == nil {
					return fmt.Errorf("sdk: model %q declares capability %q but handler does not implement sdk.ChatHandler", m.UID, c)
				}
			case CapabilityVisionAnalysis:
				if p.visionH == nil {
					return fmt.Errorf("sdk: model %q declares capability %q but handler does not implement sdk.VisionHandler", m.UID, c)
				}
			case CapabilityResultsAnalysis:
				if p.reasonH == nil {
					return fmt.Errorf("sdk: model %q declares capability %q but handler does not implement sdk.ReasoningHandler", m.UID, c)
				}
			case CapabilitySimulationGeneration, CapabilitySimulationExecution, CapabilitySimulationDecisions:
				// Simple only — guaranteed by the Handler interface itself.
				// (Reasoning is used opportunistically when the model supports it,
				// but is not required, matching simulation_execution semantics.)
			}
		}
	}
	return nil
}
