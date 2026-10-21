import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the inline polygon draw canvas (2026-07-05): an agent's
// simulation body (footprint.points) AND display figure (display.figure) polygons
// are authored by CLICKING on a mini-canvas instead of typing X/Y numbers, with a
// live bounding-box size readout. Both persist as anchor-relative world points.

let token = ''
async function api(page: Page, method: 'post' | 'put', url: string, data?: unknown): Promise<APIResponse> {
  return page.request[method](url, {
    headers: { Authorization: `Bearer ${token}` },
    ...(data !== undefined ? { data } : {}),
  })
}

// Add a vertex by dispatching a native pointerdown at a fractional position on
// the canvas. Playwright's synthetic mouse.click doesn't reliably deliver
// pointerdown to the inline SVG, so we fire it directly (as real mouse input
// does). One point per call + an awaited count assertion between calls lets Vue
// flush the v-model round-trip so the next click sees the updated points.
async function clickCanvas(page: Page, canvasIndex: number, fx: number, fy: number): Promise<void> {
  await page.evaluate(
    ({ canvasIndex, fx, fy }) => {
      const canvas = document.querySelectorAll('[data-testid="polygon-draw"]')[canvasIndex]
      const svg = canvas?.querySelector('svg')
      if (!svg) throw new Error('no canvas svg')
      const r = svg.getBoundingClientRect()
      svg.dispatchEvent(
        new PointerEvent('pointerdown', {
          bubbles: true,
          cancelable: true,
          clientX: r.left + r.width * fx,
          clientY: r.top + r.height * fy,
        })
      )
    },
    { canvasIndex, fx, fy }
  )
}

async function drawPoints(page: Page, canvasIndex: number, corners: [number, number][]): Promise<void> {
  const vertices = page.locator('[data-testid="polygon-draw"]').nth(canvasIndex).locator('[data-testid="polygon-vertex"]')
  for (let i = 0; i < corners.length; i++) {
    await clickCanvas(page, canvasIndex, corners[i]![0], corners[i]![1])
    await expect(vertices).toHaveCount(i + 1)
  }
}

async function saveAndGetModel(page: Page): Promise<any> {
  const [resp] = await Promise.all([
    page.waitForResponse(
      (r) => /\/api\/v1\/intellisim\/models\/[^/]+$/.test(r.url()) && r.request().method() === 'PUT'
    ),
    page.getByRole('button', { name: 'Save', exact: true }).click(),
  ])
  expect(resp.ok(), `save PUT failed: ${resp.status()} ${await resp.text()}`).toBeTruthy()
  return resp.json()
}

test('agent polygons are drawn on a canvas (body + display figure)', async ({ page }) => {
  test.setTimeout(120_000)
  await login(page)
  token = await getToken(page)

  const created = await (
    await api(page, 'post', '/api/v1/intellisim/models', {
      name: 'E2E Polygon Draw',
      is_spatial: true,
      llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
    })
  ).json()
  const modelId: string = created.id

  await page.goto(`/models/${modelId}`)
  await page.getByRole('tab', { name: /agents/i }).click()
  await page.getByRole('button', { name: /add agent/i }).click()
  const uid = page.getByPlaceholder('wolf')
  await expect(uid).toBeVisible({ timeout: 20_000 })
  await uid.fill('blob')

  // Open Body & display (collapsed by default).
  await page.getByText('Body & display', { exact: true }).click()

  // --- Simulation body = polygon → draw 4 vertices ---
  await page.getByText('Simulation body', { exact: true }).locator('..').getByRole('combobox').click()
  await page.getByRole('option', { name: 'polygon', exact: true }).click()

  const bodyCanvas = page.locator('[data-testid="polygon-draw"]').nth(0)
  await expect(bodyCanvas).toBeVisible({ timeout: 10_000 })
  await drawPoints(page, 0, [
    [0.3, 0.3],
    [0.7, 0.3],
    [0.7, 0.7],
    [0.3, 0.7],
  ])
  await expect(bodyCanvas.locator('[data-testid="polygon-vertex"]')).toHaveCount(4)
  await expect(bodyCanvas.getByTestId('polygon-size')).toContainText('units')

  const body = await saveAndGetModel(page)
  const fp = body.model.agents[0].footprint
  expect(fp.kind).toBe('polygon')
  expect(fp.points).toHaveLength(4)

  // --- Display = Figure → draw 3 vertices on the SECOND canvas ---
  await page.getByText('Figure', { exact: true }).click()
  const figureCanvas = page.locator('[data-testid="polygon-draw"]').nth(1)
  await expect(figureCanvas).toBeVisible({ timeout: 10_000 })
  await drawPoints(page, 1, [
    [0.5, 0.25],
    [0.75, 0.7],
    [0.25, 0.7],
  ])
  await expect(figureCanvas.locator('[data-testid="polygon-vertex"]')).toHaveCount(3)

  const body2 = await saveAndGetModel(page)
  const disp = body2.model.agents[0].display
  expect(disp.kind).toBe('figure')
  expect(disp.figure).toHaveLength(3)
  // Body polygon survived the second save.
  expect(body2.model.agents[0].footprint.points).toHaveLength(4)
})
