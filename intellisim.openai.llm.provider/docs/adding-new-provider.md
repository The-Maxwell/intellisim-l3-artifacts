# Building a New IntelliSim LLM Provider

The canonical guide for building a provider is the SDK README:

> **github.com/The-Maxwell/intellisim.llm.provider.go.sdk**

This document summarises the five steps. Refer to the SDK README for the full API reference, connection lifecycle details, error codes, and proto regeneration instructions.

---

## Overview

A provider is a single-binary gRPC client that connects to IntelliSim Core's `LLMBridge` service via the SDK, receives `Invocation` messages, calls the target LLM API, and returns results via `CompleteInvocation`. The SDK owns all connection, registration, dispatch, and reconnect logic.

---

## Five Steps

### 1. Get the SDK

```sh
go get github.com/The-Maxwell/intellisim.llm.provider.go.sdk
```

Until the module is published to GitHub with a release tag, reference it via a local `replace` directive:

```
require github.com/The-Maxwell/intellisim.llm.provider.go.sdk v0.0.0

replace github.com/The-Maxwell/intellisim.llm.provider.go.sdk => ../intellisim.llm.provider.go.sdk
```

### 2. Implement `sdk.Handler` + optional capability interfaces

Every provider must implement `sdk.Handler` (the `Simple` method). Implement additional interfaces only for capabilities your models declare:

| Capability | Required interface |
|------------|--------------------|
| `chat`, `model_generation` | `sdk.ChatHandler` — `Chat` method |
| `vision_analysis` | `sdk.VisionHandler` — `Vision` method |
| `results_analysis` | `sdk.ReasoningHandler` — `Reasoning` method |
| `simulation_execution`, `simulation_decisions` | `sdk.Handler` only |

`sdk.New` validates at construction time that every declared capability has a handler. A missing method is a startup error, not a runtime panic.

### 3. Declare `sdk.Registration` with models, capabilities, and efforts

```go
sdk.Registration{
    UID:         "my-provider",
    Name:        "My LLM Provider",
    Description: "...",
    Version:     "1.0.0",
    Models: []sdk.Model{
        {
            UID:             "my-model-1",
            Name:            "My Model",
            Capabilities:    []sdk.Capability{sdk.CapabilityChat, sdk.CapabilitySimulationDecisions},
            Efforts:         []sdk.Effort{},   // omit if model has no effort control
            ContextWindow:   128000,
            MaxOutputTokens: 4096,
        },
    },
}
```

Capabilities and efforts use the closed constant sets exported by the SDK (`sdk.CapabilityChat`, `sdk.EffortLow`, etc.).

### 4. Call `sdk.New` and `Run`

```go
p, err := sdk.New(sdk.Options{
    CoreAddr:     coreAddr,   // "host:port" of core's gRPC server
    Logger:       logger,     // *slog.Logger
    Handler:      myHandler,
    Registration: reg,
})
if err != nil {
    // registration configuration is invalid — fix and restart
    log.Fatal(err)
}

ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer cancel()
if err := p.Run(ctx); err != nil {
    log.Fatal(err)
}
```

`Run` blocks until the context is canceled, managing reconnects internally.

### 5. Configure `CoreAddr`

Pass the core gRPC address as `sdk.Options.CoreAddr`. The standard approach is to read it from an environment variable:

```go
coreAddr := os.Getenv("CORE_ADDR")
if coreAddr == "" {
    coreAddr = "localhost:50051"
}
```

Core's gRPC server port is controlled by `GRPC_SERVER_PORT` (default `50051`).

---

## Reference Implementation

`intellisim.openai.llm.provider` is the production reference implementation. Examine its structure for a complete example:

- `cmd/openai-provider/main.go` — bootstrap (~60 lines)
- `internal/provider/handler.go` — four handler interfaces
- `internal/provider/openai.go` — LLM HTTP client
- `internal/provider/models.go` — model registration table
- `internal/config/` — environment variable parsing
