package sdk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- test handlers ---

type simpleOnly struct{}

func (simpleOnly) Simple(ctx context.Context, req *SimpleRequest) (*Completion, error) {
	return &Completion{Text: "ok"}, nil
}

type fullHandler struct{ simpleOnly }

func (fullHandler) Chat(ctx context.Context, req *ChatRequest) (*Completion, error) {
	return &Completion{Text: "chat:" + req.Message}, nil
}
func (fullHandler) Vision(ctx context.Context, req *VisionRequest) (*Completion, error) {
	return &Completion{Text: "vision"}, nil
}
func (fullHandler) Reasoning(ctx context.Context, req *ReasoningRequest) (*ReasoningCompletion, error) {
	return &ReasoningCompletion{Analysis: "analysis"}, nil
}

func validRegistration() Registration {
	return Registration{
		UID: "test", Name: "Test", Version: "1.0.0",
		Models: []Model{{
			UID: "m1", Name: "M1",
			Capabilities: []Capability{CapabilitySimulationDecisions},
		}},
	}
}

func TestNew_Validation(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*Options)
		wantErr string
	}{
		{"missing core addr", func(o *Options) { o.CoreAddr = "" }, "CoreAddr"},
		{"nil handler", func(o *Options) { o.Handler = nil }, "Handler"},
		{"empty provider uid", func(o *Options) { o.Registration.UID = "" }, "UID"},
		{"no models", func(o *Options) { o.Registration.Models = nil }, "at least one model"},
		{"empty model uid", func(o *Options) { o.Registration.Models[0].UID = "" }, "model UID"},
		{"duplicate model uid", func(o *Options) {
			o.Registration.Models = append(o.Registration.Models, o.Registration.Models[0])
		}, "duplicate"},
		{"no capabilities", func(o *Options) { o.Registration.Models[0].Capabilities = nil }, "capability"},
		{"invalid capability", func(o *Options) {
			o.Registration.Models[0].Capabilities = []Capability{"custom_thing"}
		}, "unknown capability"},
		{"invalid effort", func(o *Options) {
			o.Registration.Models[0].Efforts = []Effort{"ultra"}
		}, "unknown effort"},
		{"chat capability without ChatHandler", func(o *Options) {
			o.Registration.Models[0].Capabilities = []Capability{CapabilityChat}
		}, "sdk.ChatHandler"},
		{"model_generation without ChatHandler", func(o *Options) {
			o.Registration.Models[0].Capabilities = []Capability{CapabilityModelGeneration}
		}, "sdk.ChatHandler"},
		{"vision capability without VisionHandler", func(o *Options) {
			o.Registration.Models[0].Capabilities = []Capability{CapabilityVisionAnalysis}
		}, "sdk.VisionHandler"},
		{"results_analysis without ReasoningHandler", func(o *Options) {
			o.Registration.Models[0].Capabilities = []Capability{CapabilityResultsAnalysis}
		}, "sdk.ReasoningHandler"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := Options{CoreAddr: "localhost:1", Registration: validRegistration(), Handler: simpleOnly{}}
			tc.mutate(&opts)
			_, err := New(opts)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestNew_SimulationCapsNeedOnlySimple(t *testing.T) {
	reg := validRegistration()
	reg.Models[0].Capabilities = []Capability{CapabilitySimulationGeneration, CapabilitySimulationExecution, CapabilitySimulationDecisions}
	_, err := New(Options{CoreAddr: "localhost:1", Registration: reg, Handler: simpleOnly{}})
	assert.NoError(t, err)
}

func TestNew_FullHandlerAllCaps(t *testing.T) {
	reg := validRegistration()
	reg.Models[0].Capabilities = []Capability{CapabilityChat, CapabilityModelGeneration,
		CapabilitySimulationGeneration, CapabilitySimulationExecution, CapabilitySimulationDecisions,
		CapabilityResultsAnalysis, CapabilityVisionAnalysis}
	reg.Models[0].Efforts = []Effort{EffortLow, EffortHigh}
	p, err := New(Options{CoreAddr: "localhost:1", Registration: reg, Handler: fullHandler{}})
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestNew_Defaults(t *testing.T) {
	p, err := New(Options{CoreAddr: "localhost:1", Registration: validRegistration(), Handler: simpleOnly{}})
	assert.NoError(t, err)
	assert.Equal(t, 1*time.Second, p.opts.InitialBackoff)
	assert.Equal(t, 30*time.Second, p.opts.MaxBackoff)
	assert.Equal(t, 60*time.Second, p.opts.DefaultTimeout)
	assert.Equal(t, 10*time.Second, p.opts.DrainTimeout)
	assert.NotNil(t, p.logger)
}

func TestValidateConfigParams_RejectsEnumWithoutValues(t *testing.T) {
	err := validateConfigParams("m1", []ConfigParam{{Key: "response_style", Type: ConfigParamEnum}})
	if err == nil {
		t.Fatal("expected error for enum param with no enum_values")
	}
}

func TestValidateConfigParams_AcceptsValid(t *testing.T) {
	min, max := 0.0, 2.0
	err := validateConfigParams("m1", []ConfigParam{
		{Key: "temperature", Type: ConfigParamDouble, Min: &min, Max: &max},
		{Key: "stop", Type: ConfigParamStringList},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
