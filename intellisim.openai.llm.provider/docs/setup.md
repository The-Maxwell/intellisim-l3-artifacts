# Setup Guide

Get the OpenAI LLM Provider running locally or in production.

## Prerequisites

- **Go 1.25** or later
  ```bash
  go version
  ```
  
- **gRPC Runtime** — Included in Go toolchain, no separate installation needed

- **IntelliSim Core** running and reachable at `GRPC_HOST:GRPC_PORT`
  - Default: `localhost:9000`
  - Ensure core is accessible before starting provider

- **OpenAI API Key**
  - Get from: https://platform.openai.com/api-keys
  - Free tier or paid account required
  - Store securely (environment variable, secret management system)

## Environment Variables

All configuration via environment variables. Create `.env` file or export directly.

### Required Variables

| Variable | Example | Description |
|----------|---------|-------------|
| `OPENAI_API_KEY` | `sk-proj-...` | **REQUIRED** — OpenAI API key from platform.openai.com |

### Provider Identity

| Variable | Default | Description |
|----------|---------|-------------|
| `PROVIDER_ID` | `openai_gpt4o` | Unique identifier for this provider (registration with core) |
| `PROVIDER_NAME` | `OpenAI GPT-4o` | Human-readable provider name (shown in UI, logs) |

### Core Connection

| Variable | Default | Description |
|----------|---------|-------------|
| `GRPC_HOST` | `localhost` | IntelliSim Core gRPC server hostname/IP |
| `GRPC_PORT` | `9000` | IntelliSim Core gRPC server port (must match core's `GRPC_PORT`) |

### OpenAI API Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENAI_BASE_URL` | `https://api.openai.com/v1` | OpenAI API base URL (change for proxies/custom endpoints) |
| `OPENAI_DEFAULT_MODEL` | `gpt-4o` | Default model for requests without explicit model selection. Options: `gpt-4o`, `gpt-4o-mini`, `gpt-3.5-turbo` |
| `OPENAI_TIMEOUT_SECONDS` | `30` | HTTP request timeout in seconds. Increase for large vision requests. |
| `OPENAI_MAX_RETRIES` | `3` | Maximum retry attempts on OpenAI API failures (exponential backoff). |

### Example `.env` File

```env
# Provider identity (shown in core's /llms endpoint)
PROVIDER_ID=openai_gpt4o
PROVIDER_NAME=OpenAI GPT-4o

# Core connection (must match core's GRPC_PORT)
GRPC_HOST=localhost
GRPC_PORT=9000

# OpenAI API
OPENAI_API_KEY=sk-proj-your-actual-key-here
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_DEFAULT_MODEL=gpt-4o
OPENAI_TIMEOUT_SECONDS=30
OPENAI_MAX_RETRIES=3
```

## Build

### From Source

```bash
# Clone or navigate to provider directory
cd intellisim.openai.llm.provider

# Build executable
go build -o bin/openai-provider.exe ./cmd/openai-provider

# On Linux/macOS: output is bin/openai-provider (no .exe)
# On Windows: output is bin/openai-provider.exe
```

### Verification

```bash
# Check build succeeded
file bin/openai-provider.exe    # Windows
file bin/openai-provider        # Linux/macOS

# Show version info
bin/openai-provider.exe version 2>/dev/null || echo "Provider doesn't have version endpoint"
```

## Run

### Development (with .env file)

```bash
# Ensure .env exists in current directory
ls -la .env

# Run
go run ./cmd/openai-provider

# Expected output:
# {"level":"info","ts":1699564789.123,"caller":"main.go:25","msg":"Starting OpenAI LLM Provider"}
# {"level":"info","ts":1699564789.456,"caller":"config.go:29","msg":"Config loaded","provider_id":"openai_gpt4o","openai_base_url":"https://api.openai.com/v1","openai_model":"gpt-4o"}
# {"level":"info","ts":1699564789.789,"caller":"main.go:62","msg":"Registered with core","message":"Registration successful"}
# {"level":"info","ts":1699564789.999,"caller":"main.go:79","msg":"Stream connected to core"}
```

### Production (with explicit env vars)

```bash
# Export variables
export OPENAI_API_KEY="sk-proj-your-key"
export PROVIDER_ID="openai_gpt4o"
export PROVIDER_NAME="OpenAI GPT-4o"
export GRPC_HOST="core.example.com"
export GRPC_PORT="9000"
export OPENAI_DEFAULT_MODEL="gpt-4o"
export OPENAI_TIMEOUT_SECONDS="30"
export OPENAI_MAX_RETRIES="3"

# Run executable
./bin/openai-provider.exe
```

### Docker (optional)

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o openai-provider ./cmd/openai-provider

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/openai-provider /usr/local/bin/

ENTRYPOINT ["openai-provider"]
```

Build:
```bash
docker build -t intellisim-openai-provider:latest .
```

Run:
```bash
docker run \
  -e OPENAI_API_KEY="sk-proj-..." \
  -e PROVIDER_ID="openai_gpt4o" \
  -e GRPC_HOST="host.docker.internal" \
  -e GRPC_PORT="9000" \
  intellisim-openai-provider:latest
```

## Verification

### 1. Check Provider Registered with Core

Query core's LLM list:

```bash
# HTTP request to core
curl -X GET http://localhost:8080/api/v1/intellisim/llms

# Expected response:
{
  "providers": [
    {
      "provider_id": "openai_gpt4o",
      "name": "OpenAI GPT-4o",
      "models": [
        {
          "id": "gpt-4o",
          "name": "GPT-4o",
          "capabilities": ["chat", "simple", "vision", "reasoning"],
          "max_output_tokens": 16384,
          "context_window": 128000
        },
        ...
      ]
    }
  ]
}
```

### 2. Check Provider Logs

Look for these key log lines:

```
"msg":"Starting OpenAI LLM Provider"               # Process started
"msg":"Config loaded"                              # Config parsed
"msg":"Registered with core"                       # Registration successful
"msg":"Stream connected to core"                   # Ready for requests
```

### 3. Send Test Request from Core

Via core's chat endpoint:

```bash
curl -X POST http://localhost:8080/api/v1/intellisim/chats/{chatId}/messages \
  -H "Content-Type: application/json" \
  -d '{
    "message": "What is 2+2?",
    "model": "gpt-4o",
    "provider": "openai_gpt4o"
  }'

# Expected: Response from OpenAI GPT-4o model
```

Monitor provider logs for:
```
"msg":"Received request from core","request_id":"req_..."
"msg":"Chat response","latency_ms":1234.5
```

### 4. Heartbeat Monitoring

Provider sends heartbeats every 15 seconds. Check logs:

```
"msg":"Heartbeat acknowledged"   # Every 15s if core is responsive
```

Missing heartbeats (after 60+ seconds) may indicate network issues.

### 5. Graceful Shutdown

Stop the provider:

```bash
# In terminal running provider, press Ctrl+C
# Or send SIGTERM
kill -TERM <PID>

# Expected logs:
"msg":"Shutting down..."
"msg":"Provider stopped"
```

Check core logs — provider should be removed from registry.

## Troubleshooting

### "OPENAI_API_KEY is required"

**Cause**: API key not provided or not in environment

**Fix**:
```bash
# Check env var is set
echo $OPENAI_API_KEY

# Set if missing
export OPENAI_API_KEY="sk-proj-your-key"

# Run again
go run ./cmd/openai-provider
```

### "Failed to open Connect stream"

**Cause**: Cannot reach Core's gRPC server

**Fix**:
```bash
# Verify core is running
lsof -i :9000            # macOS/Linux
netstat -ano | grep 9000 # Windows

# Check GRPC_HOST and GRPC_PORT
echo "GRPC_HOST=$GRPC_HOST GRPC_PORT=$GRPC_PORT"

# If core on different host:
export GRPC_HOST="core.example.com"
export GRPC_PORT="9000"

# Test connectivity
nc -zv localhost 9000    # Connection test
```

### "Registration rejected"

**Cause**: Provider ID already registered, or invalid model

**Fix**:
```bash
# Use unique provider ID
export PROVIDER_ID="openai_gpt4o_backup"

# Or restart core to clear stale registrations
# Or check core logs for rejection reason
```

### "Heartbeat not acknowledged"

**Cause**: Network latency, core overload, or connection lost

**Monitoring**:
- Single heartbeat miss: Not critical, retry in 15s
- Multiple consecutive misses: Indicates network problem
- All requests failing: Core likely unreachable

**Fix**:
```bash
# Check network
ping core.example.com

# Check core is responsive
curl http://core.example.com:8080/health

# Restart provider if state is corrupted
```

### "API error 401"

**Cause**: Invalid or expired OpenAI API key

**Fix**:
```bash
# Verify key is valid at https://platform.openai.com/api-keys
# Create new key if expired

export OPENAI_API_KEY="sk-proj-new-key"
go run ./cmd/openai-provider
```

### "API error 429" (Rate Limited)

**Cause**: OpenAI rate limit exceeded

**Provider Behavior**:
- Automatically retries with backoff (100ms, 300ms, 500ms)
- After 3 retries, returns error to core
- Core may queue request for retry later

**To Reduce**:
- Increase `OPENAI_MAX_RETRIES` to 5
- Reduce concurrent requests from core
- Use `gpt-4o-mini` or `gpt-3.5-turbo` for lower cost tier

### Provider Exits Immediately

**Cause**: Missing required config

**Fix**:
```bash
# Check all required env vars
env | grep OPENAI
env | grep GRPC

# Ensure:
# - OPENAI_API_KEY is set (required)
# - GRPC_HOST and GRPC_PORT are set
# - GRPC_HOST is reachable
```

## Performance Tuning

### Timeout Adjustment

For slow networks or large vision requests:

```env
OPENAI_TIMEOUT_SECONDS=60   # Increase from default 30
```

### Retry Strategy

For unreliable networks:

```env
OPENAI_MAX_RETRIES=5        # Increase from default 3
```

**Trade-off**: Higher retries = slower failure detection, but better resilience.

### Model Selection

For cost/speed trade-off:

```env
# Fast and cheap (but lower quality)
OPENAI_DEFAULT_MODEL=gpt-3.5-turbo

# Balanced (recommended)
OPENAI_DEFAULT_MODEL=gpt-4o-mini

# Best quality (highest cost)
OPENAI_DEFAULT_MODEL=gpt-4o
```

### Concurrent Request Limits

No explicit concurrency limit in provider. Limits come from:
- OpenAI API rate limits (RPM, TPM)
- Core's request distribution
- Network bandwidth

**Monitoring**: Check core's simulation engine configuration for request batching.

## Integration Checklist

Before going to production:

- [ ] OpenAI API key obtained and validated
- [ ] Core is running and accessible at configured GRPC_HOST:GRPC_PORT
- [ ] Provider builds without errors: `go build ./cmd/openai-provider`
- [ ] Provider starts and registers: `go run ./cmd/openai-provider`
- [ ] Core's `/llms` endpoint lists openai_gpt4o provider
- [ ] Test request succeeds: Chat, Simple, or Vision request returns response
- [ ] Heartbeats logged regularly (every 15s)
- [ ] Graceful shutdown works: SIGTERM cleanly unregisters
- [ ] Error handling verified: Invalid requests return 501/500 errors
- [ ] Logs are readable and don't expose API keys

## Summary

Setup is straightforward:

1. Set `OPENAI_API_KEY` (required)
2. Ensure Core is reachable at `GRPC_HOST:GRPC_PORT`
3. Build: `go build ./cmd/openai-provider`
4. Run: `./openai-provider`
5. Verify: Check Core's `/llms` endpoint and test a request

See logs for registration confirmation and any errors.
