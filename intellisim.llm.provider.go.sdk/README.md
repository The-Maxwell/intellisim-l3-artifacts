# intellisim.llm.provider.go.sdk

Go SDK for building IntelliSim LLM providers. The SDK encapsulates the entire gRPC bridge lifecycle — dialing core, sending a `ConnectRequest` to register the provider and its models, reconnecting with exponential backoff when the stream breaks, dispatching each inbound `Invocation` to the correct handler method, auto-filling usage fields, and delivering the result back via `CompleteInvocation` — so that providers only need to implement one or more handler interfaces and call `Run`.

---

## Install

```sh
go get github.com/The-Maxwell/intellisim.llm.provider.go.sdk
```

**Note:** in this repository the SDK is consumed as a sibling module — `intellisim.core` and
`intellisim.openai.llm.provider` reference it via a local `replace` directive, so the folder
layout must be preserved:

```
require github.com/The-Maxwell/intellisim.llm.provider.go.sdk v0.0.0

replace github.com/The-Maxwell/intellisim.llm.provider.go.sdk => ../intellisim.llm.provider.go.sdk
```

Standalone consumers can instead pin a published release with `go get`.

---

## Quickstart

The `examples/echo-provider` directory contains a minimal, runnable provider that echoes every request back with fixed fake usage. Use it as a template.

```go
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
		Text:         fmt.Sprintf("echo: %s (image: %d bytes %s)", req.Message, len(req.ImageData), req.ImageFormat),
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
```

**Options** — pass `CoreAddr` (the `host:port` of core's gRPC server), a populated `Registration`, your `Handler` implementation, and an optional `*slog.Logger`. Timing fields (`InitialBackoff`, `MaxBackoff`, `DefaultTimeout`, `DrainTimeout`) have sensible defaults and can be omitted.

**Registration** — declares the provider's identity (`UID`, `Name`, `Version`) and the slice of `Model` values it serves. Each model lists the capabilities it supports (from the closed `Capability` set) and optionally the `Effort` levels it accepts.

**Handler** — `echoHandler` implements all four handler interfaces. For a real provider, implement only the interfaces required by the capabilities your models declare (see the table below).

**Run** — blocks until the context is canceled, managing the connection loop internally. Pass a context wired to OS signals (`signal.NotifyContext`) for clean shutdown.

---

## Handler interfaces

`sdk.New()` validates, at construction time, that every capability declared by any registered model has a corresponding handler method. If the check fails, `New` returns an error immediately and the provider does not start.

| Declared capability (any registered model) | Required interface |
|---|---|
| *(all registrations)* | `Handler` — `Simple` method (compile-time) |
| `chat`, `model_generation` | `ChatHandler` — `Chat` method |
| `vision_analysis` | `VisionHandler` — `Vision` method |
| `results_analysis` | `ReasoningHandler` — `Reasoning` method |
| `simulation_generation`, `simulation_execution`, `simulation_decisions` | `Handler` only (`Simple`) |

Error format on mismatch:

```
sdk: model "gpt-4o" declares capability "vision_analysis" but handler does not implement sdk.VisionHandler
```

At runtime, if an invocation type arrives for which no handler is present (core version skew), the SDK replies with `capability_not_supported` and never panics — the process stays healthy.

---

## Request / response types

All request types carry `ModelUID string` and `Effort Effort` (populated from the invocation by core) plus a `Config GenerationConfig` field.

### Request types

| Type | Key fields |
|---|---|
| `SimpleRequest` | `Message string`, `SystemPrompt string`, `Config GenerationConfig` |
| `ChatRequest` | `Message string` (new turn), `SystemMessage string`, `History []ChatMessage`, `Config GenerationConfig` |
| `VisionRequest` | `Message string`, `ImageData []byte`, `ImageFormat string` (`"png"/"jpeg"/"webp"/"gif"`), `Detail string` (`"auto"/"low"/"high"`), `SystemPrompt string`, `Config GenerationConfig` |
| `ReasoningRequest` | `Prompt string`, `Context string` (simulation summary/metrics), `SystemPrompt string`, `Config GenerationConfig` |

### GenerationConfig

`GenerationConfig` carries optional per-request tuning as a dynamic map keyed by the
parameter names your model advertised via `ConfigParams` (see [Declaring tunable
configuration](#declaring-tunable-configuration)). Read submitted values from
`req.Config.Params`. An absent key means "use the provider or model default."

```go
type GenerationConfig struct {
    Params map[string]ConfigValue
}

// ConfigValue is a tagged union — exactly one field is non-nil.
type ConfigValue struct {
    Double *float64
    Int    *int64
    Uint   *uint64
    Bool   *bool
    String *string  // also carries enum selections
    List   []string
}
```

`ResponseFormat` is **not** part of `GenerationConfig`; it is a separate,
core-controlled field on every request struct (`req.ResponseFormat`, one of
`"" | "text" | "json"`).

### Response types

Handlers return either `*Completion` (Simple / Chat / Vision) or `*ReasoningCompletion` (Reasoning).

### Usage auto-fill

The SDK automatically back-fills three fields of the `Usage` value embedded in the response before sending it to core:

| Field | Auto-fill rule |
|---|---|
| `ModelUID` | Set from the invocation's `model_uid` when the handler left it empty (`""`) |
| `Effort` | Set from the invocation's `effort` when the handler left it as the zero value |
| `LatencyMs` | Set to the measured wall-clock duration of the handler call when the handler left it `0` |

Populate these fields yourself if you need to report a different model or effort than the one requested (e.g. a fallback model was used).

---

## Error handling

Return a plain `error` from a handler for unexpected failures. Return `sdk.Errorf(code, ...)` to send a specific wire-level error code to core.

```go
// typed error — exact code reaches core
return nil, sdk.Errorf(sdk.ErrCodeModelNotSupported, "model %q is not loaded", req.ModelUID)

// plain error — becomes upstream_error on the wire
return nil, fmt.Errorf("OpenAI API returned 503")
```

| `ErrorCode` constant | Wire value | Meaning to core |
|---|---|---|
| `ErrCodeInvalidRequest` | `invalid_request` | The invocation payload is malformed |
| `ErrCodeModelNotSupported` | `model_not_supported` | The requested model is unknown or unavailable |
| `ErrCodeEffortNotSupported` | `effort_not_supported` | The requested effort level is not supported by the model |
| `ErrCodeCapabilityNotSupported` | `capability_not_supported` | The invocation type is not handled (SDK also emits this on version skew) |
| `ErrCodeUpstreamError` | `upstream_error` | The underlying LLM API call failed; any plain `error` is mapped here |
| `ErrCodeTimeout` | `timeout` | The handler's context deadline was exceeded; `context.DeadlineExceeded` is mapped here automatically |
| `ErrCodeInternal` | `internal` | Handler panic; recovered by the SDK — the process continues |

**Panic recovery:** the SDK wraps every handler call in a `recover()`. A panicking handler produces an `internal` error response; no goroutine escapes, no other invocations are affected.

**Context deadline:** if `ctx.Err() != nil` when the handler returns (or the handler returns `context.DeadlineExceeded`), the error is mapped to `ErrCodeTimeout` regardless of what the handler itself returned.

---

## Connection lifecycle

`Run` opens a single `grpc.ClientConn` for its entire lifetime and drives the following loop:

```
Run(ctx)
└─ loop until ctx canceled:
   ├─ open Connect(registration) → server-streaming session
   │    ├─ gRPC InvalidArgument  →  Run returns the error immediately (no retry —
   │    │                           a registration bug will not fix itself)
   │    └─ any other error  →  exponential backoff, then retry
   ├─ connected  →  log, reset backoff to InitialBackoff (1s)
   ├─ receive loop: for each Invocation → go dispatch(...)
   │    ├─ context with timeout = invocation.timeout_ms  (or DefaultTimeout if 0)
   │    ├─ route oneof to handler; recover panics into ERROR_CODE_INTERNAL
   │    ├─ auto-fill Usage defaults
   │    └─ CompleteInvocation(result)
   │         ├─ transient transport error  →  retry up to 2 times (500 ms delay)
   │         ├─ gRPC Canceled              →  drop silently (connection closing)
   │         └─ ack.accepted == false      →  drop silently (core moved on)
   ├─ stream breaks  →  reconnect WITHOUT waiting for in-flight handlers; a healthy
   │                    session resets the delay to InitialBackoff (1 s ±20%)
   └─ ctx canceled   →  stop reading, drain ≤ DrainTimeout, return nil
```

**Backoff:** starts at `InitialBackoff` (default 1 s), doubles on each failed connect attempt, capped at `MaxBackoff` (default 30 s). Each delay is jittered ±20% to spread reconnect storms.

**Registration rejection:** a `gRPC InvalidArgument` status from `Connect` means core rejected the registration (duplicate UID, unknown capability, etc.). `Run` returns the error wrapped as `"sdk: registration rejected by core: ..."` and does not retry. Fix the `Options` and restart.

**Drain on shutdown:** when the context is canceled, `Run` stops reading new invocations and waits up to `DrainTimeout` (default 10 s) for all in-flight handler goroutines to finish. Handlers that have not finished by the deadline are abandoned but keep running detached; their `CompleteInvocation` calls are best-effort and may fail against the then-closed connection. Do not assume that `Run` returning means every handler goroutine has stopped.

**Default dial options** (used unless `DialOptions` is set):

| Parameter | Value |
|---|---|
| Transport | Insecure (plaintext) |
| Keepalive ping interval | 15 s |
| Keepalive ping timeout | 5 s |
| Permit keepalive without active streams | yes |
| Max send / recv message size | 32 MB (vision payloads exceed the 4 MB gRPC default) |

To use TLS or a custom dialer, set `Options.DialOptions`; this fully replaces the SDK defaults.

---

## Proto regeneration

The generated files (`proto/llmbridge/v1/llmbridge.pb.go` and `llmbridge_grpc.pb.go`) are committed. Regenerate only when `llmbridge.proto` changes.

**Prerequisites (install once):**

```sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1
```

**Windows** (PowerShell) — `protoc` is resolved automatically; if it is not on `PATH` and `$env:PROTOC` is not set, the script downloads protoc 29.3 into `tools/` (gitignored):

```powershell
powershell -ExecutionPolicy Bypass -File scripts\gen-proto.ps1
```

**Linux / macOS** (bash) — requires `protoc` on `PATH` or `$PROTOC` set:

```bash
bash scripts/gen-proto.sh
```

Both scripts place the generated files back under `proto/llmbridge/v1/` using `paths=source_relative`.

---

## Publish flow

1. Push `main` to `github.com/The-Maxwell/intellisim.llm.provider.go.sdk`.
2. Tag the commit: `git tag v0.1.0 && git push origin v0.1.0`.
3. In each consumer (`intellisim.core`, `intellisim.openai.llm.provider`, etc.): remove the local `replace` directive and run `go get github.com/The-Maxwell/intellisim.llm.provider.go.sdk@v0.1.0`.

---

## Declaring tunable configuration

Each `Model` you register may advertise the generation parameters it accepts via
`ConfigParams`. Core stores these descriptors, exposes them on `GET /llms`, and
**validates** any submitted values against them before invoking you.

- All generation params are **optional**. An absent param means "use the
  provider/model default".
- A descriptor has: `Key`, `Type` (`double`/`int`/`uint`/`bool`/`string`/`enum`/`string_list`),
  `Required`, optional `Min`/`Max` (numeric), `EnumValues` (for `enum`), and an
  optional `Default`.
- Values arrive on each request as `req.Config.Params` (a `map[string]ConfigValue`).
- `response_format` is **core-controlled** — it arrives as `req.ResponseFormat`, is
  NOT a tunable param, and must not be advertised as a `ConfigParam`.
- `Reasoning bool` is a UI hint (e.g. reasoning models that reject sampling params).

Example:

```go
min, max := 0.0, 2.0
sdk.Model{
    UID: "gpt-4o", Name: "GPT-4o", Reasoning: false,
    Capabilities: []sdk.Capability{sdk.CapabilityChat},
    ConfigParams: []sdk.ConfigParam{
        {Key: "temperature", Type: sdk.ConfigParamDouble, Min: &min, Max: &max, Label: "Temperature"},
        {Key: "stop", Type: sdk.ConfigParamStringList, Label: "Stop sequences"},
    },
}
```

---

## License

Copyright (c) 2026 Vladyslav Pryshchepa.

Licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE) — free to use, modify and
share for **noncommercial purposes** such as scientific research, academic study and teaching.

**Commercial use requires a separate license.** Contact vlad.prishhepa@gmail.com.
