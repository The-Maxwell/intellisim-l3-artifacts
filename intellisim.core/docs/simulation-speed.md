# Simulation Speed Control

Simulation speed is a **backend-controlled parameter** that paces how fast the run
engine produces steps. It can be set when starting a run and changed **live**
mid-run. The frontend slider is only a UI for this backend value — it is not a
frontend-only playback concern.

Speed is modeled as an **inter-step delay** in milliseconds (`speed_ms`): the run
loop sleeps this long between producing consecutive steps. `0` = fastest (no sleep);
higher = slower.

## Backend mechanism (`modules/simulations/`)

| Concern | Where |
|---------|-------|
| Default delay (env `SIMULATIONS_DEFAULT_STEP_DELAY_MS`, default `100`) | `run_config.go` — `DefaultStepDelayMs` |
| Run loop sleeps `delay` between steps | `engine.go` — `runLoop`, `select { case <-time.After(delay) }` |
| `ctrlSpeed` control action + `controlMsg.speedMs` | `engine.go` |
| Live change: `handleControl` mutates the loop's `delay` pointer | `engine.go` — `case ctrlSpeed` |
| REST endpoint `POST /simulations/:id/control` binds `speed_ms` | `run_api.go` — `ControlSimulation` |
| Request DTO (`SpeedMs *int`, validated `min=0,max=10000`, 0 = fastest); `action: set_speed` requires `speed_ms` (`required_if`) | `run_dtos.go` — `ControlRequest` |
| Service sends the lifecycle verb + optional separate `ctrlSpeed` message for pause/resume/stop; for the dedicated `set_speed` action it sends ONLY `ctrlSpeed` (no lifecycle verb) — legal while `running` or `paused`, so the speed slider works on a live run without piggybacking on pause/resume | `run_service.go` — `Control(...)`, `run_validation.go` — `ValidateControl` |

The WebSocket stream (`/simulations/:id/ws`, `stream.go`) only **broadcasts**
produced step frames — it carries no speed control. Speed is controlled
exclusively through the REST `/control` endpoint.

### Flow

```
slider → store.setSpeed / control → POST /:id/control {speed_ms}
       → runService.Control → RunManager.Control(ctrlSpeed, speedMs)
       → handleControl mutates run loop's `delay`
       → runLoop sleeps time.After(delay) between steps
```

Because `delay` is a pointer read on each loop iteration, a mid-run `speed_ms`
message re-paces step production immediately.

## Frontend (`intellisim.front`)

- `RunControls.vue` — speed slider over discrete delays `[1000, 500, 250, 100, 50, 0]` ms;
  emits `('speed', ms)`.
- `runStore.ts` — `speedMs` ref; `setSpeed()` / `control()` POST `speed_ms` to `/control`.
- The same `speedMs` is *also* passed to the PixiJS canvas (`RunWorldCanvas.vue`,
  `stepDelayMs` prop) purely to size the animation tween so rendering matches the
  frame arrival pace. This is a rendering hint, not the source of truth.

## Not the same thing: finished-run replay

`RunDashboard.vue` has a **separate, frontend-only** player (`startPlay()` with a
fixed `setInterval(150ms)`) for scrubbing through already-recorded history of a
finished run. This is pure browser playback and is unrelated to the live-run
`speed_ms`.
