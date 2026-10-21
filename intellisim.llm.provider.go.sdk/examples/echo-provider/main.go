// Echo provider — a minimal IntelliSim LLM provider used as a template and as
// the keyless E2E test peer. It echoes every request back with fake usage.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	sdk "github.com/The-Maxwell/intellisim.llm.provider.go.sdk"
)

type echoHandler struct{}

func usage() sdk.Usage {
	return sdk.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
}

func (echoHandler) Simple(ctx context.Context, req *sdk.SimpleRequest) (*sdk.Completion, error) {
	return &sdk.Completion{Text: "echo: " + req.Message, FinishReason: "stop", Usage: usage()}, nil
}

func (echoHandler) Chat(ctx context.Context, req *sdk.ChatRequest) (*sdk.Completion, error) {
	return &sdk.Completion{
		Text:         fmt.Sprintf("echo: %s (history: %d)", req.Message, len(req.History)),
		FinishReason: "stop", Usage: usage(),
	}, nil
}

func (echoHandler) Vision(ctx context.Context, req *sdk.VisionRequest) (*sdk.Completion, error) {
	return &sdk.Completion{
		Text:         fmt.Sprintf("echo: %s (images: %d)", req.Message, len(req.Images)),
		FinishReason: "stop", Usage: usage(),
	}, nil
}

func (echoHandler) Reasoning(ctx context.Context, req *sdk.ReasoningRequest) (*sdk.ReasoningCompletion, error) {
	return &sdk.ReasoningCompletion{
		Analysis: "echo analysis: " + req.Prompt, Reasoning: "echo reasoning",
		FinishReason: "stop", Usage: usage(),
	}, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	coreAddr := os.Getenv("CORE_ADDR")
	if coreAddr == "" {
		coreAddr = "localhost:50051"
	}

	provider, err := sdk.New(sdk.Options{
		CoreAddr: coreAddr,
		Logger:   logger,
		Handler:  echoHandler{},
		Registration: sdk.Registration{
			UID: "echo", Name: "Echo Provider",
			Description: "Echoes requests back — for testing", Version: "1.0.0",
			Models: []sdk.Model{{
				UID: "echo-1", Name: "Echo One",
				Capabilities: []sdk.Capability{
					sdk.CapabilityChat, sdk.CapabilityModelGeneration,
					sdk.CapabilitySimulationGeneration,
					sdk.CapabilitySimulationExecution, sdk.CapabilitySimulationDecisions,
					sdk.CapabilityResultsAnalysis, sdk.CapabilityVisionAnalysis,
				},
				Efforts:       []sdk.Effort{sdk.EffortMinimal, sdk.EffortLow, sdk.EffortMedium, sdk.EffortHigh, sdk.EffortMax},
				ContextWindow: 128000, MaxOutputTokens: 4096,
			}},
		},
	})
	if err != nil {
		logger.Error("invalid provider configuration", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := provider.Run(ctx); err != nil {
		logger.Error("provider stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("provider stopped")
}
