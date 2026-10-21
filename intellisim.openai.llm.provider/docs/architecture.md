# Architecture

The OpenAI LLM Provider is a single-binary gRPC client microservice. It has no HTTP server. Its sole function is to bridge IntelliSim Core's `LLMBridge` service to the OpenAI API.

## Bootstrap Chain

```
main.go
  → Load config          (caarlos0/env → internal/config)
  → Create zap logger    (go.uber.org/zap)
  → Bridge zap to slog   (go.uber.org/zap/exp/zapslog)
  → NewOpenAIClient      (HTTP client for OpenAI API)
  → NewOpenAIHandler     (implements sdk.Handler + capability interfaces)
  → sdk.New(Options{CoreAddr, Logger, Handler, Registration})
  → provider.Run(ctx)    ← SDK takes over from here
```

## SDK Responsibilities

The SDK (`github.com/The-Maxwell/intellisim.llm.provider.go.sdk`) owns the entire connection lifecycle:

- gRPC dial to core
- `Connect(ConnectRequest)` — sends registration payload and opens the server-streaming session
- Receive loop: dispatches each inbound `Invocation` to the handler in its own goroutine
- `CompleteInvocation` — delivers results back to core
- Exponential-backoff reconnect when the stream breaks
- Drain-on-shutdown: waits up to `DrainTimeout` for in-flight handlers

The provider code never manages a gRPC stream or connection directly.

## Provider Responsibilities

| Component | File | Role |
|-----------|------|------|
| Bootstrap | `cmd/openai-provider/main.go` (~60 lines) | Config, logger, wiring, `sdk.New`, `Run` |
| Config | `internal/config/` | Env var parsing (`PROVIDER_ID`, `GRPC_HOST`, `GRPC_PORT`, `OPENAI_API_KEY`, etc.) |
| OpenAI HTTP client | `internal/provider/openai.go` | `Chat`, `Simple`, `Vision`, `Reasoning` calls + pure body builders |
| Handler | `internal/provider/handler.go` | Implements `sdk.Handler`, `sdk.ChatHandler`, `sdk.VisionHandler`, `sdk.ReasoningHandler`; guards unknown model UIDs via `checkModel` |
| Model registry | `internal/provider/models.go` | `[]sdk.Model` table with capabilities, efforts, and token limits; drives core's `GET /llms` listing |

## Handler Interfaces

`handler.go` implements all four SDK handler interfaces. Each method maps an SDK request type to an OpenAI API call and returns an SDK response type:

| Interface | Method | OpenAI endpoint |
|-----------|--------|-----------------|
| `sdk.Handler` | `Simple` | `/responses` API |
| `sdk.ChatHandler` | `Chat` | `/chat/completions` with history |
| `sdk.VisionHandler` | `Vision` | `/chat/completions` with base64 image |
| `sdk.ReasoningHandler` | `Reasoning` | `/chat/completions` (analytical prompt) |

## Capability and Model Gating

Capability gating (deciding whether a given capability is allowed for the current simulation strategy) happens in core before an invocation is dispatched. The provider only performs a local guard: `checkModel` in `handler.go` rejects invocations for model UIDs that are not present in `models.go`, returning `sdk.ErrCodeModelNotSupported`.

## Model Registration

`internal/provider/models.go` declares the `[]sdk.Model` slice passed to `sdk.New`. The SDK sends this list to core in the `ConnectRequest`. Core's `GET /llms` endpoint reflects these models after the provider connects. To add or remove a model, edit `models.go` and restart the provider.
