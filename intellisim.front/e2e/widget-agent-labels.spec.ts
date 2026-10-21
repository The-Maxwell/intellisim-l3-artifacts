import { test, expect, type Page, type APIResponse } from '@playwright/test'
import { login, getToken } from './_helpers/login'

// E2E coverage for the widget-editor terminology change (2026-07-05):
// the source builder refers to "Agents" (not "Archetypes") everywhere the user
// sees it — the multiselect label + placeholder, the agent_table columns hint,
// and the client-side validation messages. Data-model is unchanged (the widget
// source still keys off agent_uids); this proves the UI wording.

let token = ''
async function api(page: Page, method: 'post' | 'put', url: string, data?: unknown): Promise<APIResponse> {
  return page.request[method](url, {
    headers: { Authorization: `Bearer ${token}` },
    ...(data !== undefined ? { data } : {}),
  })
}

test('Widget editor says "Agents", never "Archetypes"', async ({ page }) => {
  test.setTimeout(120_000)
  await login(page)
  token = await getToken(page)

  // Seed a spatial model with one named agent carrying a numeric property + states,
  // so the widget source builder's Agent multiselect has a real option.
  const created = await (
    await api(page, 'post', '/api/v1/intellisim/models', {
      name: 'E2E Widget Labels',
      is_spatial: true,
      llm_target: { provider_uid: 'openai', model_uid: 'gpt-4o' },
    })
  ).json()
  const modelId: string = created.id
  const model = created.model
  model.agents = [
    {
      uid: 'wolf',
      name: 'Wolf',
      type: 'agent',
      move_type: 'movable',
      properties: [{ uid: 'energy', name: 'Energy', type: 'number', default_value: 10 }],
      states: ['hunting', 'resting'],
      initialization: { population_count: 2, count_mode: 'fixed', initial_state: 'hunting', placement: { type: 'random' } },
    },
  ]
  const put = await api(page, 'put', `/api/v1/intellisim/models/${modelId}`, { model })
  expect(put.ok(), `model PUT failed: ${put.status()} ${await put.text()}`).toBeTruthy()

  // Open the editor → Widgets tab → Add widget (defaults to a monitor with an
  // agent_property source, so the "Agents" multiselect shows immediately).
  await page.goto(`/models/${modelId}`)
  await page.getByRole('tab', { name: /widgets/i }).click()
  await page.getByRole('button', { name: /add widget/i }).click()

  const dialog = page.getByRole('dialog')
  await expect(dialog.getByText('Add widget', { exact: true })).toBeVisible({ timeout: 20_000 })

  // The multiselect label + placeholder use "Agents"; "Archetypes" appears nowhere.
  await expect(dialog.getByText('Agents', { exact: true })).toBeVisible()
  await expect(dialog.getByText('Select agents')).toBeVisible()
  await expect(page.getByText(/archetype/i)).toHaveCount(0)

  // The Agents multiselect lists the seeded agent by name.
  const agentPicker = dialog.locator('.p-multiselect').first()
  await agentPicker.click()
  await page.getByRole('option', { name: 'Wolf', exact: true }).click()
  await page.keyboard.press('Escape')

  // Switch the source to Population (valid for a monitor) → label gains "(optional filter)".
  const sourceType = dialog.locator('.p-select').first()
  await sourceType.click()
  await page.getByRole('option', { name: 'Population', exact: true }).click()
  await expect(dialog.getByText(/Agents\s*\(optional filter\)/)).toBeVisible()

  // Switch widget type to Agent table → the columns picker hint says "Pick an agent first"
  // (its source resets with no agent selected).
  await dialog.getByRole('button', { name: /Agent table/ }).click()
  await expect(dialog.getByText('Pick an agent first')).toBeVisible()

  // The validation panel mirrors the backend matrix with agent wording — an
  // agent_table with no agent selected requires "exactly one agent".
  await expect(dialog.getByText(/requires exactly one agent/)).toBeVisible()
})
