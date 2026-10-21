# intellisim.openai.llm.provider

OpenAI adapter microservice for IntelliSim. A pure gRPC client of the core backend, built on
[`intellisim.llm.provider.go.sdk`](../intellisim.llm.provider.go.sdk): it registers its model
catalog with the core, receives LLM invocations (chat, simple, vision, reasoning) over the
`llmbridge.v1` stream, translates them into OpenAI API calls, and returns completions with token
usage. The SDK owns all connection, registration, reconnect and dispatch logic.

## Run

```sh
export OPENAI_API_KEY=sk-...   # PowerShell: $env:OPENAI_API_KEY="sk-..."
go run ./cmd/openai-provider
```

Configuration is read from the process environment; `.env.example` lists every variable and its
default. `GRPC_HOST`/`GRPC_PORT` must point at the core's gRPC server (default `localhost:50051`).
Start the core first — the provider reconnects automatically with exponential backoff.

## Documentation

- `docs/setup.md` — full setup walkthrough
- `docs/architecture.md` — service structure
- `docs/openai-api.md` — OpenAI API usage details
- `docs/grpc-protocol.md` — the llmbridge protocol
- `docs/adding-new-provider.md` — how to build another provider on the SDK

Supported models and their capabilities are declared in `internal/provider/models.go`.

## License

Copyright (c) 2026 Vladyslav Pryshchepa.

Licensed under the [PolyForm Noncommercial License 1.0.0](LICENSE) — free to use, modify and
share for **noncommercial purposes** such as scientific research, academic study and teaching.

**Commercial use requires a separate license.** Contact vlad.prishhepa@gmail.com.
