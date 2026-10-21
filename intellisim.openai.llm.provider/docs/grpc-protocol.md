# gRPC Protocol Reference

## Wire Contract

The provider communicates with IntelliSim Core via the `llmbridge.v1.LLMBridge` service, defined in the SDK repository:

```
intellisim.llm.provider.go.sdk/proto/llmbridge/v1/llmbridge.proto
```

The service exposes exactly two RPCs:

| RPC | Kind | Description |
|-----|------|-------------|
| `Connect(ConnectRequest)` | Server-streaming | Registration + invocation delivery. The provider sends a single `ConnectRequest` (carrying identity and model list) and then receives a stream of `Invocation` messages from core. |
| `CompleteInvocation(CompleteInvocationRequest)` | Unary | Delivers a handler result (completion or error) back to core for a specific invocation. Called once per `Invocation` received. |

There are no separate Register, liveness-ping, or teardown RPCs — those belonged to the old contract and have been removed.

## Liveness

Connection liveness is maintained entirely by gRPC keepalive. The SDK configures the following default parameters on the client connection:

- Keepalive ping interval: 15 s
- Keepalive ping timeout: 5 s
- Permit pings without active streams: yes

No application-level heartbeat messages are sent.

## Further Reading

- **Connection lifecycle** (reconnect loop, backoff, drain-on-shutdown): SDK README — "Connection lifecycle" section.
- **Invocation dispatch, capability validation, error codes**: SDK README — "Handler interfaces" and "Error handling" sections.
- **Core-side bridge implementation**: `intellisim.core/docs/llm-integration.md` — "gRPC Bridge Protocol" section.
