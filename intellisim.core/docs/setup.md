# Setup

## Prerequisites

- Go 1.25+
- MongoDB 4.4+ (running locally or remote)
- OpenAI API key

## Configuration

Create a `.env` file in the project root:

```env
# HTTP Server
PORT=8080
GRPC_PORT=50051

# MongoDB
MONGO_HOST=localhost:27017
MONGO_DATABASE=intellisim
MONGO_USERNAME=                    # Optional: MongoDB username
MONGO_PASSWORD=                    # Optional: MongoDB password
MONGO_AUTH_SOURCE=                 # Optional: Auth source database
MONGO_REPLICA_SET=                 # Optional: Replica set name

# API Configuration
OPENAI_API_KEY=sk-your-key-here
WS_ALLOWED_ORIGIN=*

# Logging
LOG_LEVEL=info
```

See `.env-list` for the PowerShell format of required variables.

## Build & Run

```bash
# Install dependencies
go mod download

# Build
go build -o bin/intelli-sim.exe ./cmd/intelli-sim

# Run
./bin/intelli-sim.exe
```

Or use the startup scripts:
- Windows: `start-dev.bat`
- Linux/macOS: `./start-dev.sh`

The server starts at `http://localhost:8080`.

## Development

```bash
# Run tests
go test ./...

# Build only
go build ./...
```

## Running with LLM Provider

The gRPC server allows external LLM providers to integrate with the core system. To run with an external provider:

1. **Start the core backend** (this also starts gRPC server on `GRPC_PORT`):
```bash
./bin/intelli-sim.exe
```

2. **Start your LLM provider** (e.g., OpenAI provider bridge):
```bash
cd ../path-to-llm-provider
./start-provider.sh  # or equivalent command
```

The provider connects via gRPC to the core backend on the configured `GRPC_PORT` (default: 50051).

## Full Stack Development

To run all three services together:

### Terminal 1 — Core Backend
```bash
cd intellisim.core
./bin/intelli-sim.exe
# Backend: http://localhost:8080
# gRPC:    localhost:50051
```

### Terminal 2 — LLM Provider
```bash
cd ../intellisim-openai-provider  # or your provider directory
./start-provider.sh
# Connects to gRPC backend at localhost:50051
```

### Terminal 3 — Frontend
```bash
cd ../intellisim.front
npm install
npm run dev
# Frontend: http://localhost:3000
# Connects to backend API at http://localhost:8080
```

The frontend will automatically connect to the backend API, which will route LLM calls through the integrated gRPC bridge to the external provider.

### Docker Compose (Alternative)

For simplified full-stack deployment, create a `docker-compose.yml` in the workspace root orchestrating all three services with proper networking.
