import { test, expect, type Page } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E for the multi-image environment-generation feature. Backend contracts +
// a LIVE multi-image vision generation (gpt-4o) are exercised via the API (the
// same contract the UI calls); the editor UI is exercised for rendering + the
// add-dialog + delete paths. The over-limit (too_many_images_for_model) path is
// covered by the Go unit test TestService_GenerateEnvironment_OverModelMax — it
// is not reachable live because both vision models advertise max_images=5 and the
// library cap is also 5 (attached can never exceed the model max).

const API = '/api/v1/intellisim'
const VISION = { provider_uid: 'openai', model_uid: 'gpt-4o' }

// Draw a small distinct "layout" PNG in the browser and return raw base64.
async function makeLayoutPng(page: Page, hue: number): Promise<string> {
  return page.evaluate((h) => {
    const c = document.createElement('canvas')
    c.width = 64
    c.height = 64
    const ctx = c.getContext('2d')!
    ctx.fillStyle = '#eeeeee'
    ctx.fillRect(0, 0, 64, 64)
    ctx.fillStyle = `hsl(${h},70%,45%)`
    ctx.fillRect(6, 6, 22, 22)
    ctx.fillStyle = `hsl(${(h + 140) % 360},70%,45%)`
    ctx.fillRect(36, 36, 22, 22)
    return c.toDataURL('image/png').split(',')[1]
  }, hue)
}

// Re-tiered @llm 2026-07-12: this suite performs a LIVE gpt-4o vision
// generation and must not run in the deterministic core tier.
test.describe('Multi-image environment generation @llm', () => {
  test('library contracts + live multi-image generation + PUT no-clobber + no_images', async ({
    page,
  }) => {
    test.setTimeout(180_000)
    await login(page)
    const token = await getToken(page)
    const auth = { Authorization: `Bearer ${token}` }
    const png = await makeLayoutPng(page, 30)

    // spatial model with a vision-capable target
    const created = await (
      await page.request.post(`${API}/models`, {
        headers: auth,
        data: { name: 'E2E MultiImage', is_spatial: true, llm_target: VISION },
      })
    ).json()
    const id: string = created.id

    // add 5 labelled images
    let last: any
    for (let i = 0; i < 5; i++) {
      const r = await page.request.post(`${API}/models/${id}/images`, {
        headers: auth,
        data: {
          name: `layer ${i + 1}`,
          description: i === 0 ? 'base map' : `overlay ${i}`,
          mime_type: 'image/png',
          data: png,
        },
      })
      expect(r.status(), `add image ${i}`).toBe(201)
      last = await r.json()
    }
    expect(last.model.images.length).toBe(5)
    expect(last.model.images[0].uid, 'image gets a UID').toBeTruthy()
    expect(last.model.images[0].description).toBe('base map')

    // 6th add → 422 too_many_images (library cap)
    const sixth = await page.request.post(`${API}/models/${id}/images`, {
      headers: auth,
      data: { mime_type: 'image/png', data: png },
    })
    expect(sixth.status()).toBe(422)
    expect((await sixth.json()).code).toBe('too_many_images')

    // delete one → 200 + 4 remaining
    const delUid: string = last.model.images[2].uid
    const del = await page.request.delete(`${API}/models/${id}/images/${delUid}`, { headers: auth })
    expect(del.status()).toBe(200)
    expect((await del.json()).model.images.length).toBe(4)

    // PUT no-clobber: a full-model PUT carrying images:[] must NOT wipe the library.
    const beforePut = await (await page.request.get(`${API}/models/${id}`, { headers: auth })).json()
    const put = await page.request.put(`${API}/models/${id}`, {
      headers: auth,
      data: { model: { ...beforePut.model, images: [] } },
    })
    expect(put.ok(), `PUT status ${put.status()}`).toBeTruthy()
    expect((await put.json()).model.images.length, 'PUT must preserve the image library').toBe(4)

    // LIVE: generate the environment from the 4 stored images (gpt-4o vision).
    // Capture the env before generation so we assert it actually CHANGED.
    const preGen = await (await page.request.get(`${API}/models/${id}`, { headers: auth })).json()
    const beforeEnv = JSON.stringify(preGen.model.environment ?? null)
    const gen = await page.request.post(`${API}/models/${id}/environment/generate`, {
      headers: auth,
      data: { llm_target: VISION, prompt: 'Treat colored blocks as obstacles; build a small grid.' },
    })
    expect(gen.status(), `generate (202 expected): ${gen.status()}`).toBe(202)

    let generated = false
    for (let i = 0; i < 70 && !generated; i++) {
      await page.waitForTimeout(2000)
      const m = await (await page.request.get(`${API}/models/${id}`, { headers: auth })).json()
      const env = m.model.environment
      const changed = JSON.stringify(env ?? null) !== beforeEnv
      if (changed && env && (Number(env.width) > 0 || (env.shapes?.length ?? 0) > 0)) generated = true
    }
    expect(generated, 'multi-image vision generation should populate the environment').toBeTruthy()

    // no_images: a fresh spatial model with an empty library → 422 no_images
    const empty = await (
      await page.request.post(`${API}/models`, {
        headers: auth,
        data: { name: 'E2E NoImages', is_spatial: true, llm_target: VISION },
      })
    ).json()
    const genEmpty = await page.request.post(`${API}/models/${empty.id}/environment/generate`, {
      headers: auth,
      data: {},
    })
    expect(genEmpty.status()).toBe(422)
    expect((await genEmpty.json()).code).toBe('no_images')
  })

  test('editor UI: library renders, add via dialog, delete via thumbnail', async ({ page }) => {
    test.setTimeout(120_000)
    await login(page)
    const token = await getToken(page)
    const auth = { Authorization: `Bearer ${token}` }
    const png = await makeLayoutPng(page, 210)

    const created = await (
      await page.request.post(`${API}/models`, {
        headers: auth,
        data: { name: 'E2E ImgUI', is_spatial: true, llm_target: VISION },
      })
    ).json()
    // seed two images via API
    for (const [name, desc] of [
      ['floor', 'floor plan'],
      ['walls', 'obstacles'],
    ]) {
      await page.request.post(`${API}/models/${created.id}/images`, {
        headers: auth,
        data: { name, description: desc, mime_type: 'image/png', data: png },
      })
    }

    await page.goto(`/models/${created.id}`)
    // open the Environment section (Tabs redesign 2026-06-30: role=tab, not the old accordion button)
    await page.getByRole('tab', { name: /environment/i }).click()

    const count = page.locator('[data-testid="image-library-count"]')
    await expect(count).toContainText('2/5', { timeout: 20_000 })
    await expect(page.locator('[data-testid="generate-environment"]')).toBeAttached()

    // add a third image through the dialog (real file upload)
    await page.locator('[data-testid="add-image-tile"]').click()
    const dialog = page.getByRole('dialog')
    await dialog
      .locator('input[type=file]')
      .setInputFiles({ name: 'zones.png', mimeType: 'image/png', buffer: Buffer.from(png, 'base64') })
    await dialog.locator('input:not([type=file])').first().fill('zones')
    await dialog.locator('textarea').first().fill('zone overlay')
    await page.locator('[data-testid="confirm-add-image"]').click()
    await expect(count).toContainText('3/5', { timeout: 20_000 })

    // delete one image via its thumbnail × button → back to 2/5
    await page.locator('[data-testid="remove-image"]').first().click()
    await expect(count).toContainText('2/5', { timeout: 20_000 })
  })
})
