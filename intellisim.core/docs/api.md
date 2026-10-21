# API Reference

Base path: `/api/v1/intellisim`

## Health

| Method | Path | Description |
|--------|------|-------------|
| GET | `/status` | Returns `{"status": "Ok"}` |

## LLMs (`/llms`)

Managed via `modules/llm/api.go`. Handles provider registry and LLM configuration.

| Method | Path | Description | Query Params |
|--------|------|-------------|--------------|
| GET | `/llms` | List LLM providers, optionally filtered | `capability`, `model` |

## Chats (`/chats`)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/chats` | List all chats (paginated) |
| POST | `/chats` | Create a new chat |
| GET | `/chats/:id` | Get chat by ID |
| DELETE | `/chats/:id` | Delete chat |
| GET | `/chats/:id/messages` | Get chat messages |
| POST | `/chats/:id/messages` | Send message (triggers LLM response) |

## Models (`/models`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/models` | owner/admin | Create model (manual / LLM-seeded / with-LLM kickoff) |
| GET | `/models` | owner/admin | List caller's models (admin: all). Query: `page`, `page_size` (1–100, default 20), `sort_by` (`name`\|`updated_at`\|`created_by`), `sort_order` (`asc`\|`desc`). Response: `{models, page, page_size, total, has_more}` (`total` = full match count for the paginator). |
| GET | `/models/options` | owner/admin | Lightweight **unpaginated** id+name list of all caller's models (admin: all), sorted by name — for selector UIs (e.g. the simulations model filter). Response: `{options: [{id, name}]}`. |
| GET | `/models/:id` | owner/admin | Full model + advisory validation |
| PUT | `/models/:id` | owner/admin | Full manual replace (does not touch the image library) |
| DELETE | `/models/:id` | owner/admin | Delete model (+ companion chat, messages) |
| POST | `/models/:id/duplicate` | owner/admin | Deep-copy a model (201 `ModelResponse`). Also clones its companion chat — target/config + full message history, re-linked to the new model (best-effort: a chat-clone failure leaves the copy chat-less). Body: `{"name"?: "..."}` (default `"<source name> (copy)"`). |
| GET | `/models/:id/validation` | owner/admin | Recompute advisory completeness validation |
| GET | `/models/:id/chat` | owner/admin | Get companion chat meta + target + status |
| PUT | `/models/:id/chat` | owner/admin | Set/replace LLM target (creates the chat if absent) |
| GET | `/models/:id/chat/messages` | owner/admin | Cursor-paginated chat history |
| POST | `/models/:id/chat/messages` | owner/admin | Enqueue async generate/edit (optional scope) |
| GET | `/models/:id/chat/events` | owner/admin | SSE stream — real-time model-chat events |
| POST | `/models/:id/images` | owner/admin | Add one image to the managed library (201 ModelResponse) |
| DELETE | `/models/:id/images/:imageId` | owner/admin | Remove one image from the library by UID (200 ModelResponse) |
| POST | `/models/:id/environment/generate` | owner/admin | Enqueue async environment generation from stored images (Vision, 202) |

The model generation flow: user creates a model, optionally attaches a companion chat, then sends messages to refine it. Images for environment generation are managed via the `/images` sub-resource and analysed all at once in a single vision call via `/environment/generate`. SSE `model.updated` delivers the result.

## Simulations (`/simulations`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/simulations` | owner/admin | Create simulation from model (async 202 + SSE `generation.progress/completed/failed`) |
| GET | `/simulations` | owner/admin | List caller's simulations (admin: all). Query: `page`, `page_size` (1–100, default 20), `model_id` (**repeatable** — `?model_id=a&model_id=b` filters by several models via `$in`), `sort_by` (`name`\|`model_name`\|`status`\|`steps`\|`agent_count`\|`total_tokens`\|`updated_at`\|`created_by`), `sort_order` (`asc`\|`desc`). Response: `{items, total, has_more}`. |
| GET | `/simulations/:id` | owner/admin | Get simulation |
| GET | `/simulations/:id/model-snapshot` | owner/admin | Full frozen source-model definition the simulation was generated from (`models.ModelResponse`, same shape as `GET /models/:id`, incl. rules/interactions/events/llm_config/widgets/images). Distinct from the trimmed inline `model_snapshot` on the simulation response. `404 model_snapshot_unavailable` if absent. |
| PATCH | `/simulations/:id` | owner/admin | Update simulation (manual edit) |
| DELETE | `/simulations/:id` | owner/admin | Delete simulation (cascade: steps + analyses + analysis files) |
| GET | `/simulations/:id/events` | owner/admin | SSE hub — generation + run + analysis lifecycle events |

### Run Control Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/simulations/:id/run` | owner/admin | Start a run (202 async) |
| POST | `/simulations/:id/control` | owner/admin | Control a live run: `{"action": "pause"\|"resume"\|"stop"\|"set_speed", "speed_ms": <ms>}` (`speed_ms` required for `set_speed`; `set_speed` is legal while running or paused and sends no lifecycle verb) |
| POST | `/simulations/:id/stream-ticket` | owner/admin | Issue a single-use WebSocket connect ticket |
| GET | `/simulations/:id/ws?ticket=…` | ticket | WebSocket step stream (per-step agent frames; skips Bearer middleware, auth via ticket) |
| GET | `/simulations/:id/steps` | owner/admin | Replay history (paginated timeseries query; `to` defaults to the last recorded step when omitted) |

Run lifecycle SSE events: `run.progress`, `run.completed`.

### Results Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/simulations/:id/results/summary` | Per-archetype population + property stats snapshot |
| GET | `/simulations/:id/results/series` | Time-series data (property values over steps) |

### Analysis Endpoints (Level-3)

Analysis is async: `POST /:id/analyses` returns **202** immediately; progress rides the SSE hub. PDFs are stored in MongoDB GridFS bucket `analysis_files`.

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/simulations/:id/analyses` | owner/admin | Start analysis (202 async). Body: `{"focus": [...], "aggregation": {...}, "language": "en"\|"uk"}`. SSE: `analysis.progress`, `analysis.completed`, `analysis.failed`. |
| GET | `/simulations/:id/analyses` | owner/admin | List analyses for this simulation |
| GET | `/simulations/:id/analyses/:aid` | owner/admin | Get one analysis record |
| DELETE | `/simulations/:id/analyses/:aid` | owner/admin | Delete analysis record + GridFS PDF |
| GET | `/simulations/:id/analyses/:aid/file` | owner/admin | Stream the PDF from GridFS (`Content-Type: application/pdf`) |

## WebSocket

| Protocol | Path | Description |
|----------|------|-------------|
| WS | `/simulations/:id/ws?ticket=…` | Real-time per-step agent frames during a run. Authenticated via single-use ticket from `POST /:id/stream-ticket`. |

See [WebSocket Protocol](websocket-protocol.md) for detailed message formats.

## gRPC

| Service | Method | Description |
|---------|--------|-------------|
| LLMBridge | Custom | External LLM provider bridge service for extensibility |

See [LLM Integration](llm-integration.md) for gRPC service definitions and provider implementation details.

## Environment Variables

```env
# HTTP Server
PORT=8080                     # HTTP server port (default: 8080)
GRPC_PORT=50051              # gRPC server port (default: 50051)

# MongoDB
MONGO_HOST=localhost:27017    # MongoDB connection host
MONGO_DATABASE=intellisim     # Database name
MONGO_USERNAME=               # MongoDB username (optional)
MONGO_PASSWORD=               # MongoDB password (optional)
MONGO_AUTH_SOURCE=            # MongoDB auth source database (optional)
MONGO_REPLICA_SET=            # MongoDB replica set name (optional)

# API Configuration
OPENAI_API_KEY=sk-...         # OpenAI API key
WS_ALLOWED_ORIGIN=*           # WebSocket allowed origins (default: *)

# Logging
LOG_LEVEL=info                # Logging level (info/debug)
```
