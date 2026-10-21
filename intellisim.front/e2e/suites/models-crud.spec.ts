import { test, expect, type APIRequestContext, type Page } from '@playwright/test'
import { login } from '../_helpers/login'
import { apiContext, apiJson, u } from '../_helpers/api'
import { deleteQuietly, minimalSpatial, seedModel, uniqueName } from '../_helpers/fixtures'

// MDL — Models: CRUD & lists (plan §5.3). Deterministic, no LLM generation.
// All entities are uniqueName()-prefixed; everything created here is deleted
// in afterAll (tracked in createdModelIds).

let api: APIRequestContext
const createdModelIds: string[] = []

// Shared fixtures (seeded once in beforeAll)
let readyModel: { id: string; name: string } // simulation-ready (minimalSpatial)
let draftModelId = ''
const draftModelName = uniqueName('mdl-draft')

// MDL-02 seeds: 6 names sorting low + 6 sorting high (all lowercase so byte
// order == lexicographic order; foreign rows may interleave — assertions are
// relative-order only).
const aaaNames: string[] = []
const zzzNames: string[] = []

function trackId(id: string) {
  if (id && !createdModelIds.includes(id)) createdModelIds.push(id)
}

async function createBlankModelViaApi(name: string): Promise<string> {
  const created = await apiJson<{ id: string }>(api, 'post', '/models', { name }, 201)
  trackId(created.id)
  return created.id
}

function modelIdFromUrl(url: string): string {
  const m = url.match(/\/models\/([^/?#]+)/)
  return m ? m[1] : ''
}

// Table rows of the (single) models DataTable on /models.
function tableRows(page: Page) {
  return page.locator('.p-datatable-table tbody tr')
}

async function rowByName(page: Page, name: string) {
  const row = page.getByRole('row', { name })
  await expect(row).toBeVisible({ timeout: 15_000 })
  return row
}

test.beforeAll(async () => {
  api = await apiContext()

  readyModel = await seedModel(api, minimalSpatial(uniqueName('mdl-ready')))
  trackId(readyModel.id)
  // sanity: fixture really is simulation-ready
  const full = await apiJson<any>(api, 'get', `/models/${readyModel.id}`, undefined, 200)
  expect(full.validation?.simulation_ready, JSON.stringify(full.validation?.errors)).toBe(true)

  draftModelId = await createBlankModelViaApi(draftModelName)

  // MDL-02: 12 tiny models with low/high-sorting names.
  for (let i = 0; i < 6; i++) {
    const lo = uniqueName(`aaa-pg${i}`)
    const hi = uniqueName(`zzz-pg${i}`)
    aaaNames.push(lo)
    zzzNames.push(hi)
    await createBlankModelViaApi(lo)
    await createBlankModelViaApi(hi)
  }
})

test.afterAll(async () => {
  for (const id of createdModelIds) await deleteQuietly(api, `/models/${id}`)
  await api.dispose()
})

test('MDL-01 New model SplitButton → Blank dialog → create navigates to draft editor and lists the row', async ({ page }) => {
  const name = uniqueName('mdl-blank')
  await login(page)
  await page.goto('/models')

  // SplitButton main action (default = Blank) opens the name dialog.
  await page.locator('.p-splitbutton').getByRole('button', { name: 'New model' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog.getByText('New blank model')).toBeVisible()
  await dialog.getByRole('textbox').fill(name)
  await dialog.getByRole('button', { name: 'Create' }).click()

  // Navigates to the new model's editor.
  await page.waitForURL(/\/models\/[^/?#]+$/, { timeout: 20_000 })
  const id = modelIdFromUrl(page.url())
  expect(id).toBeTruthy()
  trackId(id)
  await expect(page.getByPlaceholder('Model name')).toHaveValue(name, { timeout: 15_000 })
  // Blank model is a draft: editor readiness indicator shows "Not ready".
  await expect(page.getByText('Not ready', { exact: true })).toBeVisible()

  // Back on the list, the row exists with a "draft" status tag.
  await page.goto('/models')
  const row = await rowByName(page, name)
  await expect(row.getByText('draft', { exact: true })).toBeVisible()
})

test('MDL-02 server-side pagination + sort by name asc/desc (tolerant of foreign rows)', async ({ page }) => {
  await login(page)
  await page.goto('/models')
  await expect(tableRows(page).first()).toBeVisible({ timeout: 15_000 })

  // Paginator always visible; total is plausible (>= our 12 seeds + shared fixtures).
  const paginator = page.locator('.p-paginator')
  await expect(paginator).toBeVisible()
  const report = paginator.locator('.p-paginator-current')
  await expect(report).toContainText('Showing')
  const total = Number((await report.innerText()).match(/of\s+(\d+)/)?.[1] ?? '0')
  expect(total).toBeGreaterThanOrEqual(14)

  // Rows-per-page: 10 shows exactly 10 rows, 100 shows more than 10.
  const rppSelect = paginator.locator('.p-select')
  await rppSelect.click()
  await page.getByRole('option', { name: '10', exact: true }).click()
  await expect.poll(() => tableRows(page).count()).toBe(10)
  await rppSelect.click()
  await page.getByRole('option', { name: '100', exact: true }).click()
  await expect.poll(() => tableRows(page).count()).toBeGreaterThan(10)

  const nameHeader = page.getByRole('columnheader', { name: 'Name' })
  const visibleNames = async (): Promise<string[]> => {
    const cells = await page.locator('.p-datatable-table tbody tr td:first-child').allInnerTexts()
    return cells.map((t) => t.trim()).filter(Boolean)
  }
  // A page is "sorted" if adjacent pairs are ordered under byte order OR
  // case-insensitive order (tolerates either backend collation).
  const isSorted = (names: string[], dir: 1 | -1): boolean => {
    const ok = (x: string, y: string) => (dir === 1 ? x <= y : x >= y)
    const byte = names.every((n, i) => i === 0 || ok(names[i - 1], n))
    const ci = names.every((n, i) => i === 0 || ok(names[i - 1].toLowerCase(), n.toLowerCase()))
    return byte || ci
  }

  // Dataset-independent sort assertions (the dev DB exceeds one 100-row page,
  // so "both seeded groups visible" cannot be relied on): the header click must
  // issue a sorted server request, and the visible page must be ordered.
  const ascReq = page.waitForRequest((r) => r.url().includes('/models?') && r.url().includes('sort_order=asc'), { timeout: 10_000 })
  await nameHeader.click()
  await expect(nameHeader).toHaveAttribute('aria-sort', 'ascending')
  expect(new URL((await ascReq).url()).searchParams.get('sort_by')).toBe('name')
  await expect
    .poll(async () => isSorted(await visibleNames(), 1), { message: 'asc: visible page must be name-ordered' })
    .toBe(true)

  // Descending: order flips.
  const descReq = page.waitForRequest((r) => r.url().includes('/models?') && r.url().includes('sort_order=desc'), { timeout: 10_000 })
  await nameHeader.click()
  await expect(nameHeader).toHaveAttribute('aria-sort', 'descending')
  expect(new URL((await descReq).url()).searchParams.get('sort_by')).toBe('name')
  await expect
    .poll(async () => isSorted(await visibleNames(), -1), { message: 'desc: visible page must be name-ordered' })
    .toBe(true)
})

test('MDL-03 row click opens editor; row actions rendered, Create simulation disabled for draft', async ({ page }) => {
  await login(page)
  await page.goto('/models')

  // Draft row: all four actions rendered, Create simulation disabled.
  const draftRow = await rowByName(page, draftModelName)
  await expect(draftRow.getByRole('button', { name: 'View simulations' })).toBeVisible()
  await expect(draftRow.getByRole('button', { name: 'Create simulation' })).toBeDisabled()
  await expect(draftRow.getByRole('button', { name: 'Duplicate' })).toBeVisible()
  await expect(draftRow.getByRole('button', { name: 'Delete' })).toBeVisible()

  // Ready row: Create simulation enabled.
  const readyRow = await rowByName(page, readyModel.name)
  await expect(readyRow.getByRole('button', { name: 'Create simulation' })).toBeEnabled()

  // Row click (on the name cell, away from the action buttons) opens the editor.
  await readyRow.getByText(readyModel.name).click()
  await page.waitForURL(new RegExp(`/models/${readyModel.id}$`), { timeout: 15_000 })
  await expect(page.getByPlaceholder('Model name')).toHaveValue(readyModel.name)
})

test('MDL-04 duplicate from list: "(copy)" default, redirect to copy editor, companion chat cloned', async ({ page }) => {
  // Dedicated ready model WITH a companion chat (PUT /models/:id/chat creates
  // the chat without any LLM invocation — just sets the target).
  const src = await seedModel(api, minimalSpatial(uniqueName('mdl-dup')))
  trackId(src.id)
  const srcChat = await apiJson<{ chat_id: string }>(
    api, 'put', `/models/${src.id}/chat`,
    { llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' } }, 200,
  )
  expect(srcChat.chat_id).toBeTruthy()
  const srcBefore = await apiJson<any>(api, 'get', `/models/${src.id}`, undefined, 200)

  await login(page)
  await page.goto('/models')
  const row = await rowByName(page, src.name)
  await row.getByRole('button', { name: 'Duplicate' }).click()

  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  // Default name ends with "(copy)".
  await expect(dialog.getByRole('textbox')).toHaveValue(`${src.name} (copy)`)
  await dialog.getByRole('button', { name: 'Duplicate', exact: true }).click()

  // Redirects to the NEW copy's editor.
  await page.waitForURL(/\/models\/[^/?#]+$/, { timeout: 20_000 })
  const copyId = modelIdFromUrl(page.url())
  expect(copyId).toBeTruthy()
  expect(copyId).not.toBe(src.id)
  trackId(copyId)
  await expect(page.getByPlaceholder('Model name')).toHaveValue(`${src.name} (copy)`, { timeout: 15_000 })

  // [API] the copy has its own companion chat (cloned, distinct chat id)…
  const copyChat = await apiJson<{ chat_id: string }>(api, 'get', `/models/${copyId}/chat`, undefined, 200)
  expect(copyChat.chat_id).toBeTruthy()
  expect(copyChat.chat_id).not.toBe(srcChat.chat_id)

  // …and the original is untouched (same name, same chat).
  const srcAfter = await apiJson<any>(api, 'get', `/models/${src.id}`, undefined, 200)
  expect(srcAfter.model?.name).toBe(src.name)
  expect(srcAfter.updated_at).toBe(srcBefore.updated_at)
  const srcChatAfter = await apiJson<{ chat_id: string }>(api, 'get', `/models/${src.id}/chat`, undefined, 200)
  expect(srcChatAfter.chat_id).toBe(srcChat.chat_id)
})

test('MDL-05 delete from list via ConfirmDialog removes the row; GET → 404', async ({ page }) => {
  const name = uniqueName('mdl-del')
  const id = await createBlankModelViaApi(name)

  await login(page)
  await page.goto('/models')
  const row = await rowByName(page, name)
  await row.getByRole('button', { name: 'Delete' }).click()

  // PrimeVue ConfirmDialog: header "Delete model", accept button "Delete".
  const confirm = page.locator('.p-confirmdialog')
  await expect(confirm).toBeVisible()
  await expect(confirm).toContainText('Delete model')
  await expect(confirm).toContainText(name)
  await confirm.getByRole('button', { name: 'Delete', exact: true }).click()

  await expect(page.getByRole('row', { name })).toBeHidden({ timeout: 15_000 })
  const res = await api.get(u(`/models/${id}`))
  expect(res.status()).toBe(404)
})

test('MDL-06 "View simulations" deep-links to /simulations?model_id=<id> with the filter applied', async ({ page }) => {
  await login(page)
  await page.goto('/models')
  const row = await rowByName(page, readyModel.name)
  await row.getByRole('button', { name: 'View simulations' }).click()

  await page.waitForURL(new RegExp(`/simulations\\?model_id=${readyModel.id}`), { timeout: 15_000 })
  // The header MultiSelect reflects the applied model filter (label shows the
  // model name once the options list has loaded).
  await expect(page.locator('.p-multiselect-label')).toHaveText(readyModel.name, { timeout: 15_000 })
})

test('MDL-07 GET /models/options + create-sim dialog model dropdown gates not-ready models', async ({ page }) => {
  // [API] options endpoint returns id+name entries including the ready model.
  const opts = await apiJson<{ options: Array<{ id: string; name: string }> }>(
    api, 'get', '/models/options', undefined, 200,
  )
  const ready = opts.options.find((o) => o.id === readyModel.id)
  expect(ready, 'ready model present in /models/options').toBeTruthy()
  expect(ready!.name).toBe(readyModel.name)

  // [UI] the Create-Simulation dialog (Simulations list → "Create simulation")
  // lists the ready model enabled and the draft one disabled ("(not ready)").
  await login(page)
  await page.goto('/simulations')
  await page.getByRole('button', { name: 'Create simulation' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()

  // Open the model Select (placeholder "Select a model" once options loaded).
  // NOTE .first(): the dialog also embeds two LlmExecutionConfig pickers, each
  // with its own .p-select — the model picker is the first in DOM order.
  const select = dialog.locator('.p-select').first()
  await expect(select).toContainText('Select a model', { timeout: 15_000 })
  await select.click()

  const readyOption = page.getByRole('option', { name: readyModel.name, exact: true })
  await expect(readyOption).toBeVisible({ timeout: 15_000 })
  await expect(readyOption).not.toHaveAttribute('aria-disabled', 'true')

  const draftOption = page.getByRole('option', { name: `${draftModelName} (not ready)`, exact: true })
  await expect(draftOption).toBeVisible()
  await expect(draftOption).toHaveAttribute('aria-disabled', 'true')

  // Picking the ready model enables Create (we don't create — no cleanup churn).
  await readyOption.click()
  await expect(dialog.getByRole('button', { name: 'Create', exact: true })).toBeEnabled()
})
