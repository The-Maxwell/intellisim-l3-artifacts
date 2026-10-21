# Architecture

## Overview

IntelliSim is an LLM-based meta-system for agent-based modeling (ABM) simulation. Users describe simulations in natural language; the system generates agent models, rules, and interactions, then executes simulations with configurable LLM integration depth.

The project follows a scientific foundation described in the article "Проєктування LLM-базованої метасистеми для моделювання агентно-орієнтованого моделювання" (Pryshchepa, Troyanovsky).

## System Architecture

```
┌─────────────────┐     HTTP/WS      ┌──────────────────────────────┐
│  Vue 3 Frontend │ ◄──────────────► │      Go Backend (Echo)       │
│  (intellisim.   │                  │                              │
│   front)        │                  │  ┌─────────┐  ┌───────────┐ │
└─────────────────┘                  │  │ Modules  │  │    API    │ │
                                     │  │ (chats,  │  │  Router   │ │
                                     │  │  models, │  └───────────┘ │
                                     │  │  llm,    │                │
                                     │  │  sims)   │  ┌───────────┐ │
                                     │  └─────────┘  │  MongoDB   │ │
                                     │               └───────────┘ │
                                     │  ┌──────────────────────────┐│
                                     │  │  gRPC Server + LLM Bridge││
                                     │  │  (external providers)    ││
                                     │  └──────────────────────────┘│
                                     └──────────────────────────────┘
                                              ▲
                                              │ gRPC
                                              │
                                     ┌────────┴────────┐
                                     │  LLM Providers  │
                                     │  (OpenAI, etc)  │
                                     └─────────────────┘
```

## Tech Stack

| Component | Technology | Purpose |
|-----------|-----------|---------|
| Language | Go 1.25.5 | Backend server |
| HTTP | Echo v4 | REST API + middleware (CORS, logging, recovery) |
| DI | Uber FX | Dependency injection, lifecycle management |
| Database | MongoDB | Documents, timeseries collections |
| WebSocket | gorilla/websocket | Real-time simulation step broadcasting |
| gRPC | google.golang.org/grpc | LLM provider communication (`llmbridge.v1.LLMBridge`) |
| Protocol Buffers | google.golang.org/protobuf | Message serialization (contract owned by `github.com/The-Maxwell/intellisim.llm.provider.go.sdk`) |
| Logging | Uber Zap | Structured logging |
| Config | caarlos0/env | Environment variable parsing |
| Validation | go-playground/validator | Request struct validation |
| Frontend | Vue 3 + TypeScript + PrimeVue + Vite | Web UI |

## Project Structure

```
intellisim.core/
├── cmd/intelli-sim/     # Entry point: main.go + app.go (FX wiring)
├── api/                 # Route setup, delegates to module APIs
├── modules/
│   ├── chats/           # Chat CRUD (conversations for model generation)
│   ├── llm/             # LLM Service (validating consumer API), ProviderBridge (gRPC LLMBridge server), registry, context fitting, token estimation
│   ├── models/          # Model generation from LLM, validation, mapping
│   └── simulations/     # Simulation engines, rule engine, broadcaster
├── data/
│   ├── chats/           # ChatDocument, ChatMessageDocument, repository
│   ├── models/          # ModelDocument (agents, rules, interactions), repository
│   └── simulations/     # SimulationDocument, SimulationStepDocument, repository
├── scripts/
│   └── gen-llmbridge-proto.ps1  # Regenerates llmbridge proto from the SDK repo checkout
└── pkg/
    ├── httpserver/      # Echo server setup + lifecycle hooks
    ├── grpcserver/      # gRPC server setup + lifecycle hooks
    ├── mongodb/         # Client, indexes, timeseries, URI builder
    ├── logs/            # Zap logger configuration
    └── dtos/            # Shared DTOs (pagination, chat)
```

## Module Pattern

Each module follows a consistent structure:
- **fx.go** — `fx.Module` declaration with `fx.Provide()` for all components
- **api.go** — HTTP handlers (Echo handler functions)
- **service.go** — Business logic
- **mapper.go** — DTO ↔ Document mapping
- **dtos.go** — Request/response types
- **repository.go** (in `data/`) — MongoDB CRUD operations

## Boot Sequence

1. `main.go` calls `InitializeIntelliSimCoreApp()` in `app.go`
2. FX wires all modules: `llm.Module`, `chats.Module`, `models.Module`, `simulations.Module`
3. Core providers: logger, MongoDB client, repositories, API router, HTTP server
4. Lifecycle hooks start MongoDB connection and HTTP server
5. Routes registered via `api.SetupRoutes(echo)` under `/api/v1/intellisim/`

## LLM Integration Strategies

The system supports 6 strategies for LLM involvement in simulations, configurable per model:

| Strategy | Key | Description |
|----------|-----|-------------|
| None | `none` | Pure rule-based execution, no LLM calls |
| Individual Full | `individual_full` | LLM controls each agent (movement, properties, interactions) |
| Archetype Full | `archetype_full` | LLM provides strategies per agent archetype |
| Individual Decision | `individual_decision` | LLM makes decisions, rule engine executes them |
| Archetype Decision | `archetype_decision` | LLM makes archetype-level decisions, rules execute |
| Hybrid | `hybrid` | Mix strategies per archetype via `ArchetypeStrategies` map |

Performance controls: `DecisionFrequency` (call LLM every N steps), `BatchSize` (agents per LLM call), `MemorySize` (agent memory truncation).

## Data Flow

```
User chat message
  → LLM generates/refines model (agents, rules, interactions, environment)
    → Model stored in MongoDB
      → User creates Simulation from Model
        → Engine runs steps (rule evaluation + optional LLM calls)
          → Steps stored in timeseries collection
          → Steps broadcast via WebSocket to frontend
```

## MongoDB Collections

| Collection | Type | Purpose |
|-----------|------|---------|
| `chats` | Regular | Chat sessions |
| `chat_messages` | Regular | Messages within chats |
| `models` | Regular | ABM model definitions (embedded agents, rules, interactions) |
| `simulations` | Regular | Simulation runs (embeds denormalized model snapshot) |
| `simulation_steps` | Timeseries | Step data (metadata.simulation_id, timestamp-indexed) |
