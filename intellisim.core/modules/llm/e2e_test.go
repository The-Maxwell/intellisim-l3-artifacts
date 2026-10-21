package llm

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	sdk "github.com/The-Maxwell/intellisim.llm.provider.go.sdk"
	llmbridgev1 "github.com/The-Maxwell/intellisim.llm.provider.go.sdk/proto/llmbridge/v1"
	"intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
)

// echoSDKHandler implements all four sdk handler interfaces deterministically.
type echoSDKHandler struct{}

func sdkUsage() sdk.Usage {
	return sdk.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
}

func (echoSDKHandler) Simple(ctx context.Context, req *sdk.SimpleRequest) (*sdk.Completion, error) {
	return &sdk.Completion{Text: "echo: " + req.Message, FinishReason: "stop", Usage: sdkUsage()}, nil
}
func (echoSDKHandler) Chat(ctx context.Context, req *sdk.ChatRequest) (*sdk.Completion, error) {
	return &sdk.Completion{
		Text:         fmt.Sprintf("echo: %s (history: %d)", req.Message, len(req.History)),
		FinishReason: "stop", Usage: sdkUsage(),
	}, nil
}
func (echoSDKHandler) Vision(ctx context.Context, req *sdk.VisionRequest) (*sdk.Completion, error) {
	return &sdk.Completion{
		Text:         fmt.Sprintf("echo: %s (images: %d)", req.Message, len(req.Images)),
		FinishReason: "stop", Usage: sdkUsage(),
	}, nil
}
func (echoSDKHandler) Reasoning(ctx context.Context, req *sdk.ReasoningRequest) (*sdk.ReasoningCompletion, error) {
	return &sdk.ReasoningCompletion{
		Analysis: "echo analysis: " + req.Prompt, Reasoning: "echo reasoning",
		FinishReason: "stop", Usage: sdkUsage(),
	}, nil
}

// startCoreGRPC starts a production-configured gRPC server on an ephemeral port.
func startCoreGRPC(t *testing.T, bridge *ProviderBridge) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime: 10 * time.Second, PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time: 30 * time.Second, Timeout: 10 * time.Second,
		}),
		grpc.MaxRecvMsgSize(32*1024*1024),
		grpc.MaxSendMsgSize(32*1024*1024),
	)
	llmbridgev1.RegisterLLMBridgeServer(srv, bridge)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func echoProviderOptions(addr string) sdk.Options {
	return sdk.Options{
		CoreAddr:       addr,
		Handler:        echoSDKHandler{},
		InitialBackoff: 50 * time.Millisecond,
		MaxBackoff:     200 * time.Millisecond,
		Registration: sdk.Registration{
			UID: "echo", Name: "Echo", Version: "1.0.0",
			Models: []sdk.Model{{
				UID: "echo-1", Name: "Echo One",
				Capabilities: []sdk.Capability{
					sdk.CapabilityChat, sdk.CapabilityModelGeneration,
					sdk.CapabilitySimulationExecution, sdk.CapabilitySimulationDecisions,
					sdk.CapabilityResultsAnalysis, sdk.CapabilityVisionAnalysis,
				},
				Efforts:       []sdk.Effort{sdk.EffortLow, sdk.EffortMedium, sdk.EffortHigh},
				ContextWindow: 128000, MaxOutputTokens: 4096,
			}},
		},
	}
}

func TestE2E_FullFlowOverRealTCP(t *testing.T) {
	logger := zap.NewNop()
	registry := NewRegistry(logger)
	bridge := NewProviderBridge(logger, registry)
	addr := startCoreGRPC(t, bridge)

	provider, err := sdk.New(echoProviderOptions(addr))
	require.NoError(t, err)
	runCtx, stopProvider := context.WithCancel(context.Background())
	defer stopProvider()
	go provider.Run(runCtx)

	require.Eventually(t, func() bool { _, ok := registry.Get("echo"); return ok },
		5*time.Second, 20*time.Millisecond, "provider must register")

	svc := NewService(logger, registry, NewContextFitter(logger, NewTokenEstimator(logger)))
	ctx := context.Background()
	echoTarget := common.LLMTarget{ProviderUID: "echo", ModelUID: "echo-1", Effort: common.LLMEffortLow}

	// 1. simple
	simple, err := svc.Simple(ctx, SimpleRequest{Target: echoTarget, Message: "hello"})
	require.NoError(t, err)
	assert.Equal(t, "echo: hello", simple.Text)
	assert.Equal(t, "echo", simple.Usage.ProviderUID)
	assert.Equal(t, "echo-1", simple.Usage.ModelUID)
	assert.Equal(t, common.LLMEffortLow, simple.Usage.Effort) // SDK auto-fill
	assert.Equal(t, uint64(1), simple.Usage.Calls)
	assert.Equal(t, uint64(15), simple.Usage.TotalTokens)

	// 2. chat with history
	chat, err := svc.Chat(ctx, ChatRequest{
		Target: echoTarget, Message: "hi",
		Messages: []chats.ChatMessage{
			{Role: chats.ChatMessageUserRole, TextContent: "m1"},
			{Role: chats.ChatMessageAssistantRole, TextContent: "m2"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "echo: hi (history: 2)", chat.Text)

	// 3. vision
	vision, err := svc.Vision(ctx, VisionRequest{
		Target: echoTarget, Message: "shapes",
		Images: []VisionImage{{Data: []byte{1, 2, 3}, Format: "png"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "echo: shapes (images: 1)", vision.Text)

	// 4. reasoning
	reasoning, err := svc.Reasoning(ctx, ReasoningRequest{Target: echoTarget, Prompt: "why"})
	require.NoError(t, err)
	assert.Equal(t, "echo analysis: why", reasoning.Analysis)

	// 5. validation failure (effort not declared)
	_, err = svc.Simple(ctx, SimpleRequest{
		Target: common.LLMTarget{ProviderUID: "echo", ModelUID: "echo-1", Effort: common.LLMEffortMax},
	})
	assert.ErrorIs(t, err, ErrEffortNotAllowed)

	// 6. provider restart -> reconnect -> serve again
	stopProvider()
	require.Eventually(t, func() bool { _, ok := registry.Get("echo"); return !ok },
		5*time.Second, 20*time.Millisecond, "provider must unregister after stop")

	provider2, err := sdk.New(echoProviderOptions(addr))
	require.NoError(t, err)
	runCtx2, stop2 := context.WithCancel(context.Background())
	defer stop2()
	go provider2.Run(runCtx2)

	require.Eventually(t, func() bool { _, ok := registry.Get("echo"); return ok },
		5*time.Second, 20*time.Millisecond, "provider must re-register")
	again, err := svc.Simple(ctx, SimpleRequest{Target: echoTarget, Message: "back"})
	require.NoError(t, err)
	assert.Equal(t, "echo: back", again.Text)
}
