import { test, expect, type APIRequestContext, type Page, type Locator } from '@playwright/test'
import { login } from '../_helpers/login'
import { apiContext, apiJson, u, type ApiErrorBody } from '../_helpers/api'
import { deleteQuietly, minimalSpatial, rulesRich, seedModel, uniqueName } from '../_helpers/fixtures'
import { pickCheapModel } from '../_helpers/llm'

// MDE-12…MDE-22 — Models: manual editor part B (plan §5.4). Deterministic —
// no LLM generation is ever triggered (LLM CONFIG authoring only). Everything
// created here is uniqueName()-prefixed and deleted in afterAll.
//
// Legacy coverage referenced (run separately, results in the wave report):
//   MDE-16 core → e2e/editor-autosave.spec.ts
//   MDE-19 core → e2e/llm-config-advanced.spec.ts
//   MDE-21      → e2e/multi-image-environment.spec.ts

let api: APIRequestContext
const createdModelIds: string[] = []

function trackId(id: string) {
  if (id && !createdModelIds.includes(id)) createdModelIds.push(id)
}

async function seedTracked(body: Record<string, any>): Promise<{ id: string; name: string }> {
  const seeded = await seedModel(api, body)
  trackId(seeded.id)
  return seeded
}

async function createBlank(name: string): Promise<string> {
  const created = await apiJson<{ id: string }>(api, 'post', '/models', { name }, 201)
  trackId(created.id)
  return created.id
}

async function getModel(id: string): Promise<any> {
  return apiJson<any>(api, 'get', `/models/${id}`, undefined, 200)
}

// PUT and return {status, body} without throwing (negative-path helper).
async function putModel(id: string, model: Record<string, any>): Promise<{ status: number; body: any }> {
  const res = await api.put(u(`/models/${id}`), { data: { model } })
  const text = await res.text()
  return { status: res.status(), body: text ? JSON.parse(text) : undefined }
}

async function openEditor(page: Page, id: string) {
  await login(page)
  await page.goto(`/models/${id}`)
  await expect(page.getByPlaceholder('Model name')).toBeVisible({ timeout: 20_000 })
}

function tabByName(page: Page, name: RegExp | string) {
  return page.getByRole('tab', { name })
}

// The editor's form labels are plain (not for-linked), so target the input
// inside the label's field wrapper: <div><label>X</label><Input/></div>.
function fieldInput(scope: Page | Locator, label: string): Locator {
  return scope.locator(`div:has(> label:text-is("${label}")) input`).first()
}
function fieldControl(scope: Page | Locator, label: string, control: string): Locator {
  return scope.locator(`div:has(> label:text-is("${label}")) ${control}`).first()
}

// Pick option(s) in a PrimeVue Select / MultiSelect that lives next to a label.
// Options render in a body-level overlay; scope to the LAST overlay and wait for
// it to unmount before moving on (a lingering overlay makes option names ambiguous).
async function selectOption(page: Page, scope: Page | Locator, label: string, option: string) {
  await fieldControl(scope, label, '.p-select').click()
  const overlay = page.locator('.p-select-overlay').last()
  await overlay.getByRole('option', { name: option, exact: true }).click()
  await expect(page.locator('.p-select-overlay')).toHaveCount(0)
}
async function multiSelect(page: Page, scope: Page | Locator, label: string, options: string[]) {
  await fieldControl(scope, label, '.p-multiselect').click()
  const overlay = page.locator('.p-multiselect-overlay').last()
  await expect(overlay).toBeVisible()
  for (const o of options) await overlay.getByRole('option', { name: o, exact: true }).click()
  await page.keyboard.press('Escape')
  await expect(page.locator('.p-multiselect-overlay')).toHaveCount(0)
}

async function saveEditor(page: Page) {
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(page.getByText('Model saved')).toBeVisible({ timeout: 10_000 })
  // Let the toast fade state settle; dirty flag must be gone.
  await expect(page.getByText('unsaved', { exact: true })).toHaveCount(0)
}

// ---------------------------------------------------------------------------
// MDE-12 payload: every condition type (15) and action type (14). All are
// accepted and round-tripped verbatim, but readiness validation is no longer
// enum-only — it now flags incomplete type-specific fields (missing_field /
// env_condition_unsupported). For this breadth model to stay simulation-ready,
// the fields the engine actually needs must be present: proximity/count require
// a `radius`, and a `custom` condition requires `parameters.property`. (The
// breadth conditions below carry those so the model is advisory-clean.)
// ---------------------------------------------------------------------------

const ALL_CONDITION_TYPES = [
  'global_property', 'property', 'position', 'proximity', 'step', 'composite', 'count',
  'barrier', 'state', 'distance', 'decision', 'random', 'memory', 'event', 'custom',
] as const

const ALL_ACTION_TYPES = [
  'set_property', 'modify_property', 'move', 'move_toward', 'move_away', 'interact',
  'emit_event', 'transition', 'call_llm', 'spawn_agent', 'remove_agent', 'transform',
  'teleport', 'custom',
] as const

// One condition per type, with the minimal type-appropriate fields.
const ALL_CONDITIONS: Record<string, any>[] = [
  { type: 'global_property', property_uid: 'alarm_count', operator: 'gte', value: 0 },
  { type: 'property', property_uid: 'ticks', operator: 'lt', value: 100 },
  { type: 'position', operator: 'gte', value: 0, parameters: { axis: 'x' } },
  { type: 'proximity', target_uid: 'beacon', radius: 5 },
  { type: 'step', operator: 'gte', value: 0 },
  {
    type: 'composite', logic_operator: 'AND',
    sub_conditions: [{ type: 'step', operator: 'gte', value: 0 }, { type: 'random', value: 0.5 }],
  },
  { type: 'count', target_uid: 'runner', operator: 'gte', value: 1, radius: 5 },
  { type: 'barrier', radius: 2 },
  { type: 'state', operator: 'eq', value: 'calm' },
  { type: 'distance', target_uid: 'beacon', operator: 'lte', value: 10 },
  { type: 'decision', value: 'flee' },
  { type: 'random', value: 0.5 },
  { type: 'memory', operator: 'contains', value: 'beacon', parameters: { memory_type: 'interaction' } },
  { type: 'event', target_uid: 'pulse' },
  { type: 'custom', parameters: { expr: 'ticks > 1', property: 'ticks' } },
]

// One action per type. References resolve against the rulesRich fixture
// (agents runner/beacon, event pulse, interaction i_greet).
const ALL_ACTIONS: Record<string, any>[] = [
  { type: 'set_property', property_uid: 'ticks', modifier: 'set', value: 1 },
  { type: 'modify_property', property_uid: 'ticks', modifier: 'add', value: 1 },
  { type: 'move', speed: 1 },
  { type: 'move_toward', target_uid: 'beacon', speed: 1 },
  { type: 'move_away', target_uid: 'beacon', speed: 1 },
  { type: 'interact', target_uid: 'i_greet' },
  { type: 'emit_event', event_uid: 'pulse', event_data: { k: 1 } },
  { type: 'transition', new_state: 'alert' },
  { type: 'call_llm' },
  { type: 'spawn_agent', spawn_agent_uid: 'runner', spawn_count: 1, spawn_position: { type: 'nearby', radius: 3 } },
  { type: 'remove_agent' },
  { type: 'transform', spawn_agent_uid: 'beacon', spawn_position: { type: 'same' } },
  { type: 'teleport', parameters: { x: 1, y: 2 } },
  { type: 'custom', parameters: { set_properties: { ticks: 0 } } },
]

test.beforeAll(async () => {
  api = await apiContext()
})

test.afterAll(async () => {
  for (const id of createdModelIds) await deleteQuietly(api, `/models/${id}`)
  await api.dispose()
})

// ---------------------------------------------------------------------------
// MDE-12 — rule-engine breadth: all 15 condition types + all 14 action types
// round-trip through PUT /models/:id, and the Rules tab renders them.
// ---------------------------------------------------------------------------
test('MDE-12 all 15 condition types + all 14 action types round-trip via PUT; Rules tab renders them', async ({ page }) => {
  const body = rulesRich(uniqueName('mdeb-rules'))
  body.rules = [
    {
      id: 'r_conds', name: 'all conditions', description: '', priority: 10, applies_to: ['runner'],
      conditions: ALL_CONDITIONS, actions: [{ type: 'move', speed: 1 }],
      execute_once: false, cooldown_steps: 0, enabled: true, locked: false,
    },
    {
      id: 'r_acts', name: 'all actions', description: '', priority: 9, applies_to: ['runner'],
      conditions: [{ type: 'step', operator: 'gte', value: 0 }], actions: ALL_ACTIONS,
      execute_once: false, cooldown_steps: 0, enabled: true, locked: false,
    },
  ]
  const seeded = await seedTracked(body) // seedModel PUTs the full body (throws on non-200)

  // [API] GET round-trips every type, in order, with key fields intact.
  const got = await getModel(seeded.id)
  const rules: Record<string, any> = Object.fromEntries(got.model.rules.map((r: any) => [r.id, r]))
  expect(rules.r_conds.conditions.map((c: any) => c.type)).toEqual([...ALL_CONDITION_TYPES])
  expect(rules.r_acts.actions.map((a: any) => a.type)).toEqual([...ALL_ACTION_TYPES])
  // Spot-check type-specific fields survived.
  expect(rules.r_conds.conditions[3]).toMatchObject({ type: 'proximity', target_uid: 'beacon', radius: 5 })
  expect(rules.r_conds.conditions[5].sub_conditions).toHaveLength(2)
  expect(rules.r_conds.conditions[12].parameters).toMatchObject({ memory_type: 'interaction' })
  expect(rules.r_acts.actions[7]).toMatchObject({ type: 'transition', new_state: 'alert' })
  expect(rules.r_acts.actions[9]).toMatchObject({
    type: 'spawn_agent', spawn_agent_uid: 'runner', spawn_count: 1, spawn_position: { type: 'nearby', radius: 3 },
  })
  expect(rules.r_acts.actions[11]).toMatchObject({ type: 'transform', spawn_agent_uid: 'beacon' })
  // Breadth rules are advisory-clean: the model stays simulation-ready.
  expect(got.validation?.simulation_ready, JSON.stringify(got.validation?.errors)).toBe(true)

  // [UI] the Rules tab renders the list without errors; counts match.
  await openEditor(page, seeded.id)
  await tabByName(page, /rules/i).click()
  const tabpanel = page.getByRole('tabpanel')
  await expect(tabpanel.getByRole('button', { name: 'all conditions' })).toBeVisible({ timeout: 15_000 })
  await expect(tabpanel.getByRole('button', { name: 'all actions' })).toBeVisible()
  // Tab header count chip shows 2 rules.
  await expect(tabByName(page, /rules/i)).toContainText('2')
  // The all-conditions rule's detail renders one ConditionBuilder per condition:
  // 15 top-level + 2 nested (the composite's sub_conditions render their own
  // ConditionBuilder cards) = 17 "Condition type" selects.
  await tabpanel.getByRole('button', { name: 'all conditions' }).click()
  await expect(tabpanel.locator('label:text-is("Condition type")')).toHaveCount(17)
  // The all-actions rule's detail renders one ActionBuilder per action.
  await tabpanel.getByRole('button', { name: 'all actions' }).click()
  await expect(tabpanel.locator('label:text-is("Action type")')).toHaveCount(14)
})

// ---------------------------------------------------------------------------
// MDE-13 — interactions tab: author one agent_agent interaction via UI,
// save, API-verify, then remove it.
// ---------------------------------------------------------------------------
test('MDE-13 author an agent_agent interaction via UI, save, verify, remove', async ({ page }) => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-inter')))
  await openEditor(page, seeded.id)
  await tabByName(page, /interactions/i).click()
  const panel = page.getByRole('tabpanel')

  await panel.getByRole('button', { name: 'Add interaction' }).click()
  await fieldInput(panel, 'ID').fill('i_meet')
  await fieldInput(panel, 'Name').fill('walkers meet')
  await fieldInput(panel, 'Radius').fill('4')
  await fieldInput(panel, 'Radius').press('Tab') // InputNumber commits on blur

  await multiSelect(page, panel, 'Initiator UIDs', ['Walker'])
  await multiSelect(page, panel, 'Target UIDs', ['Walker'])

  // One initiator action: modify_property energy += 1.
  await panel.getByRole('button', { name: 'Add initiator action' }).click()
  const action = panel.locator('div.rounded-lg.border', { has: page.locator('label:text-is("Action type")') }).first()
  await selectOption(page, action, 'Action type', 'modify_property')
  // Property UID is an editable Select — type straight into its input, then close
  // its suggestion overlay before the next select (an editable-Select overlay can
  // linger under load and make the next selectOption's "no overlay" wait fail).
  await fieldControl(action, 'Property UID', 'input').fill('energy')
  await page.keyboard.press('Escape')
  await expect(page.locator('.p-select-overlay')).toHaveCount(0)
  await selectOption(page, action, 'Modifier', 'add')
  await fieldInput(action, 'Value').fill('1')

  await saveEditor(page)

  // [API] the interaction persisted with the authored fields.
  let got = await getModel(seeded.id)
  expect(got.model.interactions).toHaveLength(1)
  const it = got.model.interactions[0]
  expect(it).toMatchObject({
    id: 'i_meet', name: 'walkers meet', type: 'agent_agent', radius: 4,
    initiator_uids: ['walker'], target_uids: ['walker'],
  })
  expect(it.initiator_actions).toHaveLength(1)
  expect(it.initiator_actions[0]).toMatchObject({ type: 'modify_property', property_uid: 'energy', modifier: 'add' })
  // Value normalization on save: a numeric-looking value referencing a numeric
  // property (energy) round-trips as a NUMBER now (was previously kept as the raw
  // UI string).
  expect(it.initiator_actions[0].value).toBe(1)

  // Remove it, save, verify gone.
  await panel.getByRole('button', { name: 'Remove interaction' }).click()
  await expect(panel.getByText('No interactions yet.')).toBeVisible()
  await saveEditor(page)
  got = await getModel(seeded.id)
  expect(got.model.interactions ?? []).toHaveLength(0)
})

// ---------------------------------------------------------------------------
// MDE-14 — events tab: author global / local(radius) / targeted(consumers)
// event definitions with TTL + producers, save, API-verify, delete one.
// ---------------------------------------------------------------------------
test('MDE-14 author global/local/targeted events with TTL + producers via UI; delete one', async ({ page }) => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-events')))
  await openEditor(page, seeded.id)
  await tabByName(page, /events/i).click()
  const panel = page.getByRole('tabpanel')

  // Event 1: global scope (default), TTL 2, producers agent+environment.
  await panel.getByRole('button', { name: 'Add event' }).click()
  await fieldInput(panel, 'UID').fill('ev_global')
  await fieldInput(panel, 'Name').fill('Global ev')
  // Global scope has no radius field.
  await expect(panel.locator('label:text-is("Radius")')).toHaveCount(0)
  await fieldInput(panel, 'TTL steps').fill('2')
  await fieldInput(panel, 'TTL steps').press('Tab')
  await multiSelect(page, panel, 'Producers', ['agent', 'environment'])

  // Event 2: local scope → the Radius field appears.
  await panel.getByRole('button', { name: 'Add event' }).click()
  await fieldInput(panel, 'UID').fill('ev_local')
  await fieldInput(panel, 'Name').fill('Local ev')
  await selectOption(page, panel, 'Scope', 'local')
  await expect(panel.locator('label:text-is("Radius")')).toHaveCount(1)
  await fieldInput(panel, 'Radius').fill('5')
  await fieldInput(panel, 'Radius').press('Tab')
  await multiSelect(page, panel, 'Producers', ['agent'])

  // Event 3: targeted scope → no radius; consumer UIDs; llm producer shows
  // the optional LLM-target section.
  await panel.getByRole('button', { name: 'Add event' }).click()
  await fieldInput(panel, 'UID').fill('ev_target')
  await fieldInput(panel, 'Name').fill('Targeted ev')
  await selectOption(page, panel, 'Scope', 'targeted')
  await expect(panel.locator('label:text-is("Radius")')).toHaveCount(0)
  await multiSelect(page, panel, 'Consumer UIDs', ['Walker'])
  await multiSelect(page, panel, 'Producers', ['llm'])
  await expect(panel.locator('label:text-is("LLM target")')).toBeVisible()

  await saveEditor(page)

  // [API] all three persisted with the authored scope/radius/ttl/producers.
  let got = await getModel(seeded.id)
  const events: Record<string, any> = Object.fromEntries(got.model.events.map((e: any) => [e.uid, e]))
  expect(Object.keys(events).sort()).toEqual(['ev_global', 'ev_local', 'ev_target'])
  expect(events.ev_global).toMatchObject({ scope: 'global', ttl_steps: 2 })
  expect([...events.ev_global.producers].sort()).toEqual(['agent', 'environment'])
  expect(events.ev_local).toMatchObject({ scope: 'local', radius: 5, producers: ['agent'] })
  expect(events.ev_target).toMatchObject({ scope: 'targeted', consumer_uids: ['walker'], producers: ['llm'] })

  // Delete the first event (select it in the master list, then Remove).
  await panel.getByRole('button', { name: 'Global ev' }).click()
  await panel.getByRole('button', { name: 'Remove event' }).click()
  await saveEditor(page)
  got = await getModel(seeded.id)
  expect(got.model.events.map((e: any) => e.uid).sort()).toEqual(['ev_local', 'ev_target'])
})

// ---------------------------------------------------------------------------
// MDE-15 — save-validation banner: duplicate/reserved uids blocked with 422
// field errors [API]; a UI-achievable blocking error (cleared agent UID)
// surfaces the inline banner and keeps the tab's data.
// ---------------------------------------------------------------------------
test('MDE-15 [API] duplicate agent uid and reserved uid "environment" are 422 with field errors', async () => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-uid')))

  // Duplicate uid: give a second agent the first one's uid. (The UI cannot
  // produce this state in one action — uids are free-text but the save gate
  // catches it — so this half is API-forced by design.)
  const dup = minimalSpatial(uniqueName('mdeb-uid-dup'))
  dup.agents.push({ ...JSON.parse(JSON.stringify(dup.agents[0])), name: 'Walker 2' }) // same uid "walker"
  const dupRes = await putModel(seeded.id, dup)
  expect(dupRes.status).toBe(422)
  const dupBody = dupRes.body as ApiErrorBody
  expect(dupBody.code).toBe('validation_failed')
  expect(dupBody.fields?.['agents[1].uid']).toContain('already used')

  // Reserved uid "environment".
  const reserved = minimalSpatial(uniqueName('mdeb-uid-res'))
  reserved.agents[0].uid = 'environment'
  const resRes = await putModel(seeded.id, reserved)
  expect(resRes.status).toBe(422)
  const resBody = resRes.body as ApiErrorBody
  expect(resBody.code).toBe('validation_failed')
  expect(resBody.fields?.['agents[0].uid']).toContain('reserved')

  // The failed PUTs must not have persisted anything.
  const got = await getModel(seeded.id)
  expect(got.model.agents).toHaveLength(1)
  expect(got.model.agents[0].uid).toBe('walker')
})

test('MDE-15 [UI] clearing an agent UID → manual Save surfaces the validation banner, data kept', async ({ page }) => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-banner')))
  await openEditor(page, seeded.id)
  await tabByName(page, /agents/i).click()

  // Clear the UID (the one save-blocking state the UI can reach directly).
  const uidInput = page.getByPlaceholder('wolf')
  await expect(uidInput).toHaveValue('walker')
  await uidInput.fill('')
  await expect(page.getByText('unsaved', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'Save', exact: true }).click()

  // Non-blocking inline banner with the field error; edits + tab data kept.
  await expect(page.getByText('Save failed — validation:')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByText(/agents\[0\]\.uid: uid is required/)).toBeVisible()
  await expect(page.getByText('unsaved', { exact: true })).toBeVisible()
  await expect(tabByName(page, /agents/i)).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByPlaceholder('Agent name')).toHaveValue('Walker') // form data intact

  // Restoring the uid clears the failure on the next save.
  await uidInput.fill('walker')
  await saveEditor(page)
})

// ---------------------------------------------------------------------------
// MDE-16 — auto-save extension: a REAL backend-rejected (422) auto-save still
// switches tabs, shows the banner, and keeps the edits. (editor-autosave.spec.ts
// covers tab-switch/route-leave and a mocked 422; this uses a genuine one.)
// ---------------------------------------------------------------------------
test('MDE-16 a real 422-failing auto-save still switches tabs and banners (edits kept)', async ({ page }) => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-autosave')))
  await openEditor(page, seeded.id)
  await tabByName(page, /agents/i).click()

  // Make the model genuinely invalid in a way the UI allows: empty agent UID.
  await page.getByPlaceholder('wolf').fill('')
  await expect(page.getByText('unsaved', { exact: true })).toBeVisible()

  // Switch tabs → auto-save fires a real PUT that the backend 422s.
  const [put] = await Promise.all([
    page.waitForResponse((r) => r.request().method() === 'PUT' && /\/models\/[^/]+$/.test(r.url())),
    tabByName(page, /rules/i).click(),
  ])
  expect(put.status()).toBe(422)

  // The switch is allowed, the banner shows the real field error, edits stay.
  await expect(tabByName(page, /rules/i)).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByText('Save failed — validation:')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByText(/uid is required/)).toBeVisible()
  await expect(page.getByText('unsaved', { exact: true })).toBeVisible()
})

// ---------------------------------------------------------------------------
// MDE-17 — external-update conflict: an API PUT while the editor holds dirty
// edits raises the conflict banner; both resolution paths behave. The banner
// copy is source-agnostic ("updated elsewhere") since any external model.updated
// triggers it (here a plain API PUT).
// ---------------------------------------------------------------------------
const CONFLICT_TEXT = 'This model was updated elsewhere while you have unsaved changes.'

async function makeDirtyDescription(page: Page, marker: string) {
  // Dirty the editor WITHOUT switching tabs (tab switches auto-save).
  const desc = page.locator('div:has(> label:text-is("Description")) textarea').first()
  await expect(desc).toBeVisible()
  await desc.fill(marker)
  await expect(page.getByText('unsaved', { exact: true })).toBeVisible()
}

test('MDE-17a conflict banner → "Apply update" adopts the external version, local edits discarded', async ({ page }) => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-conf-a')))
  await openEditor(page, seeded.id)
  await makeDirtyDescription(page, 'local dirty edit A')

  // External update via API: rename the model.
  const current = await getModel(seeded.id)
  const externalName = `${seeded.name}-ext`
  await apiJson(api, 'put', `/models/${seeded.id}`, { model: { ...current.model, name: externalName } }, 200)

  // Conflict banner appears (via the model SSE) with both resolution buttons.
  await expect(page.getByText(CONFLICT_TEXT)).toBeVisible({ timeout: 15_000 })
  await page.getByRole('button', { name: 'Apply update (discard my edits)' }).click()

  // The editor now reflects the external version; the dirty edit is gone.
  await expect(page.getByText(CONFLICT_TEXT)).toHaveCount(0)
  await expect(page.getByPlaceholder('Model name')).toHaveValue(externalName)
  await expect(page.locator('div:has(> label:text-is("Description")) textarea').first())
    .not.toHaveValue('local dirty edit A')
  await expect(page.getByText('unsaved', { exact: true })).toHaveCount(0)
})

test('MDE-17b conflict banner → "Keep my edits" keeps local state; next Save overwrites the external update', async ({ page }) => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-conf-b')))
  await openEditor(page, seeded.id)
  await makeDirtyDescription(page, 'local dirty edit B')

  const current = await getModel(seeded.id)
  const externalName = `${seeded.name}-ext`
  await apiJson(api, 'put', `/models/${seeded.id}`, { model: { ...current.model, name: externalName } }, 200)

  await expect(page.getByText(CONFLICT_TEXT)).toBeVisible({ timeout: 15_000 })
  await page.getByRole('button', { name: 'Keep my edits' }).click()
  await expect(page.getByText(CONFLICT_TEXT)).toHaveCount(0)

  // Local state kept: old name, dirty description, still unsaved.
  await expect(page.getByPlaceholder('Model name')).toHaveValue(seeded.name)
  await expect(page.getByText('unsaved', { exact: true })).toBeVisible()

  // The next manual Save makes the local version authoritative (overwrites
  // the external rename).
  await saveEditor(page)
  const after = await getModel(seeded.id)
  expect(after.model.name).toBe(seeded.name)
  expect(after.model.description).toBe('local dirty edit B')
})

// ---------------------------------------------------------------------------
// MDE-18 — editor header: Duplicate dialog → copy → redirect; "New simulation"
// readiness gating; Summary⟷Chat toggle; right-pane collapse.
// ---------------------------------------------------------------------------
test('MDE-18 header: Duplicate redirects to the copy; New simulation gated on readiness; pane toggle + collapse', async ({ page }) => {
  const ready = await seedTracked(minimalSpatial(uniqueName('mdeb-header')))
  const draftName = uniqueName('mdeb-header-draft')
  const draftId = await createBlank(draftName)

  // Draft model: "New simulation" disabled (not simulation-ready).
  await openEditor(page, draftId)
  await expect(page.getByText('Not ready', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'New simulation' })).toBeDisabled()

  // Ready model: enabled.
  await page.goto(`/models/${ready.id}`)
  await expect(page.getByPlaceholder('Model name')).toHaveValue(ready.name, { timeout: 20_000 })
  await expect(page.getByText('Ready', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'New simulation' })).toBeEnabled()

  // Summary ⟷ Chat SelectButton toggles the right pane.
  await expect(page.getByText('Model summary')).toBeVisible()
  await page.getByRole('button', { name: 'Chat', exact: true }).click()
  await expect(page.getByText('Model-editing assistant')).toBeVisible()
  await expect(page.getByText('Model summary')).toBeHidden()
  await page.getByRole('button', { name: 'Summary', exact: true }).click()
  await expect(page.getByText('Model summary')).toBeVisible()

  // Collapse hides the whole pane (and the Summary/Chat selector); re-expand restores it.
  const collapse = page.getByRole('button', { name: 'Toggle side panel' })
  await collapse.click()
  await expect(page.getByText('Model summary')).toBeHidden()
  await expect(page.getByRole('button', { name: 'Summary', exact: true })).toHaveCount(0)
  await collapse.click()
  await expect(page.getByText('Model summary')).toBeVisible()

  // Duplicate: dialog defaults "<name> (copy)", creates the copy, redirects.
  await page.getByRole('button', { name: 'Duplicate', exact: true }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog).toContainText('Duplicate model')
  await expect(dialog.getByRole('textbox')).toHaveValue(`${ready.name} (copy)`)
  await dialog.getByRole('button', { name: 'Duplicate', exact: true }).click()

  await page.waitForURL((url) => /\/models\/[^/?#]+$/.test(url.pathname) && !url.pathname.endsWith(ready.id), { timeout: 20_000 })
  const copyId = page.url().match(/\/models\/([^/?#]+)/)![1]
  trackId(copyId)
  await expect(page.getByPlaceholder('Model name')).toHaveValue(`${ready.name} (copy)`, { timeout: 15_000 })
})

// ---------------------------------------------------------------------------
// MDE-19 — LLM tab: three config fieldsets; model+effort visible by default;
// Advanced (generation + timeout) collapsed, auto-opens when values are set.
// (llm-config-advanced.spec.ts covers the collapse/persist cycle for one slot;
// this covers the 3-slot layout + per-slot Advanced gating.)
// ---------------------------------------------------------------------------
test('MDE-19 LLM tab renders 3 config slots; Advanced gated per slot and auto-opens when set', async ({ page }) => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-llm')))

  // Seed: model-gen target WITH an advanced value (timeout) → auto-open;
  // sim-gen target with NO advanced values → Advanced present but collapsed;
  // exec target empty → no Advanced at all.
  const current = await getModel(seeded.id)
  const model = {
    ...current.model,
    llm_config: { target: { provider_uid: 'openai', model_uid: 'gpt-4o', timeout_ms: 45000 } },
    simulation_config: {
      ...current.model.simulation_config,
      llm_simulation_generation_config: { target: { provider_uid: 'openai', model_uid: 'gpt-4o' } },
    },
  }
  await apiJson(api, 'put', `/models/${seeded.id}`, { model }, 200)

  await openEditor(page, seeded.id)
  await tabByName(page, /llm/i).click()

  // The three slots render as legended fieldsets.
  const genFs = page.locator('fieldset', { hasText: 'Model generation & chat LLM' }).first()
  const simGenFs = page.locator('fieldset', { hasText: 'Simulation generation LLM' }).first()
  const execFs = page.locator('fieldset', { hasText: 'Simulation execution LLM' }).first()
  await expect(genFs).toBeVisible({ timeout: 15_000 })
  await expect(simGenFs).toBeVisible()
  await expect(execFs).toBeVisible()

  // Each slot shows a model picker by default (chosen model or placeholder).
  await expect(genFs.locator('.p-select').first()).toContainText('GPT-4o', { timeout: 15_000 })
  await expect(simGenFs.locator('.p-select').first()).toContainText('GPT-4o')
  await expect(execFs.locator('.p-select').first()).toContainText('Select execution model')

  // Model-gen slot: target carries timeout_ms → Advanced AUTO-OPENED, value shown.
  const genTimeout = genFs.getByPlaceholder('30000')
  await expect(genFs.getByText('Advanced', { exact: true })).toBeVisible()
  await expect(genTimeout).toBeVisible()
  await expect(genTimeout).toHaveValue(/45,?000/)

  // Sim-gen slot: target set but no advanced values → Advanced present, COLLAPSED.
  await expect(simGenFs.getByText('Advanced', { exact: true })).toBeVisible()
  await expect(simGenFs.getByPlaceholder('30000')).not.toBeVisible()
  // Expanding reveals generation params + timeout.
  await simGenFs.getByText('Advanced', { exact: true }).click()
  await expect(simGenFs.getByPlaceholder('30000')).toBeVisible()
  await expect(simGenFs.getByText('Temperature')).toBeVisible()

  // Exec slot: no model chosen → no Advanced section (gated on model_uid);
  // its own Strategy/Simulation config fieldsets are the exec-specific knobs.
  await expect(execFs.getByText('Advanced', { exact: true })).toHaveCount(0)
  await expect(execFs.getByText('Strategy', { exact: true })).toBeVisible()
})

// ---------------------------------------------------------------------------
// MDE-20 — capability gating on save [API]: llm placement without a sim-gen
// target ⇒ 422 sim_generation_target_required; an LLM-using execution strategy
// without an exec target ⇒ 422 sim_execution_target_required. Fixing the
// target clears each 422. Observed trigger conditions (live-verified):
//   generation gate: any agent/shape with count_mode=llm OR placement.type=llm
//     OR a property generation.mode=llm;
//   execution gate: default (or per-agent/shape) strategy with type≠none AND
//     ≥1 use_for_* purpose (type=decision canonicalizes to use_for_decisions);
//     strategy type=none does NOT gate.
// ---------------------------------------------------------------------------
test('MDE-20 [API] sim_generation_target_required / sim_execution_target_required gate and clear', async () => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-gate')))

  // --- generation gate: llm placement, no sim-gen target ---
  const genModel = minimalSpatial(uniqueName('mdeb-gate-gen'))
  genModel.agents[0].initialization.placement = { type: 'llm', prompt_context: 'x' }
  const genRes = await putModel(seeded.id, genModel)
  expect(genRes.status).toBe(422)
  expect((genRes.body as ApiErrorBody).code).toBe('sim_generation_target_required')

  // Fix with a connected simulation_generation-capable target → 200.
  const genTarget = await pickCheapModel('simulation_generation')
  expect(genTarget, 'a connected provider must advertise simulation_generation').toBeTruthy()
  genModel.simulation_config = {
    update_mode: 'sequential',
    llm_simulation_generation_config: { target: genTarget! },
    llm_simulation_execution_config: { target: { provider_uid: '', model_uid: '' }, default_strategy: { type: 'none' } },
  }
  const genFixed = await putModel(seeded.id, genModel)
  expect(genFixed.status, JSON.stringify(genFixed.body)).toBe(200)

  // --- execution gate: decision strategy, no exec target ---
  const execModel = minimalSpatial(uniqueName('mdeb-gate-exec'))
  execModel.simulation_config = {
    update_mode: 'sequential',
    llm_simulation_generation_config: { target: { provider_uid: '', model_uid: '' } },
    llm_simulation_execution_config: {
      target: { provider_uid: '', model_uid: '' },
      default_strategy: { type: 'decision', use_for_decisions: true, decision_frequency: 1, batch_size: 0 },
    },
  }
  const execRes = await putModel(seeded.id, execModel)
  expect(execRes.status).toBe(422)
  expect((execRes.body as ApiErrorBody).code).toBe('sim_execution_target_required')

  // Control: strategy type "none" does NOT trigger the gate.
  execModel.simulation_config.llm_simulation_execution_config.default_strategy = { type: 'none' }
  const noneRes = await putModel(seeded.id, execModel)
  expect(noneRes.status, JSON.stringify(noneRes.body)).toBe(200)

  // Fix the decision strategy with a simulation_decisions-capable target → 200.
  const execTarget = await pickCheapModel('simulation_decisions')
  expect(execTarget, 'a connected provider must advertise simulation_decisions').toBeTruthy()
  execModel.simulation_config.llm_simulation_execution_config = {
    target: execTarget!,
    default_strategy: { type: 'decision', use_for_decisions: true, decision_frequency: 1, batch_size: 0 },
  }
  const execFixed = await putModel(seeded.id, execModel)
  expect(execFixed.status, JSON.stringify(execFixed.body)).toBe(200)
})

// ---------------------------------------------------------------------------
// MDE-21 — images: Add-image dialog → thumbnail in the library; remove image.
// Replaces the stale editor-UI half of multi-image-environment.spec.ts (that
// spec still expects the pre-tabs accordion editor). NO "Generate environment"
// click — no LLM call.
// ---------------------------------------------------------------------------
const TINY_PNG_B64 =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='

test('MDE-21 image library: add via dialog shows the thumbnail + count; remove restores', async ({ page }) => {
  const seeded = await seedTracked(minimalSpatial(uniqueName('mdeb-img')))
  // Seed one image via the API so the library renders non-empty on open.
  await apiJson(api, 'post', `/models/${seeded.id}/images`, {
    name: 'floor', description: 'floor plan', mime_type: 'image/png', data: TINY_PNG_B64,
  }, 201)

  await openEditor(page, seeded.id)
  await tabByName(page, /environment/i).click()

  const count = page.getByTestId('image-library-count')
  await expect(count).toContainText('1/5', { timeout: 15_000 })
  await expect(page.locator('img[alt="floor"]')).toBeVisible()
  // The generate button exists but is NEVER clicked (LLM).
  await expect(page.getByTestId('generate-environment')).toBeAttached()

  // Add a second image through the dialog (real file upload).
  await page.getByTestId('add-image-tile').click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await dialog.locator('input[type=file]').setInputFiles({
    name: 'zones.png', mimeType: 'image/png', buffer: Buffer.from(TINY_PNG_B64, 'base64'),
  })
  await dialog.locator('input:not([type=file])').first().fill('zones')
  await dialog.locator('textarea').first().fill('zone overlay')
  await page.getByTestId('confirm-add-image').click()

  await expect(count).toContainText('2/5', { timeout: 15_000 })
  await expect(page.locator('img[alt="zones"]')).toBeVisible()

  // [API] the library persisted both images.
  let got = await getModel(seeded.id)
  expect((got.model.images ?? []).map((i: any) => i.name).sort()).toEqual(['floor', 'zones'])

  // Remove one via its thumbnail × → count back to 1/5, API agrees.
  await page.getByTestId('remove-image').first().click()
  await expect(count).toContainText('1/5', { timeout: 15_000 })
  got = await getModel(seeded.id)
  expect(got.model.images ?? []).toHaveLength(1)
})

// ---------------------------------------------------------------------------
// MDE-22 — GET /models/:id/validation reflects readiness [API].
// ---------------------------------------------------------------------------
test('MDE-22 [API] validation endpoint: blank draft not ready with errors; fixed model ready with none', async () => {
  const id = await createBlank(uniqueName('mdeb-valid'))

  // Blank draft: errors non-empty (no_agents), not simulation-ready.
  const before = await apiJson<any>(api, 'get', `/models/${id}/validation`, undefined, 200)
  expect(before.simulation_ready).toBe(false)
  expect(before.errors?.length ?? 0).toBeGreaterThan(0)
  expect(before.errors.map((e: any) => e.code)).toContain('no_agents')

  // Fix it with the minimal spatial body → ready, errors empty.
  await apiJson(api, 'put', `/models/${id}`, { model: minimalSpatial(uniqueName('mdeb-valid-fixed')) }, 200)
  const after = await apiJson<any>(api, 'get', `/models/${id}/validation`, undefined, 200)
  expect(after.simulation_ready).toBe(true)
  expect(after.errors ?? []).toHaveLength(0)
})
