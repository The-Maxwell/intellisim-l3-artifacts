import { test, expect, type Page, type APIRequestContext } from '@playwright/test'
import { apiContext, apiJson, pollUntil, u, API_BASE } from '../_helpers/api'
import { login } from '../_helpers/login'
import {
  minimalSpatial,
  seedModel,
  seedReadySim,
  runToCompletion,
  uniqueName,
  deleteQuietly,
} from '../_helpers/fixtures'

// §5.12 ERR — error matrix & resilience. Deterministic (NO LLM): the fixture
// model uses no LLM anywhere, so generation is local/instant.
//
// DELIVERABLE: assert BOTH the HTTP status and the `code` field of the error
// envelope `{code, message, fields?}` for each case, and document where reality
// differs from the plan's expected codes. Every observed status+code is recorded
// in the block comment at the bottom of this file and echoed in the agent report.
//
// Endpoints are hit raw via APIRequestContext (paths wrapped in u()); ERR-05/06/07
// drive the browser. Bogus but well-formed ids are 24-hex; malformed ids are
// non-hex. Unknown API routes now return 404 {code:'not_found'} and a wrong
// method on a real path returns 405 {code:'method_not_allowed'} (was 500) —
// both covered in ERR-01 below.

const BOGUS = '000000000000000000000000' // valid 24-hex ObjectID that never exists

// Full-body sim update built from a GET response (partial bodies fail request
// validation before the editable-state check, masking the 409 we assert).
function toUpdateBody(sim: any): Record<string, unknown> {
  return {
    name: sim.name,
    description: sim.description ?? '',
    steps: sim.steps,
    seed: sim.seed,
    llm_generation_target: sim.llm_generation_target ?? null,
    llm_execution_target: sim.llm_execution_target ?? null,
    agents: sim.agents ?? [],
    environment: sim.environment ?? null,
    widgets: sim.widgets ?? [],
  }
}

// Read an error envelope (status + parsed body) from any api.* response.
async function envelope(res: { status(): number; text(): Promise<string> }) {
  const status = res.status()
  const text = await res.text()
  let body: any = null
  try {
    body = text ? JSON.parse(text) : null
  } catch {
    body = { raw: text }
  }
  return { status, body }
}

// Poll a simulation to `running` after POST /run (never send control before the
// async run has actually started — see the fixtures/runToCompletion note).
async function pollRunning(api: APIRequestContext, simId: string): Promise<void> {
  await pollUntil(
    async () => {
      const s = await apiJson<{ status: string }>(api, 'get', `/simulations/${simId}`)
      if (['finished', 'stopped', 'failed'].includes(s.status)) throw new Error(`run ended early: ${s.status}`)
      return s.status === 'running'
    },
    { timeoutMs: 20_000, intervalMs: 250, label: `sim ${simId} running` },
  )
}

async function stopQuietly(api: APIRequestContext, simId: string): Promise<void> {
  await api.post(u(`/simulations/${simId}/control`), { data: { action: 'stop' } }).catch(() => null)
}

// ---------------------------------------------------------------------------
// In-page WebSocket probes (ERR-05). Opened from a page loaded on
// http://localhost:3000 so the browser sends Origin: http://localhost:3000
// (core WS_ALLOWED_ORIGIN default) — the WS itself authenticates via the ticket,
// not a bearer header (the browser WS API can't send one).
// ---------------------------------------------------------------------------

interface WsResult {
  opened: boolean
  closeCode: number | null
  errored: boolean
  timeout: boolean
}

// Open a single WS, resolve on the first of open / close / error / timeout.
async function wsProbe(page: Page, wsUrl: string): Promise<WsResult> {
  return page.evaluate(
    (url) =>
      new Promise<WsResult>((resolve) => {
        let settled = false
        const done = (r: WsResult) => {
          if (!settled) {
            settled = true
            resolve(r)
          }
        }
        let ws: WebSocket
        try {
          ws = new WebSocket(url)
        } catch {
          done({ opened: false, closeCode: null, errored: true, timeout: false })
          return
        }
        const timer = setTimeout(() => {
          try {
            ws.close()
          } catch {
            /* noop */
          }
          done({ opened: false, closeCode: null, errored: false, timeout: true })
        }, 4000)
        ws.onopen = () => {
          clearTimeout(timer)
          done({ opened: true, closeCode: null, errored: false, timeout: false })
          try {
            ws.close()
          } catch {
            /* noop */
          }
        }
        ws.onerror = () => {
          clearTimeout(timer)
          done({ opened: false, closeCode: null, errored: true, timeout: false })
        }
        ws.onclose = (ev) => {
          clearTimeout(timer)
          done({ opened: false, closeCode: ev.code, errored: false, timeout: false })
        }
      }),
    wsUrl,
  )
}

// Open ws1 (hold it open once connected), THEN open ws2 with the same URL; return
// both outcomes. Proves single-use: ws1 opens, ws2 (reused ticket) is rejected.
async function wsSingleUseProbe(
  page: Page,
  wsUrl: string,
): Promise<{ first: WsResult; second: WsResult }> {
  return page.evaluate((url) => {
    const settle = (ws: WebSocket, holdOpen: boolean): Promise<WsResult> =>
      new Promise<WsResult>((resolve) => {
        let settled = false
        const done = (r: WsResult) => {
          if (!settled) {
            settled = true
            resolve(r)
          }
        }
        const timer = setTimeout(() => done({ opened: false, closeCode: null, errored: false, timeout: true }), 4000)
        ws.onopen = () => {
          clearTimeout(timer)
          done({ opened: true, closeCode: null, errored: false, timeout: false })
          // Deliberately keep ws open (holdOpen) so the ticket is unambiguously
          // consumed while the second connect is attempted.
          if (!holdOpen) {
            try {
              ws.close()
            } catch {
              /* noop */
            }
          }
        }
        ws.onerror = () => {
          clearTimeout(timer)
          done({ opened: false, closeCode: null, errored: true, timeout: false })
        }
        ws.onclose = (ev) => {
          clearTimeout(timer)
          done({ opened: false, closeCode: ev.code, errored: false, timeout: false })
        }
      })
    return (async () => {
      const ws1 = new WebSocket(url)
      const first = await settle(ws1, true)
      const ws2 = new WebSocket(url)
      const second = await settle(ws2, false)
      try {
        ws1.close()
      } catch {
        /* noop */
      }
      return { first, second }
    })()
  }, wsUrl)
}

function wsUrlFor(id: string, ticket: string): string {
  return `ws://localhost:8080/api/v1/intellisim/simulations/${id}/ws?ticket=${ticket}`
}

// ===========================================================================

test.describe('errors API matrix', () => {
  let api: APIRequestContext
  let model: { id: string; name: string }
  let readySim: { id: string; name: string } // a plain ready F1 sim (steps small)
  const simIds: string[] = []
  const modelIds: string[] = []
  const runningSimIds: string[] = [] // stop these in cleanup

  test.beforeAll(async () => {
    api = await apiContext()
    model = await seedModel(api, minimalSpatial(uniqueName('mdl-err')))
    modelIds.push(model.id)
    readySim = await seedReadySim(api, model.id, { steps: 10, seed: 42, name: uniqueName('sim-err-ready') })
    simIds.push(readySim.id)
  })

  test.afterAll(async () => {
    for (const id of runningSimIds) await stopQuietly(api, id)
    for (const id of simIds) await deleteQuietly(api, `/simulations/${id}`)
    for (const id of modelIds) await deleteQuietly(api, `/models/${id}`)
    await api.dispose()
  })

  // -------------------------------------------------------------------------
  // ERR-01 — 404 family
  // -------------------------------------------------------------------------
  test('ERR-01 404 family: bogus + malformed ids return {code} envelopes', async () => {
    // Valid-shape (24-hex) but nonexistent ids.
    const cases: Array<{ m: 'get' | 'put' | 'delete'; path: string; body?: unknown; code: string }> = [
      { m: 'get', path: `/models/${BOGUS}`, code: 'not_found' },
      { m: 'put', path: `/models/${BOGUS}`, body: { model: { name: 'x' } }, code: 'not_found' },
      { m: 'delete', path: `/models/${BOGUS}`, code: 'not_found' },
      { m: 'get', path: `/simulations/${BOGUS}`, code: 'not_found' },
      // sim-not-found short-circuits before analysis lookup → not_found (not analysis_not_found)
      { m: 'get', path: `/simulations/${BOGUS}/analyses/${BOGUS}`, code: 'not_found' },
      { m: 'get', path: `/simulations/${BOGUS}/analyses`, code: 'not_found' },
      // snapshot on a bogus sim: sim-not-found wins over model_snapshot_unavailable
      { m: 'get', path: `/simulations/${BOGUS}/model-snapshot`, code: 'not_found' },
    ]
    for (const c of cases) {
      const res = await api[c.m](u(c.path), c.body !== undefined ? { data: c.body } : undefined)
      const { status, body } = await envelope(res)
      expect(status, `${c.m.toUpperCase()} ${c.path} status`).toBe(404)
      expect(body?.code, `${c.m.toUpperCase()} ${c.path} code`).toBe(c.code)
    }

    // agent_not_found needs a REAL sim + a nonexistent agent id.
    {
      const res = await api.get(u(`/simulations/${readySim.id}/agents/999999`))
      const { status, body } = await envelope(res)
      expect(status).toBe(404)
      expect(body?.code).toBe('agent_not_found')
    }

    // Malformed (non-hex) ids: FINDING — the backend resolves them to 404
    // not_found (graceful), NOT 400 or 500. Documented per plan's "400 vs 404 vs 500!".
    for (const bad of ['not-a-valid-id', 'zzz', '12345']) {
      const res = await api.get(u(`/models/${bad}`))
      const { status, body } = await envelope(res)
      expect(status, `GET /models/${bad} status`).toBe(404)
      expect(body?.code, `GET /models/${bad} code`).toBe('not_found')
    }
    {
      const res = await api.get(u('/simulations/zzz'))
      const { status, body } = await envelope(res)
      expect(status).toBe(404)
      expect(body?.code).toBe('not_found')
    }
  })

  test('ERR-01 routing: unknown route → 404 not_found; wrong method on a real path → 405 method_not_allowed', async () => {
    // Unknown route under the API prefix → 404 {code:'not_found'} (standard
    // envelope), NOT a bare 404 or 500.
    {
      const res = await api.get(u('/definitely-not-a-route'))
      const { status, body } = await envelope(res)
      expect(status, 'unknown API route status').toBe(404)
      expect(body?.code, 'unknown API route code').toBe('not_found')
    }

    // Wrong method on a REAL path (/models supports GET+POST, not PATCH) →
    // 405 {code:'method_not_allowed'} (was 500 internal).
    {
      const res = await api.patch(u('/models'))
      const { status, body } = await envelope(res)
      expect(status, 'PATCH /models status').toBe(405)
      expect(body?.code, 'PATCH /models code').toBe('method_not_allowed')
    }
    // Same on a param path: PATCH /models/:id is not registered → 405.
    {
      const res = await api.patch(u(`/models/${BOGUS}`))
      const { status, body } = await envelope(res)
      expect(status, 'PATCH /models/:id status').toBe(405)
      expect(body?.code, 'PATCH /models/:id code').toBe('method_not_allowed')
    }
  })

  // -------------------------------------------------------------------------
  // ERR-02 — 409 family (+ documented deltas)
  // -------------------------------------------------------------------------
  test('ERR-02 409 family: not_running (pause ready) + not_streamable (ticket on ready)', async () => {
    // Pause a `ready` sim → 409 not_running.
    {
      const res = await api.post(u(`/simulations/${readySim.id}/control`), { data: { action: 'pause' } })
      const { status, body } = await envelope(res)
      expect(status).toBe(409)
      expect(body?.code).toBe('not_running')
    }
    // Stream-ticket for a `ready` (non-streamable) sim → 409 not_streamable.
    {
      const res = await api.post(u(`/simulations/${readySim.id}/stream-ticket`), { data: {} })
      const { status, body } = await envelope(res)
      expect(status).toBe(409)
      expect(body?.code).toBe('not_streamable')
    }
  })

  test('ERR-02 running-sim guards: not_editable (PUT running), resume-while-running, second /run', async () => {
    test.setTimeout(90_000)
    // Long run so it stays alive through the assertions.
    const sim = await seedReadySim(api, model.id, { steps: 5000, seed: 1, name: uniqueName('sim-err-run') })
    simIds.push(sim.id)
    runningSimIds.push(sim.id)
    await apiJson(api, 'post', `/simulations/${sim.id}/run`, {}, 202)
    await pollRunning(api, sim.id)

    const running = await apiJson<any>(api, 'get', `/simulations/${sim.id}`)

    // (a) not_editable: full-body PUT while running → 409 not_editable.
    {
      const res = await api.put(u(`/simulations/${sim.id}`), { data: toUpdateBody(running) })
      const { status, body } = await envelope(res)
      expect(status, await res.text().catch(() => '')).toBe(409)
      expect(body?.code).toBe('not_editable')
    }

    // (b) resume-while-running: FINDING — the backend REJECTS resume on a
    // running sim with 409 not_running. The frontend's speed-change path is NOT
    // "resume while running" (that 409s); a running speed change must go through
    // pause→resume. Important delta vs the original plan assumption.
    {
      const res = await api.post(u(`/simulations/${sim.id}/control`), {
        data: { action: 'resume', speed_ms: 100 },
      })
      const { status, body } = await envelope(res)
      expect(status).toBe(409)
      expect(body?.code).toBe('not_running')
    }

    // (c) run_in_flight: a second POST /run while status=running is rejected
    // with 409 run_in_flight (the in-flight-slot guard now precedes the
    // ready-state check, so a running/paused sim conflicts as run_in_flight,
    // not 422 not_runnable).
    {
      const res = await api.post(u(`/simulations/${sim.id}/run`), { data: {} })
      const { status, body } = await envelope(res)
      expect(status).toBe(409)
      expect(body?.code).toBe('run_in_flight')
    }

    await stopQuietly(api, sim.id)
    await pollUntil(
      async () => {
        const s = await apiJson<{ status: string }>(api, 'get', `/simulations/${sim.id}`)
        return ['stopped', 'finished', 'failed'].includes(s.status) ? s.status : null
      },
      { timeoutMs: 20_000, label: 'run stopped' },
    )
  })

  test('ERR-02 run_in_flight race: fire two /run without awaiting — loser is 409 run_in_flight', async () => {
    test.setTimeout(60_000)
    const sim = await seedReadySim(api, model.id, { steps: 5000, seed: 2, name: uniqueName('sim-err-race') })
    simIds.push(sim.id)
    runningSimIds.push(sim.id)

    // Fire both back-to-back without awaiting the first (Promise.all) to catch
    // the RunManager in-flight slot while status is still `ready`.
    const [a, b] = await Promise.all([
      api.post(u(`/simulations/${sim.id}/run`), { data: {} }),
      api.post(u(`/simulations/${sim.id}/run`), { data: {} }),
    ])
    const ra = await envelope(a)
    const rb = await envelope(b)
    const statuses = [ra.status, rb.status].sort()
    const codes = [ra.body?.code, rb.body?.code]

    // Exactly one 202 wins; the loser is 409 run_in_flight (the in-flight-slot
    // guard now precedes the state check, so the second request always conflicts
    // as run_in_flight — whether or not status has flipped to running yet).
    console.log(`[ERR-02 race] statuses=${JSON.stringify(statuses)} codes=${JSON.stringify(codes)}`)
    expect(statuses).toContain(202)
    const loser = ra.status === 202 ? rb : ra
    expect(loser.status).toBe(409)
    expect(loser.body?.code).toBe('run_in_flight')

    await pollRunning(api, sim.id).catch(() => null)
    await stopQuietly(api, sim.id)
  })

  test('ERR-02 generation_in_progress race: fire two /generate without awaiting', async () => {
    test.setTimeout(60_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 3, name: uniqueName('sim-err-gen') })
    simIds.push(sim.id)

    // Regenerate is <1s locally; fire two full regenerations back-to-back.
    const [a, b] = await Promise.all([
      api.post(u(`/simulations/${sim.id}/generate`), { data: { scope: { kind: 'full' } } }),
      api.post(u(`/simulations/${sim.id}/generate`), { data: { scope: { kind: 'full' } } }),
    ])
    const ra = await envelope(a)
    const rb = await envelope(b)
    console.log(`[ERR-02 gen race] statuses=${[ra.status, rb.status]} codes=${[ra.body?.code, rb.body?.code]}`)

    // At least one 202. The other is EITHER 409 generation_in_progress (race
    // observed) OR 202 (both slotted before the first registered — unobservable
    // locally). When 409 is seen, assert its code.
    expect([ra.status, rb.status]).toContain(202)
    const conflict = [ra, rb].find((r) => r.status === 409)
    if (conflict) {
      expect(conflict.body?.code).toBe('generation_in_progress')
    } else {
      // Documented as unobservable-locally on this run.
      expect([ra.status, rb.status].filter((s) => s === 202).length).toBeGreaterThanOrEqual(1)
    }

    // Wait for the sim to settle back to ready before teardown.
    await pollUntil(
      async () => {
        const s = await apiJson<{ status: string }>(api, 'get', `/simulations/${sim.id}`)
        return s.status === 'ready' ? true : null
      },
      { timeoutMs: 20_000, label: 'regen settled' },
    )
  })

  // -------------------------------------------------------------------------
  // ERR-03 — 422 / 400 validation family
  // -------------------------------------------------------------------------
  test('ERR-03 validation family: steps/speed/parts/model_not_ready/not_runnable/widget-matrix', async () => {
    test.setTimeout(90_000)

    // (a) sim create steps=0 → 422 validation_failed {steps: min=1}. Request-schema
    // validation failures are now 422 everywhere (was 400), with a `fields` map.
    {
      const res = await api.post(u('/simulations'), {
        data: { model_id: model.id, name: uniqueName('sim-steps0'), steps: 0 },
      })
      const { status, body } = await envelope(res)
      expect(status).toBe(422)
      expect(body?.code).toBe('validation_failed')
      expect(body?.fields?.steps).toBe('min=1')
    }

    // (b) control speed_ms out of range (>10000) with a valid action → 422
    // validation_failed {speed_ms: max=10000}. Request validation runs BEFORE
    // the run-state check, so a `ready` sim returns 422 (not 409) even though
    // `stop` would otherwise be state-legal.
    {
      const res = await api.post(u(`/simulations/${readySim.id}/control`), {
        data: { action: 'stop', speed_ms: 20000 },
      })
      const { status, body } = await envelope(res)
      expect(status).toBe(422)
      expect(body?.code).toBe('validation_failed')
      expect(body?.fields?.speed_ms).toBe('max=10000')
    }

    // (c) regenerate with an invalid scope part → 422 validation_failed
    // {scope.parts[0]: oneof=...}. GenerateRequest.scope = {kind, parts?}.
    {
      const res = await api.post(u(`/simulations/${readySim.id}/generate`), {
        data: { scope: { kind: 'agents', parts: ['bogus'] } },
      })
      const { status, body } = await envelope(res)
      expect(status).toBe(422)
      expect(body?.code).toBe('validation_failed')
      expect(body?.fields?.['scope.parts[0]']).toContain('oneof')
    }

    // (d) model_not_ready: create a sim from a bare draft model → 422.
    {
      const draft = await apiJson<{ id: string }>(api, 'post', '/models', { name: uniqueName('mdl-draft') }, 201)
      modelIds.push(draft.id)
      const res = await api.post(u('/simulations'), {
        data: { model_id: draft.id, name: uniqueName('sim-fromdraft'), steps: 5 },
      })
      const { status, body } = await envelope(res)
      expect(status).toBe(422)
      expect(body?.code).toBe('model_not_ready')
    }

    // (e) not_runnable: a ready sim with zero agents. Full-body PUT setting
    // agents:[] is ACCEPTED (200, stays ready) — documented; then POST /run → 422.
    {
      const sim = await seedReadySim(api, model.id, { steps: 10, seed: 9, name: uniqueName('sim-noagents') })
      simIds.push(sim.id)
      const current = await apiJson<any>(api, 'get', `/simulations/${sim.id}`)
      const put = await api.put(u(`/simulations/${sim.id}`), {
        data: { ...toUpdateBody(current), agents: [] },
      })
      expect(put.status(), 'PUT agents:[] is accepted (documented)').toBe(200)

      const run = await api.post(u(`/simulations/${sim.id}/run`), { data: {} })
      const { status, body } = await envelope(run)
      expect(status).toBe(422)
      expect(body?.code).toBe('not_runnable')
    }

    // (f) widget matrix violation via PUT /models: pie_chart + sim_metric source.
    // FINDING: model-save widget validation returns 422 validation_failed (with a
    // `fields` entry keyed by the widget path) — a DIFFERENT status than the 400
    // request-schema validation above; model field-shape validation is 422.
    {
      const mdl = minimalSpatial(uniqueName('mdl-widget'))
      const created = await seedModel(api, mdl)
      modelIds.push(created.id)
      mdl.widgets = [
        {
          id: 'w_bad',
          type: 'pie_chart',
          title: 'bad',
          source: { type: 'sim_metric', metric: 'total_agents' },
          position: { x: 0, y: 0, w: 3, h: 2 },
        },
      ]
      const res = await api.put(u(`/models/${created.id}`), { data: { model: mdl } })
      const { status, body } = await envelope(res)
      expect(status).toBe(422)
      expect(body?.code).toBe('validation_failed')
      expect(body?.fields?.['widgets[0].source.type']).toContain('not allowed')
    }
  })

  // -------------------------------------------------------------------------
  // ERR-04 — 400 family
  // -------------------------------------------------------------------------
  test('ERR-04 validation/bad_request family: bad capability (422) + seed overflow + negative seed (400)', async () => {
    // (a) GET /llms?capability=bogus → 422 standard envelope
    // {code:'validation_failed', message, fields:{capability}} (was a
    // non-standard 400 {error} envelope; now consistent with every other error).
    {
      const res = await api.get(u('/llms?capability=bogus'))
      const { status, body } = await envelope(res)
      expect(status).toBe(422)
      expect(body?.code).toBe('validation_failed')
      expect(Object.keys(body?.fields ?? {}), 'fields names the offending capability').toContain('capability')
    }

    // (b) seed = 2^64 (JSON number just over uint64) → 400 bad_request. The
    // JSON body fails to unmarshal into uint64 → generic bad_request envelope.
    {
      const res = await api.post(u('/simulations'), {
        data: { model_id: model.id, name: uniqueName('sim-seedof'), steps: 5, seed: 18446744073709551616 },
      })
      const { status, body } = await envelope(res)
      expect(status).toBe(400)
      expect(body?.code).toBe('bad_request')
    }

    // (c) seed = -1 (negative into an unsigned field) → 400 bad_request.
    {
      const res = await api.post(u('/simulations'), {
        data: { model_id: model.id, name: uniqueName('sim-seedneg'), steps: 5, seed: -1 },
      })
      const { status, body } = await envelope(res)
      expect(status).toBe(400)
      expect(body?.code).toBe('bad_request')
    }
  })

  // -------------------------------------------------------------------------
  // ERR-05 — WebSocket ticket auth
  // -------------------------------------------------------------------------
  test('ERR-05 WS ticket auth: no-ticket / garbage / single-use / finished-sim', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page) // page loaded on :3000 → correct WS Origin

    const sim = await seedReadySim(api, model.id, { steps: 5000, seed: 5, name: uniqueName('sim-err-ws') })
    simIds.push(sim.id)
    runningSimIds.push(sim.id)
    await apiJson(api, 'post', `/simulations/${sim.id}/run`, {}, 202)
    await pollRunning(api, sim.id)

    // (a) No ticket → rejected (never opens).
    {
      const r = await wsProbe(page, `ws://localhost:8080/api/v1/intellisim/simulations/${sim.id}/ws`)
      console.log(`[ERR-05a no-ticket] ${JSON.stringify(r)}`)
      expect(r.opened, 'no-ticket WS must not open').toBe(false)
    }

    // (b) Garbage ticket → rejected.
    {
      const r = await wsProbe(page, wsUrlFor(sim.id, 'deadbeef-not-a-real-ticket'))
      console.log(`[ERR-05b garbage] ${JSON.stringify(r)}`)
      expect(r.opened, 'garbage-ticket WS must not open').toBe(false)
    }

    // (c) Valid single-use ticket: field name is `ticket`. First connect OPENS;
    // a second connect with the SAME ticket is rejected (single-use consumed).
    {
      const issued = await apiJson<{ ticket: string; expires_at?: string }>(
        api,
        'post',
        `/simulations/${sim.id}/stream-ticket`,
        {},
        200,
      )
      expect(typeof issued.ticket, 'stream-ticket returns {ticket}').toBe('string')
      const { first, second } = await wsSingleUseProbe(page, wsUrlFor(sim.id, issued.ticket))
      console.log(`[ERR-05c single-use] first=${JSON.stringify(first)} second=${JSON.stringify(second)}`)
      expect(first.opened, 'first connect with a fresh ticket opens').toBe(true)
      expect(second.opened, 'reused ticket must be rejected').toBe(false)
    }

    await stopQuietly(api, sim.id)
    await pollUntil(
      async () => {
        const s = await apiJson<{ status: string }>(api, 'get', `/simulations/${sim.id}`)
        return ['stopped', 'finished', 'failed'].includes(s.status) ? s.status : null
      },
      { timeoutMs: 20_000, label: 'ws sim stopped' },
    )

    // (d) Ticket for a FINISHED sim is rejected at ISSUE time → 409 not_streamable
    // (a terminal sim has no live run to stream). Run a tiny sim to completion.
    {
      const done = await seedReadySim(api, model.id, { steps: 3, seed: 6, name: uniqueName('sim-err-wsdone') })
      simIds.push(done.id)
      await runToCompletion(api, done.id, { expect: 'finished' })
      const res = await api.post(u(`/simulations/${done.id}/stream-ticket`), { data: {} })
      const { status, body } = await envelope(res)
      expect(status).toBe(409)
      expect(body?.code).toBe('not_streamable')
    }
  })

  // -------------------------------------------------------------------------
  // ERR-06 — UI surfacing spot-checks (route-forced responses)
  // -------------------------------------------------------------------------
  test('ERR-06 sim editor surfaces a forced 422 as an inline validation banner', async ({ page }) => {
    test.setTimeout(60_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 11, name: uniqueName('sim-err-422ui') })
    simIds.push(sim.id)

    await login(page)
    // Intercept ONLY the PUT save; the GET on open must hit the real backend.
    const simUrl = `**/api/v1/intellisim/simulations/${sim.id}`
    await page.route(simUrl, async (route) => {
      if (route.request().method() !== 'PUT') return route.fallback()
      await route.fulfill({
        status: 422,
        contentType: 'application/json',
        body: JSON.stringify({
          code: 'validation_failed',
          message: 'request validation failed',
          fields: { steps: 'min=1' },
        }),
      })
    })

    await page.goto(`/simulations/${sim.id}`)
    const nameInput = page.getByPlaceholder('Untitled simulation')
    await expect(nameInput).toHaveValue(sim.name, { timeout: 15_000 })
    // Make the editor dirty, then Save → forced 422.
    await nameInput.fill(`${sim.name}-x`)
    await page.getByRole('button', { name: 'Save', exact: true }).click()

    // APPROACH THAT WORKED: route interception. The inline "Save failed —
    // validation:" Message renders the field errors from ApiError.fields.
    await expect(page.getByText('Save failed — validation:')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('steps: min=1')).toBeVisible()
    await page.unroute(simUrl)
  })

  test('ERR-06 sim editor surfaces a forced 409 not_editable as a toast (not the inline banner)', async ({ page }) => {
    test.setTimeout(60_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 12, name: uniqueName('sim-err-409ui') })
    simIds.push(sim.id)

    await login(page)
    const simUrl = `**/api/v1/intellisim/simulations/${sim.id}`
    await page.route(simUrl, async (route) => {
      if (route.request().method() !== 'PUT') return route.fallback()
      await route.fulfill({
        status: 409,
        contentType: 'application/json',
        body: JSON.stringify({ code: 'not_editable', message: 'the simulation is not in an editable state' }),
      })
    })

    await page.goto(`/simulations/${sim.id}`)
    const nameInput = page.getByPlaceholder('Untitled simulation')
    await expect(nameInput).toHaveValue(sim.name, { timeout: 15_000 })
    await nameInput.fill(`${sim.name}-y`)
    await page.getByRole('button', { name: 'Save', exact: true }).click()

    // ACTUAL UX: a 409 (non-400/422) is routed to the global toast handler with
    // summary "Save Simulation" — the inline validation banner does NOT appear.
    await expect(page.locator('.p-toast').getByText('Save Simulation')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Save failed — validation:')).toHaveCount(0)
    await page.unroute(simUrl)
  })

  // -------------------------------------------------------------------------
  // ERR-07 — SSE deletion (simulation.deleted / model.deleted)
  // -------------------------------------------------------------------------
  test('ERR-07 sim editor reacts to simulation.deleted (SSE) with a not-found empty state', async ({ page }) => {
    test.setTimeout(60_000)
    const sim = await seedReadySim(api, model.id, { steps: 10, seed: 13, name: uniqueName('sim-err-del') })

    await login(page)
    // Wait for the SSE subscription (GET /simulations/:id/events) to establish —
    // SSE has no replay, so a delete fired before the stream is open is missed.
    const sseReady = page.waitForResponse(
      (r) => r.url().includes(`/simulations/${sim.id}/events`),
      { timeout: 15_000 },
    )
    await page.goto(`/simulations/${sim.id}`)
    await expect(page.getByPlaceholder('Untitled simulation')).toHaveValue(sim.name, { timeout: 15_000 })
    await sseReady.catch(() => null)

    // Delete via API → SSE `simulation.deleted` clears store.current → the editor
    // swaps to the "Simulation not found / may have been deleted" empty state.
    await apiJson(api, 'delete', `/simulations/${sim.id}`)
    await expect(page.getByText('Simulation not found')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText('This simulation may have been deleted.')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Back to simulations' })).toBeVisible()
  })

  test('ERR-07 model editor reacts to model.deleted (SSE) with a not-found empty state', async ({ page }) => {
    test.setTimeout(60_000)
    const mdl = await seedModel(api, minimalSpatial(uniqueName('mdl-err-del')))

    await login(page)
    // Wait for the model chat SSE (GET /models/:id/chat/events) to establish.
    const sseReady = page.waitForResponse(
      (r) => r.url().includes(`/models/${mdl.id}/chat/events`),
      { timeout: 15_000 },
    )
    await page.goto(`/models/${mdl.id}`)
    // Wait for the editor to load (the model name renders as top-bar text).
    await expect(page.getByText(mdl.name).first()).toBeVisible({ timeout: 15_000 })
    await sseReady.catch(() => null)

    await apiJson(api, 'delete', `/models/${mdl.id}`)
    await expect(page.getByText('Model not found')).toBeVisible({ timeout: 15_000 })
    await expect(page.getByText('This model may have been deleted.')).toBeVisible()
  })
})

// ===========================================================================
// OBSERVED ERROR MATRIX (endpoint → status + code) — 2026-07-14, live stack
// (post error-normalization fixes: validation_failed is 422 everywhere).
//
// ERR-01 (404 / 405 routing)
//   GET/PUT/DELETE /models/<bogus24>                 → 404 not_found
//   GET  /simulations/<bogus24>                      → 404 not_found
//   GET  /simulations/<bogus>/analyses/<bogus>       → 404 not_found (sim wins)
//   GET  /simulations/<bogus>/analyses               → 404 not_found
//   GET  /simulations/<bogus>/model-snapshot         → 404 not_found (sim wins)
//   GET  /simulations/<realSim>/agents/999999        → 404 agent_not_found
//   GET  /models/<non-hex>  (malformed id)           → 404 not_found (graceful)
//   GET  /definitely-not-a-route (unknown route)     → 404 not_found
//   PATCH /models  (wrong method on a real path)     → 405 method_not_allowed  (was 500)
//   PATCH /models/<id>  (wrong method on param path) → 405 method_not_allowed
//
// ERR-02 (409 family)
//   POST /simulations/<ready>/control {pause}        → 409 not_running
//   POST /simulations/<ready>/stream-ticket          → 409 not_streamable
//   PUT  /simulations/<running> (full body)          → 409 not_editable
//   POST /simulations/<running>/control {resume}     → 409 not_running   (resume-while-running REJECTED)
//   POST /simulations/<running>/run (2nd)            → 409 run_in_flight (in-flight guard precedes state check)
//   POST /simulations/<running>/run race (2×)        → one 202, loser 409 run_in_flight
//   POST /generate ×2 race                           → one 202, other 409 generation_in_progress OR 202
//
// ERR-03 (validation family — all 422 now)
//   POST /simulations {steps:0}                      → 422 validation_failed {steps: min=1}
//   POST /simulations/<ready>/control {speed_ms:20000}→ 422 validation_failed {speed_ms: max=10000}
//                                                        (validation precedes state check)
//   POST /generate {scope.parts:[bogus]}             → 422 validation_failed {scope.parts[0]: oneof=...}
//   POST /simulations from draft model               → 422 model_not_ready
//   PUT  /simulations agents:[]                       → 200 (accepted); then POST /run → 422 not_runnable
//   PUT  /models {pie_chart + sim_metric}            → 422 validation_failed {widgets[0].source.type: "...not allowed"}
//
// ERR-04 (400 bad_request family)
//   GET  /llms?capability=bogus                       → 422 validation_failed {capability: "unknown capability ..."}
//                                                        (now the standard envelope; was 400 {error})
//   POST /simulations {seed: 2^64}                    → 400 bad_request
//   POST /simulations {seed: -1}                      → 400 bad_request
//
// ERR-05 (WS ticket auth)  — stream-ticket response field = `ticket` (+ expires_at)
//   no ticket / garbage ticket                        → WS never opens (rejected)
//   fresh ticket                                      → WS opens; reused ticket → rejected (single-use)
//   POST /stream-ticket on a FINISHED sim             → 409 not_streamable (rejected at issue time)
//
// ERR-06 (UI surfacing)
//   forced 422 on sim save                            → inline "Save failed — validation:" Message (route-interception)
//   forced 409 not_editable on sim save               → global toast "Save Simulation" (NOT the inline banner)
//
// ERR-07 (SSE deletion)
//   simulation.deleted while editor open              → "Simulation not found / may have been deleted" empty state
//   model.deleted while editor open                   → "Model not found / may have been deleted" empty state
// ===========================================================================
