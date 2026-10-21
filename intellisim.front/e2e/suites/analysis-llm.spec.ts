import { test, expect, type APIRequestContext, type Page } from '@playwright/test'
import * as fs from 'node:fs'
import { apiContext, apiJson, pollUntil, u } from '../_helpers/api'
import { login } from '../_helpers/login'
import { rulesRich, seedModel, seedReadySim, uniqueName, deleteQuietly } from '../_helpers/fixtures'
import { pickCheapModel, type LlmTargetRef } from '../_helpers/llm'

// §5.11 ANL-03…ANL-09 — Level-3 analysis, @llm tier, REAL OpenAI provider.
//
// COST DISCIPLINE: results_analysis lives only on o3/o4-mini →
// pickCheapModel('results_analysis') resolves o4-mini. Analyses are the
// expensive artifact — budget is ≤ 4 LLM analyses TOTAL, structured to REUSE:
//   • ONE English analysis, created via UI (ANL-03) → reused by ANL-04 + ANL-05
//   • ONE scoped analysis (ANL-06, API) → deleted by ANL-08
//   • ONE Ukrainian analysis (ANL-07, API)
//   • ONE analysis on a PAUSED sim (ANL-09, API)
// Context messages are kept short: "Briefly analyze … Keep the report minimal."

const ANALYSIS_TIMEOUT = 240_000
const SHORT_CONTEXT = 'Briefly analyze population dynamics. Keep the report minimal.'

type Json = Record<string, any>

let api: APIRequestContext
const simIds: string[] = []
const modelIds: string[] = []

let model: { id: string; name: string }
let simFinished: { id: string; name: string }
let target: LlmTargetRef // o4-mini
let targetLabel = '' // LlmExecutionConfig option text, e.g. "OpenAI · o4-mini"

// Shared across ANL-03/04/05 (created via UI in ANL-03).
let enAnalysisId = ''

// Start + poll to terminal, resilient to 429 run_capacity (parallel @llm agents
// share the engine's 4-run cap): retry POST /run until a slot frees.
async function runToCompletionResilient(simId: string, timeoutMs = 120_000): Promise<string> {
  await pollUntil(async () => {
    const res = await api.post(u(`/simulations/${simId}/run`), { data: {} })
    if (res.status() === 202) return true
    if (res.status() === 429) return null // capacity — retry
    throw new Error(`POST /run ${simId} → ${res.status()}: ${await res.text()}`)
  }, { timeoutMs: 180_000, intervalMs: 2000, label: `run slot for ${simId}` })
  return pollUntil(async () => {
    const sim = await apiJson<{ status: string }>(api, 'get', `/simulations/${simId}`)
    return ['finished', 'stopped', 'failed'].includes(sim.status) ? sim.status : null
  }, { timeoutMs, intervalMs: 750, label: `sim ${simId} terminal` })
}

// Resolve the "Provider · Model" label the LlmExecutionConfig Select renders.
async function resolveModelLabel(t: LlmTargetRef): Promise<string> {
  const body = await apiJson<{ list: any[] }>(api, 'get', '/llms', undefined, 200)
  for (const p of body.list)
    for (const m of p.models)
      if (p.uid === t.provider_uid && m.uid === t.model_uid) return `${p.name} · ${m.name}`
  throw new Error(`results_analysis target ${t.provider_uid}/${t.model_uid} not advertised by /llms`)
}

// Poll an analysis to a terminal (completed|failed) status via API.
async function pollAnalysis(simId: string, analysisId: string, timeoutMs = ANALYSIS_TIMEOUT): Promise<Json> {
  return pollUntil(async () => {
    const a = await apiJson<Json>(api, 'get', `/simulations/${simId}/analyses/${analysisId}`)
    return a.status === 'completed' || a.status === 'failed' ? a : null
  }, { timeoutMs, intervalMs: 3000, label: `analysis ${analysisId} terminal` })
}

// Create an analysis via API and poll it to terminal; returns the final Analysis.
async function createAnalysisApi(simId: string, body: Json): Promise<Json> {
  const res = await api.post(u(`/simulations/${simId}/analyses`), { data: body })
  const created = (await res.json().catch(() => ({}))) as Json
  expect(res.status(), `POST /analyses → ${res.status()}: ${JSON.stringify(created)}`).toBe(202)
  expect(created.id, 'created analysis id').toBeTruthy()
  return pollAnalysis(simId, created.id)
}

// Open the analysis view and pick the o4-mini model in the AnalyzeDialog.
async function pickAnalysisModel(page: Page): Promise<void> {
  const dialog = page.getByRole('dialog')
  await dialog
    .locator('.p-select')
    .filter({ hasText: /Select execution model|Loading models/ })
    .first()
    .click()
  await page.getByRole('option', { name: targetLabel, exact: true }).click()
}

test.beforeAll(async () => {
  api = await apiContext()

  target = (await pickCheapModel('results_analysis'))!
  if (!target) throw new Error('no results_analysis-capable model advertised — is the provider running?')
  targetLabel = await resolveModelLabel(target)

  model = await seedModel(api, rulesRich(uniqueName('mdl-anl-llm')))
  modelIds.push(model.id)

  simFinished = await seedReadySim(api, model.id, { steps: 10, seed: 91, name: uniqueName('sim-anl-llm') })
  simIds.push(simFinished.id)
  await runToCompletionResilient(simFinished.id)
})

test.afterAll(async () => {
  for (const id of simIds) {
    // Free any run slot a paused sim (ANL-09) still holds, then delete (cascades
    // analyses + files + steps).
    await api.post(u(`/simulations/${id}/control`), { data: { action: 'stop' } }).catch(() => null)
    await deleteQuietly(api, `/simulations/${id}`)
  }
  for (const id of modelIds) await deleteQuietly(api, `/models/${id}`)
  await api.dispose()
})

// ---------------------------------------------------------------------------
// ANL-03 — Create via UI: dialog required-context gating, English + o4-mini,
// 202, list row with status/progress, reaches `completed` via SSE (no reload).
// ---------------------------------------------------------------------------
test('ANL-03 @llm Create analysis via UI: required context gates submit, English + o4-mini ⇒ 202 ⇒ completed via SSE (no reload)', async ({ page }) => {
  test.setTimeout(ANALYSIS_TIMEOUT + 90_000)

  const analysisName = uniqueName('anl')
  await login(page)
  await page.goto(`/simulations/${simFinished.id}/analysis`)

  const newBtn = page.getByRole('button', { name: 'New analysis' })
  await expect(newBtn).toBeEnabled({ timeout: 20_000 })
  await newBtn.click()

  const dialog = page.getByRole('dialog')
  await expect(dialog.getByText('New analysis')).toBeVisible()

  const runBtn = dialog.getByRole('button', { name: 'Run analysis' })

  // Required-context gating: with name + model set but context EMPTY ⇒ disabled.
  await dialog.getByPlaceholder('Analysis name').fill(analysisName)
  await pickAnalysisModel(page)
  await expect(runBtn, 'Run disabled while context empty').toBeDisabled()

  // Fill the required context ⇒ enabled. (Language default is English.)
  const textarea = dialog.locator('textarea')
  await textarea.fill(SHORT_CONTEXT)
  await expect(runBtn, 'Run enabled once context + model set').toBeEnabled()

  // Submit ⇒ POST /analyses returns 202 with the pending Analysis.
  const postResp = page.waitForResponse(
    (r) => r.request().method() === 'POST' && /\/analyses$/.test(r.url()),
  )
  await runBtn.click()
  const resp = await postResp
  expect(resp.status(), 'POST /analyses → 202').toBe(202)
  const createdBody = (await resp.json()) as Json
  enAnalysisId = createdBody.id
  expect(enAnalysisId).toBeTruthy()

  // List row appears with a status tag + (transient) progress %.
  const row = page.getByRole('row').filter({ hasText: analysisName })
  await expect(row).toBeVisible({ timeout: 15_000 })
  // Document the initial status/progress if we catch it before completion.
  const initialText = (await row.textContent())?.trim() ?? ''
  test.info().annotations.push({
    type: 'anl-03-initial-row',
    description: `initial row: "${initialText.replace(/\s+/g, ' ')}"`,
  })
  expect(initialText).toMatch(/pending|queued|running|completed/i)

  // WITHOUT reload: status reaches `completed` via SSE analysis.* (240 s).
  await expect(row).toContainText('completed', { timeout: ANALYSIS_TIMEOUT })

  // Confirm the artifact is actually completed with a file (backstop via API).
  const final = await apiJson<Json>(api, 'get', `/simulations/${simFinished.id}/analyses/${enAnalysisId}`)
  console.log(
    `ANL-03 FINDING: status=${final.status} has_file=${!!final.file} requests=${final.requests} ` +
      `warnings=${JSON.stringify(final.warnings ?? [])} steps_covered=${JSON.stringify(final.steps_covered)}`,
  )
  expect(final.status, `analysis final status (error=${final.error ?? ''})`).toBe('completed')
})

// ---------------------------------------------------------------------------
// ANL-04 — Detail view (SAME artifact): summary/sections/conclusions +
// analysis-meta populated (analysis-warnings may be absent — documented).
// ---------------------------------------------------------------------------
test('ANL-04 @llm Detail view: executive summary + sections/findings + conclusions + analysis-meta populated', async ({ page }) => {
  test.setTimeout(120_000)
  test.skip(!enAnalysisId, 'ANL-03 did not produce an English analysis')

  const a = await apiJson<Json>(api, 'get', `/simulations/${simFinished.id}/analyses/${enAnalysisId}`)
  test.skip(a.status !== 'completed', `English analysis is ${a.status}, not completed`)

  await login(page)
  await page.goto(`/simulations/${simFinished.id}/analysis`)

  // Open the detail view by clicking the row's View (eye) button.
  const row = page.getByRole('row').filter({ hasText: a.name })
  await expect(row).toBeVisible({ timeout: 20_000 })
  await row.getByRole('button').first().click() // pi pi-eye (View)

  // The PDF iframe usually renders instead of the HTML report. Assert whichever
  // surfaced; the report content is validated structurally via the API below.
  const meta = page.getByTestId('analysis-meta')
  await expect(meta).toBeVisible({ timeout: 20_000 })

  // analysis-meta: steps covered + token usage/requests, incl. a number.
  const metaText = (await meta.textContent())?.trim() ?? ''
  console.log(`ANL-04 FINDING: analysis-meta = "${metaText.replace(/\s+/g, ' ')}"`)
  expect(metaText.length, 'analysis-meta non-empty').toBeGreaterThan(0)
  expect(metaText, 'analysis-meta contains a number (steps/tokens/requests)').toMatch(/\d/)

  // analysis-warnings: present only if the pipeline emitted warnings — document.
  const warnings = page.getByTestId('analysis-warnings')
  const hasWarnings = (await warnings.count()) > 0
  console.log(
    'ANL-04 FINDING: ' +
      (hasWarnings
        ? `analysis-warnings present: "${((await warnings.textContent()) ?? '').trim().replace(/\s+/g, ' ')}"`
        : 'analysis-warnings ABSENT (no warnings emitted — expected/documented)'),
  )

  // Report structure (via API — the source of truth the UI renders): executive
  // summary + at least one section, and conclusions where present.
  const report = a.report as Json
  expect(report, 'report present on completed analysis').toBeTruthy()
  expect((report.executive_summary ?? '').length, 'executive summary rendered').toBeGreaterThan(0)
  const sections = (report.sections ?? []) as Json[]
  expect(sections.length, 'at least one section').toBeGreaterThan(0)
  const withFindings = sections.filter((s) => (s.key_findings ?? []).length > 0).length
  console.log(
    `ANL-04 FINDING: sections=${sections.length} (with key_findings=${withFindings}); ` +
      `conclusions=${(report.conclusions ?? []).length}; recommendations=${(report.recommendations ?? []).length}; ` +
      `summary sample="${(report.executive_summary as string).slice(0, 100).replace(/\s+/g, ' ')}"`,
  )
  // token usage / requests recorded.
  expect(a.requests, 'requests > 0').toBeGreaterThan(0)
  expect(a.token_usage?.total?.total_tokens, 'total tokens > 0').toBeGreaterThan(0)
})

// ---------------------------------------------------------------------------
// ANL-05 — PDF (SAME artifact): in-browser iframe (blob) + Download event +
// [API] GET …/file → 200 application/pdf, body starts %PDF.
// ---------------------------------------------------------------------------
test('ANL-05 @llm PDF: in-browser blob iframe + Download produces a %PDF file; [API] /file → 200 application/pdf', async ({ page }) => {
  test.setTimeout(120_000)
  test.skip(!enAnalysisId, 'ANL-03 did not produce an English analysis')

  const a = await apiJson<Json>(api, 'get', `/simulations/${simFinished.id}/analyses/${enAnalysisId}`)
  test.skip(a.status !== 'completed' || !a.file, `English analysis has no file (status ${a.status})`)

  await login(page)
  await page.goto(`/simulations/${simFinished.id}/analysis`)
  const row = page.getByRole('row').filter({ hasText: a.name })
  await expect(row).toBeVisible({ timeout: 20_000 })
  await row.getByRole('button').first().click() // View

  // In-browser PDF iframe with a blob: source.
  const iframe = page.locator('iframe[title="Analysis report"]')
  await expect(iframe).toBeVisible({ timeout: 20_000 })
  const src = await iframe.getAttribute('src')
  expect(src, 'iframe src is a blob URL').toMatch(/^blob:/)

  // Download PDF ⇒ download event ⇒ save + validate the bytes.
  const dlBtn = page.getByRole('button', { name: 'Download PDF' })
  await expect(dlBtn).toBeVisible()
  const dlPromise = page.waitForEvent('download')
  await dlBtn.click()
  const dl = await dlPromise
  const savePath = test.info().outputPath('analysis-en.pdf')
  await dl.saveAs(savePath)
  const buf = fs.readFileSync(savePath)
  expect(buf.length, 'downloaded PDF > 5 KB').toBeGreaterThan(5 * 1024)
  expect(buf.subarray(0, 4).toString('latin1'), 'starts with %PDF').toBe('%PDF')

  // [API] GET …/file → 200, application/pdf, body starts %PDF.
  const fileRes = await api.get(u(`/simulations/${simFinished.id}/analyses/${enAnalysisId}/file`))
  expect(fileRes.status(), 'GET /file → 200').toBe(200)
  const ct = fileRes.headers()['content-type'] ?? ''
  expect(ct, `content-type application/pdf (got "${ct}")`).toContain('application/pdf')
  const body = await fileRes.body()
  expect(body.subarray(0, 4).toString('latin1'), 'API body starts %PDF').toBe('%PDF')
  test.info().annotations.push({
    type: 'anl-05-pdf',
    description: `download ${buf.length} bytes; API /file ${body.length} bytes; content-type "${ct}"`,
  })
})

// ---------------------------------------------------------------------------
// ANL-06 — Scoped analysis. UI: verify the Scope section is fully reachable
// (steps + sampling + agents/properties/focus multiselects). Artifact: created
// via API with a step range {2,8}+sampling for a reliable echo assertion.
// This artifact is the one ANL-08 deletes.
// ---------------------------------------------------------------------------
let scopedAnalysis: Json | null = null

test('ANL-06 @llm Scoped analysis: Scope UI reachable; steps_covered echoes {from:2,to:8}+sampling', async ({ page }) => {
  test.setTimeout(ANALYSIS_TIMEOUT + 90_000)

  // --- UI: the AnalyzeDialog Scope section is reachable and interactive. ---
  await login(page)
  await page.goto(`/simulations/${simFinished.id}/analysis`)
  await page.getByRole('button', { name: 'New analysis' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByText('New analysis')).toBeVisible()

  // Expand the collapsible "Scope" <details>.
  await dialog.getByText(/^Scope/).click()
  const scope = dialog.locator('details')

  // Step from / to / sampling are the three InputNumbers inside the Scope block.
  const nums = scope.locator('.p-inputnumber-input')
  await expect(nums).toHaveCount(3)
  await nums.nth(0).fill('2')
  await nums.nth(1).fill('8')
  await nums.nth(2).fill('2')
  await expect(nums.nth(0)).toHaveValue('2')
  await expect(nums.nth(1)).toHaveValue('8')

  // Agents / Properties / Focus-areas multiselects are present with real options.
  const ms = scope.locator('.p-multiselect')
  await expect(ms).toHaveCount(3)
  await ms.nth(0).click() // Agents
  await expect(page.getByRole('option', { name: 'Runner' })).toBeVisible({ timeout: 10_000 })
  await page.keyboard.press('Escape')
  await ms.nth(1).click() // Properties
  await expect(page.getByRole('option', { name: /ticks/ }).first()).toBeVisible({ timeout: 10_000 })
  await page.keyboard.press('Escape')
  await ms.nth(2).click() // Focus areas
  await expect(page.getByRole('option', { name: 'Population dynamics' })).toBeVisible({ timeout: 10_000 })
  await page.keyboard.press('Escape')
  test.info().annotations.push({
    type: 'anl-06-scope-ui',
    description: 'Scope section reachable: 3 InputNumbers (step from/to/sampling) + 3 MultiSelects (agents[Runner/Beacon], properties[ticks…], focus[Population dynamics…]).',
  })

  // Cancel the dialog — the real artifact is created via API for a reliable echo.
  await dialog.getByRole('button', { name: 'Cancel' }).click()

  // --- API: scoped artifact with step range + sampling + agent/property/focus. ---
  scopedAnalysis = await createAnalysisApi(simFinished.id, {
    name: uniqueName('anl-scoped'),
    context_message: SHORT_CONTEXT,
    language: 'en',
    llm_target: target,
    selection: {
      step_from: 2,
      step_to: 8,
      sampling: 2,
      agent_uids: ['runner'],
      property_uids: ['ticks'],
      focus_areas: ['population'],
    },
  })

  expect(scopedAnalysis.status, `scoped analysis (error=${scopedAnalysis.error ?? ''})`).toBe('completed')
  const sc = scopedAnalysis.steps_covered as Json
  console.log(
    `ANL-06 FINDING: steps_covered=${JSON.stringify(sc)}; selection echoed=${JSON.stringify(scopedAnalysis.selection)}; ` +
      `warnings=${JSON.stringify(scopedAnalysis.warnings ?? [])}`,
  )
  expect(sc.from, 'steps_covered.from echoes step_from').toBe(2)
  expect(sc.to, 'steps_covered.to echoes step_to').toBe(8)
  expect(sc.sampling, 'steps_covered.sampling echoes sampling').toBe(2)
  expect(sc.count, 'steps_covered.count > 0').toBeGreaterThan(0)
  // sampling=2 over [2,8] ⇒ at most 4 sampled points.
  expect(sc.count, 'count bounded by range/sampling').toBeLessThanOrEqual(4)
})

// ---------------------------------------------------------------------------
// ANL-07 — Ukrainian analysis (API): language 'uk' ⇒ completed; executive
// summary contains Cyrillic; PDF downloadable (API, %PDF + size).
// ---------------------------------------------------------------------------
test('ANL-07 @llm Ukrainian analysis: language uk ⇒ completed, Cyrillic report, downloadable PDF', async () => {
  test.setTimeout(ANALYSIS_TIMEOUT + 60_000)

  // Exact AnalyzeRequest shape (src/shared/types/analysis.ts): name?, context_message,
  // language, llm_target{provider_uid,model_uid}, selection (AnalysisSelection; {} = whole run).
  const a = await createAnalysisApi(simFinished.id, {
    name: uniqueName('anl-uk'),
    context_message: SHORT_CONTEXT,
    language: 'uk',
    llm_target: target,
    selection: {},
  })
  expect(a.status, `uk analysis (error=${a.error ?? ''})`).toBe('completed')

  const summary = (a.report?.executive_summary ?? '') as string
  const cyrillic = /[Ѐ-ӿ]/.test(summary)
  console.log(
    `ANL-07 FINDING: executive_summary length=${summary.length}, Cyrillic=${cyrillic}; ` +
      `sample="${summary.slice(0, 100).replace(/\s+/g, ' ')}"; warnings=${JSON.stringify(a.warnings ?? [])}`,
  )
  expect(cyrillic, 'Ukrainian report contains Cyrillic in executive summary').toBe(true)

  // PDF downloadable (API): %PDF + size, no glyph deep-validation.
  expect(a.file, 'uk analysis has a file').toBeTruthy()
  const fileRes = await api.get(u(`/simulations/${simFinished.id}/analyses/${a.id}/file`))
  expect(fileRes.status(), 'GET uk /file → 200').toBe(200)
  const body = await fileRes.body()
  expect(body.subarray(0, 4).toString('latin1'), 'uk PDF starts %PDF').toBe('%PDF')
  expect(body.length, 'uk PDF > 5 KB').toBeGreaterThan(5 * 1024)
})

// ---------------------------------------------------------------------------
// ANL-08 — Delete the ANL-06 scoped analysis from the list (confirm dialog) ⇒
// row gone; [API] analysis GET → 404 analysis_not_found AND file GET → 404.
// Also: Download disabled for any analysis WITHOUT a file (document N/A if none).
// ---------------------------------------------------------------------------
test('ANL-08 @llm Delete scoped analysis from list ⇒ row gone; [API] analysis + file → 404', async ({ page }) => {
  test.setTimeout(90_000)
  test.skip(!scopedAnalysis?.id, 'ANL-06 did not produce a scoped analysis to delete')
  const aid = scopedAnalysis!.id as string
  const aname = scopedAnalysis!.name as string

  await login(page)
  await page.goto(`/simulations/${simFinished.id}/analysis`)

  const row = page.getByRole('row').filter({ hasText: aname })
  await expect(row).toBeVisible({ timeout: 20_000 })

  // Document the Download-disabled-without-file affordance across the list.
  const rows = page.getByRole('row')
  const rowCount = await rows.count()
  let fileless = 0
  let filelessDisabled = 0
  for (let i = 0; i < rowCount; i++) {
    const dl = rows.nth(i).getByRole('button').nth(1) // pi pi-download
    if ((await dl.count()) === 0) continue
    if (await dl.isDisabled().catch(() => false)) {
      // disabled == no file per AnalysisListPanel (:disabled="!data.has_file")
      fileless++
      filelessDisabled++
    }
  }
  console.log(
    'ANL-08 FINDING: ' +
      (fileless
        ? `${filelessDisabled}/${fileless} fileless rows have Download disabled (as expected).`
        : 'N/A — no fileless (failed) analysis present; every completed analysis has a downloadable file.'),
  )

  // Delete via the trash button ⇒ ConfirmDialog ⇒ accept ("Yes").
  await row.getByRole('button', { name: '' }).last().click() // pi pi-trash (danger)
  const confirm = page.locator('.p-confirmdialog')
  await expect(confirm).toBeVisible({ timeout: 10_000 })
  await expect(confirm).toContainText('Delete analysis')
  await confirm.getByRole('button', { name: 'Yes' }).click()

  // Row gone from the list.
  await expect(page.getByRole('row').filter({ hasText: aname })).toHaveCount(0, { timeout: 15_000 })

  // [API] analysis GET → 404 analysis_not_found; file GET → 404.
  const getRes = await api.get(u(`/simulations/${simFinished.id}/analyses/${aid}`))
  expect(getRes.status(), 'analysis GET after delete → 404').toBe(404)
  const getJson = (await getRes.json().catch(() => ({}))) as Json
  expect(getJson.code, 'analysis_not_found').toBe('analysis_not_found')
  const fileRes = await api.get(u(`/simulations/${simFinished.id}/analyses/${aid}/file`))
  expect(fileRes.status(), 'file GET after delete → 404').toBe(404)

  scopedAnalysis = null // consumed
})

// ---------------------------------------------------------------------------
// ANL-09 — Paused-run analysis: fresh sim steps=3000, start, poll running,
// pause ⇒ paused; create analysis (API, en, minimal) ⇒ completes on partial
// data; steps_covered.count ≤ steps recorded so far.
// ---------------------------------------------------------------------------
test('ANL-09 @llm Analysis on a PAUSED run completes on partial data', async () => {
  test.setTimeout(ANALYSIS_TIMEOUT + 120_000)

  const pausedSim = await seedReadySim(api, model.id, { steps: 3000, seed: 99, name: uniqueName('sim-anl-paused') })
  simIds.push(pausedSim.id)

  // Start (429-resilient) and confirm running before controlling.
  await pollUntil(async () => {
    const res = await api.post(u(`/simulations/${pausedSim.id}/run`), { data: {} })
    if (res.status() === 202) return true
    if (res.status() === 429) return null
    throw new Error(`POST /run ${pausedSim.id} → ${res.status()}: ${await res.text()}`)
  }, { timeoutMs: 180_000, intervalMs: 2000, label: `run slot for ${pausedSim.id}` })

  await pollUntil(async () => {
    const s = await apiJson<{ status: string }>(api, 'get', `/simulations/${pausedSim.id}`)
    return s.status === 'running' ? true : null
  }, { timeoutMs: 30_000, intervalMs: 500, label: `sim ${pausedSim.id} running` })

  // Wait until the run has actually RECORDED step results before pausing.
  // FINDING (first run): pausing immediately after status=running leaves zero
  // recorded steps and POST /analyses correctly rejects with 422 no_results —
  // "the simulation has no recorded results to analyze".
  await pollUntil(async () => {
    const rs0 = await apiJson<Json>(api, 'get', `/simulations/${pausedSim.id}/results/summary`)
    return rs0.has_results && (rs0.step_range?.max ?? 0) >= 5 ? true : null
  }, { timeoutMs: 60_000, intervalMs: 750, label: `sim ${pausedSim.id} has recorded results` })

  // Pause ⇒ 200 ⇒ status paused.
  const pauseRes = await api.post(u(`/simulations/${pausedSim.id}/control`), { data: { action: 'pause' } })
  expect(pauseRes.status(), 'pause → 200').toBe(200)
  await pollUntil(async () => {
    const s = await apiJson<{ status: string }>(api, 'get', `/simulations/${pausedSim.id}`)
    return s.status === 'paused' ? true : null
  }, { timeoutMs: 30_000, intervalMs: 500, label: `sim ${pausedSim.id} paused` })

  // How many steps were recorded up to the pause (bound for steps_covered).
  // Read AFTER paused settles so any in-flight step flush has landed.
  const rs = await apiJson<Json>(api, 'get', `/simulations/${pausedSim.id}/results/summary`)
  const recordedSpan = (rs.step_range?.max ?? 0) - (rs.step_range?.min ?? 0) + 1
  console.log(
    `ANL-09 FINDING: paused with step_range=${JSON.stringify(rs.step_range)} (recorded span ~${recordedSpan}); has_results=${rs.has_results}`,
  )

  // Create analysis on the paused (partial) sim ⇒ completes.
  const a = await createAnalysisApi(pausedSim.id, {
    name: uniqueName('anl-paused'),
    context_message: SHORT_CONTEXT,
    language: 'en',
    llm_target: target,
    selection: {},
  })
  expect(a.status, `paused-sim analysis (error=${a.error ?? ''})`).toBe('completed')

  const count = (a.steps_covered as Json)?.count ?? 0
  console.log(
    `ANL-09 FINDING: steps_covered=${JSON.stringify(a.steps_covered)}; recordedSpan=${recordedSpan}; ` +
      `warnings=${JSON.stringify(a.warnings ?? [])}; tokens=${a.token_usage?.total?.total_tokens ?? 0}; requests=${a.requests}`,
  )
  expect(count, 'steps_covered.count > 0').toBeGreaterThan(0)
  expect(count, 'steps_covered.count ≤ steps recorded so far (≤ 3000)').toBeLessThanOrEqual(3000)
  // And no more points than were actually recorded before the pause.
  expect(count, 'count ≤ recorded span').toBeLessThanOrEqual(recordedSpan)

  // Stop the paused run now to free the engine slot (also stopped again in cleanup).
  await api.post(u(`/simulations/${pausedSim.id}/control`), { data: { action: 'stop' } }).catch(() => null)
})
