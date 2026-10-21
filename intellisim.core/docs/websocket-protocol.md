# Simulation Run Streaming Protocol

How the frontend receives a live simulation run from the backend. The run phase uses **two channels**:

- **WebSocket** (`/:id/ws`) — high-frequency per-step **agent frames** (positions/properties/state), chunked. Authenticated by a short-lived **connect ticket**.
- **SSE** (`/:id/events`) — authoritative run **lifecycle**: `run.progress` (step counter + status) and `run.completed` (terminal). Shares the existing simulations SSE hub (bearer-auth via `fetch-event-source`). The UI relies on SSE for status/completion even if the WS never connects (e.g. a very fast run).

Run **control** (start / pause / resume / stop / speed) is plain authenticated REST, not the socket.

Source of truth: `modules/simulations/{stream,ticket,engine,run_api}.go` and `intellisim.front/src/modules/simulation/composables/useSimulationRunSocket.ts`.

---

## Connect-ticket auth flow

The browser WebSocket API cannot send an `Authorization` header, and the core auth middleware is header-only. So the WS is authenticated with a single-use, short-lived ticket minted over authenticated REST:

1. `POST /api/v1/intellisim/simulations/{id}/stream-ticket` (bearer-authed, owner/admin only) →
   ```json
   { "ticket": "<opaque 256-bit hex>", "expires_at": "2026-06-13T16:31:55Z" }
   ```
   Only issued when the simulation is **streamable** (status `running` or `paused`); otherwise `403 invalid_ticket`/`ErrNotStreamable`. TTL = `SIMULATIONS_STREAM_TICKET_TTL_S` (default 30s).
2. `GET /api/v1/intellisim/simulations/{id}/ws?ticket=<ticket>` → WebSocket upgrade. The handler **consumes** the ticket (single-use), re-checks ownership + streamable status, checks `Origin` against `WS_ALLOWED_ORIGIN`, then joins the simulation's room. The `*/ws` route is the only one that bypasses the header-bearer middleware (it authenticates via the ticket).
3. On reconnect the client mints a **fresh** ticket (never replays a consumed one). A 4xx ticket response (e.g. run finished → no room) is terminal — the client stops reconnecting and falls back to history (`GET /:id/steps`).

---

## Room architecture

One room per simulation (`stream.go`), created on run start, torn down on completion/abandonment. A room actor goroutine serializes register/unregister/broadcast; each client has a buffered `send` channel with a `writePump` (ping ticker, write deadline, **newline-coalescing** of queued frames) and a `readPump` (drains inbound). A client whose buffer fills is dropped (best-effort delivery — the client refetches missed steps via history). Delivery is best-effort and never blocks the engine.

---

## Message envelope: `StreamMessage`

The server may pack several envelopes into one WS message separated by `\n`; the client splits on newline and JSON-parses each part.

```jsonc
{
  "type": "step | keyframe | status | progress | error | completion",
  "step": 42,                       // step/keyframe/progress
  "chunk_index": 0,                 // step/keyframe chunking
  "total_chunks": 3,
  "agents": [                       // step = CHANGED agents; keyframe = ALL agents
    { "id": 24, "position": {"x": 88, "y": 11}, "properties": {"energy": 56.7}, "state": "hunting" }
  ],
  "global_properties": { "season": "spring" },
  "statistics": { /* SimulationStatistics — streamed LIVE per step */ },
  "token_usage": { /* SimulationTokenUsage — cumulative run-so-far breakdown */ },
  "step_tokens": {                  // per-step delta (tokens spent THIS step)
    "prompt_tokens": 120, "completion_tokens": 60,
    "reasoning_tokens": 40, "total_tokens": 180, "calls": 2
  },
  "status": "running",              // status/completion
  "error": "…",                     // error
  "timestamp": 1750000000000        // unix millis
}
```

| `type` | Meaning |
|---|---|
| `step` | One tick's **changed** agents + globals/stats (chunked by a batch size). A separate `step` frame per tick carries `token_usage` + `step_tokens`. |
| `keyframe` | Full snapshot of all agents (periodic / for late joiners + reconnects). A `token_usage` sync frame (zero `step_tokens`) follows a keyframe. |
| `status` | Run status string (`running`/`paused`/…). |
| `progress` | Reserved on the WS (the step counter advances; lifecycle authoritatively rides SSE). |
| `completion` | Terminal status; the server then closes the room. |
| `error` | Transport/run error note. |

Positions are `common.Point` (`{x,y}`), agent ids are `uint64` runtime instance ids.

### Live statistics, token usage & per-step delta

- **`statistics`** now streams **live per step** (the env-row `SimulationStatistics` snapshot, `ComputeStatistics(rs)`) — byte-identical to what `GET /:id/steps` returns on replay, so the frontend reuses one type. (Previously the env row's stats were dropped on the live path and surfaced only on replay.) It rides the first chunk of a step/keyframe frame.
- **`token_usage`** is the run's **cumulative** LLM token-usage breakdown as of the step (`Total` + `by_model`/`by_agent`/`by_purpose`/`by_generation`/`by_run`), the same shape as the persisted `SimulationTokenUsage` DTO, so the run screen's `TokenUsagePanel` binds to it live. Emitted once per step (never per entity row) and on a keyframe.
- **`step_tokens`** is the **delta** for that one step (subtraction of the previous cumulative total). Keyframe token-sync frames carry a zero delta.
- **`reasoning_tokens`** (in every token dimension + the delta) is the reasoning portion of `completion_tokens` for reasoning models — a **subset**, already included in `completion_tokens`/`total_tokens`. It is a breakdown detail for cost insight, **never a summand**: `reasoning_tokens ≤ completion_tokens` and `total_tokens` already includes it.
- **Not persisted per step.** Token usage is streamed live but not written to the `simulation_steps` timeseries; the run-level aggregate on `Simulation.TokenUsage` (persisted at run start/pause/resume/finalize) remains authoritative. A mid-run crash loses the in-memory token accumulation since the last checkpoint (pre-existing behavior, unchanged).

---

## SSE lifecycle events (on `/:id/events`)

```jsonc
// event: run.progress
{ "run": 1, "step": 3, "total_steps": 6, "status": "running" }

// event: run.completed
{ "run": 1, "status": "finished", "steps_completed": 6, "error": "" }
```

The run view consumes these for the authoritative step counter + status + terminal transition (then refetches the final state + replay history). `simulation.updated` also fires after completion.

---

## REST surface (run phase)

| Method | Path | Purpose |
|---|---|---|
| POST | `/:id/run` | Start a ready run → `202` (status → running). |
| POST | `/:id/control` | `{action: pause\|resume\|stop\|set_speed, speed_ms?}` → `200`. `speed_ms` is required for `set_speed` (legal while running or paused, no lifecycle verb sent); optional piggyback on pause/resume/stop otherwise. |
| POST | `/:id/stream-ticket` | Mint a WS connect ticket. |
| GET | `/:id/ws?ticket=…` | WebSocket step stream (ticket-authed). |
| GET | `/:id/steps?from&to&page&page_size` | Persisted step history (replay/scrub). Paginated **by logical step number**; an omitted `to` resolves server-side to the last recorded step (not 0); an explicit `to` (any value) passes through unchanged. |

---

## Frontend integration

`useSimulationRunSocket.ts` mints a ticket, opens `${VITE_WS_BASE_URL}/simulations/:id/ws?ticket=…`, routes `StreamMessage` by type, reconnects with a fresh ticket on drop (terminal on 4xx). The run view (`SimulationRunView.vue`) connects only when the run is active (or right after Start) and disconnects on completion; it subscribes to the SSE hub (`useSimulationEvents`) for `run.progress`/`run.completed`. `RunDashboard.vue` replays a finished run from `GET /:id/steps` (full range) with a scrub/play timeline.

---

## Config

`WS_ALLOWED_ORIGIN` (default `http://localhost:3000`), `SIMULATIONS_STREAM_TICKET_TTL_S` (30), `SIMULATIONS_STEP_FLUSH_SIZE`/`SIMULATIONS_STEP_FLUSH_INTERVAL_MS` (step persistence), `SIMULATIONS_DEFAULT_STEP_DELAY_MS` (run speed), `SIMULATIONS_MAX_CONCURRENT_RUNS`, `SIMULATIONS_DEFAULT_PARALLEL_WORKERS`.
