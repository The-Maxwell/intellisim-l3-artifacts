import { test, expect, type APIRequestContext, type Page } from '@playwright/test'
import { apiContext, apiJson, u } from '../_helpers/api'
import { uniqueName, deleteQuietly } from '../_helpers/fixtures'
import { login } from '../_helpers/login'

// CHAT suite — deterministic tier (plan §5.2, CHAT-01…06). No LLM calls:
// chats are seeded/renamed/deleted via API + UI chrome only; messages are
// NEVER sent (POST /chats/:id/messages would trigger a provider generation).

interface ChatDTO {
  id: string
  name: string
  description: string
  llm_target: { provider_uid: string; model_uid: string; effort?: string }
  status: string
  model_available: boolean
}

interface LlmsResponse {
  list: {
    uid: string
    name: string
    models: { uid: string; name: string; capabilities: string[] }[]
  }[]
}

// A chat-capable llm_target from the live registry (provider must be up).
async function chatTarget(api: APIRequestContext): Promise<{ provider_uid: string; model_uid: string }> {
  const body = await apiJson<LlmsResponse>(api, 'get', '/llms', undefined, 200)
  for (const p of body.list ?? [])
    for (const m of p.models)
      if (m.capabilities.includes('chat')) return { provider_uid: p.uid, model_uid: m.uid }
  throw new Error('no chat-capable LLM model advertised on GET /llms — is the provider connected?')
}

async function seedChat(api: APIRequestContext, name: string): Promise<ChatDTO> {
  const target = await chatTarget(api)
  return apiJson<ChatDTO>(api, 'post', '/chats', { name, llm_target: target }, 201)
}

// Sidebar chat entries are router-links whose accessible name is the chat name.
function sidebarChatLink(page: Page, name: string) {
  return page.getByRole('link', { name })
}

test.describe('chats (general)', () => {
  test('CHAT-01 seeded chat appears in sidebar after reload and navigates to /chat/:id', async ({ page }) => {
    const api = await apiContext()
    const name = uniqueName('chat')
    let chatId = ''
    try {
      await login(page)
      const chat = await seedChat(api, name)
      chatId = chat.id

      // Sidebar loads the chat list on app-shell mount → reload picks it up.
      await page.reload()
      await expect(page.getByText('General Chats')).toBeVisible()
      const link = sidebarChatLink(page, name)
      await expect(link).toBeVisible()

      await link.click()
      await page.waitForURL(`**/chat/${chatId}`)
      await expect(page.getByRole('heading', { name })).toBeVisible()
    } finally {
      if (chatId) await deleteQuietly(api, `/chats/${chatId}`)
      await api.dispose()
    }
  })

  test('CHAT-02 inline rename persists after reload; Esc cancels an edit', async ({ page }) => {
    const api = await apiContext()
    const name = uniqueName('chat')
    const renamed = uniqueName('chat-renamed')
    let chatId = ''
    try {
      await login(page)
      const chat = await seedChat(api, name)
      chatId = chat.id
      await page.goto(`/chat/${chatId}`)
      await expect(page.getByRole('heading', { name })).toBeVisible()

      // Rename: pencil → input (maxlength 100 = CHAT_NAME_MAX) → Enter saves.
      const renameInput = page.locator('input[maxlength="100"]')
      await page.locator('button[title="Rename"]').click()
      await expect(renameInput).toBeVisible()
      await renameInput.fill(renamed)
      await renameInput.press('Enter')
      await expect(page.getByRole('heading', { name: renamed })).toBeVisible()

      // Persists server-side.
      await page.reload()
      await expect(page.getByRole('heading', { name: renamed })).toBeVisible()
      // Sidebar entry follows the rename too.
      await expect(sidebarChatLink(page, renamed)).toBeVisible()

      // Esc cancels: typed text is discarded, no save happens.
      await page.locator('button[title="Rename"]').click()
      await renameInput.fill(`${renamed}-discarded`)
      await renameInput.press('Escape')
      await expect(renameInput).toBeHidden()
      await expect(page.getByRole('heading', { name: renamed })).toBeVisible()
      await page.reload()
      await expect(page.getByRole('heading', { name: renamed })).toBeVisible()
      const after = await apiJson<ChatDTO>(api, 'get', `/chats/${chatId}`, undefined, 200)
      expect(after.name).toBe(renamed)
    } finally {
      if (chatId) await deleteQuietly(api, `/chats/${chatId}`)
      await api.dispose()
    }
  })

  test('CHAT-03 chat deletion: sidebar hover trash → confirm dialog → row gone + API 404', async ({ page }) => {
    const api = await apiContext()
    const name = uniqueName('chat')
    let chatId = ''
    try {
      await login(page)
      const chat = await seedChat(api, name)
      chatId = chat.id
      // Sidebar loads the chat list on app-shell mount → reload picks it up.
      await page.reload()
      const link = sidebarChatLink(page, name)
      await expect(link).toBeVisible()

      // Delete affordance (2026-07-14): each sidebar row has a hover-revealed
      // trash button (aria-label "Delete chat"); clicking it opens the global
      // ConfirmDialog (header "Delete chat", accept "Delete").
      await link.hover()
      const delBtn = link.getByRole('button', { name: 'Delete chat' })
      await expect(delBtn).toBeVisible()
      await delBtn.click()

      const confirm = page.getByRole('alertdialog').or(page.getByRole('dialog'))
      await expect(confirm.getByText('Delete chat')).toBeVisible()
      await confirm.getByRole('button', { name: 'Delete', exact: true }).click()

      // The row disappears from the sidebar (optimistic removeGeneralChat) …
      await expect(sidebarChatLink(page, name)).toHaveCount(0)
      // … and the delete propagated server-side.
      const gone = await api.get(u(`/chats/${chatId}`))
      expect(gone.status()).toBe(404)
      chatId = '' // deleted — skip cleanup
    } finally {
      if (chatId) await deleteQuietly(api, `/chats/${chatId}`)
      await api.dispose()
    }
  })

  test('CHAT-04 /chat/new lists provider models in the target picker and renders 4 suggested-prompt chips', async ({ page }) => {
    const api = await apiContext()
    try {
      const llms = await apiJson<LlmsResponse>(api, 'get', '/llms', undefined, 200)
      const expectedLabels = (llms.list ?? []).flatMap((p) =>
        p.models.filter((m) => m.capabilities.includes('chat')).map((m) => `${p.name} · ${m.name}`),
      )
      expect(expectedLabels.length).toBeGreaterThan(0)

      await login(page)
      await page.goto('/chat/new')
      await expect(page.getByText('IntelliSim Chat')).toBeVisible()

      // LlmTargetPicker: fresh context has no stored default target → the
      // single PrimeVue Select shows the placeholder; open it and compare the
      // options against GET /llms chat-capable models.
      const select = page.locator('.p-select').first()
      await expect(select).toBeVisible()
      await select.click()
      const options = page.getByRole('option')
      await expect(options).toHaveCount(expectedLabels.length)
      for (const label of expectedLabels) {
        await expect(page.getByRole('option', { name: label, exact: true })).toBeVisible()
      }
      await page.keyboard.press('Escape') // close without selecting — no chat is created

      // 4 suggested-prompt chips (rendered only — clicking would create a
      // chat and trigger an LLM generation, forbidden in the core tier).
      const prompts = [
        'What is agent-based modeling?',
        'How do agents interact in a simulation?',
        'What is emergent behavior in complex systems?',
        'Where is agent-based modeling used in the real world?',
      ]
      for (const prompt of prompts) {
        await expect(page.getByRole('button', { name: prompt })).toBeVisible()
      }
    } finally {
      await api.dispose()
    }
  })

  test('CHAT-05 message input enforces maxlength 10000 and shows the counter from 9000', async ({ page }) => {
    await login(page)
    await page.goto('/chat/new')
    const input = page.getByPlaceholder('Ask anything...')
    await expect(input).toBeVisible()
    await expect(input).toHaveAttribute('maxlength', '10000')

    // Below the threshold: no counter.
    await input.fill('a'.repeat(8999))
    await expect(page.getByText('8999/10000')).toHaveCount(0)

    // At/past the threshold the live counter appears.
    await input.fill('a'.repeat(9000))
    await expect(page.getByText('9000/10000')).toBeVisible()
    await input.fill('a'.repeat(9500))
    await expect(page.getByText('9500/10000')).toBeVisible()

    // At the cap: counter shows the limit and real keystrokes are rejected.
    await input.fill('a'.repeat(10000))
    await expect(page.getByText('10000/10000')).toBeVisible()
    await input.press('x') // NOT Enter — sending would trigger the LLM
    await expect(input).toHaveValue('a'.repeat(10000))

    await input.fill('') // leave the page clean; nothing was sent
  })

  test('CHAT-06 [API] chat CRUD + message-list pagination and cursor errors', async () => {
    const api = await apiContext()
    const name = uniqueName('chat-crud')
    let chatId = ''
    try {
      const target = await chatTarget(api)

      // create → 201, fields echoed, idle status
      const created = await apiJson<ChatDTO>(
        api, 'post', '/chats',
        { name, description: 'e2e crud chat', llm_target: target }, 201,
      )
      chatId = created.id
      expect(created.id).toBeTruthy()
      expect(created.name).toBe(name)
      expect(created.description).toBe('e2e crud chat')
      expect(created.llm_target.provider_uid).toBe(target.provider_uid)
      expect(created.llm_target.model_uid).toBe(target.model_uid)
      expect(created.status).toBe('idle')

      // get → 200
      const got = await apiJson<ChatDTO>(api, 'get', `/chats/${chatId}`, undefined, 200)
      expect(got.id).toBe(chatId)
      expect(got.name).toBe(name)

      // update → 200; empty update (ErrNoChanges) → 422 validation_failed
      const newName = `${name}-upd`
      const updated = await apiJson<ChatDTO>(api, 'put', `/chats/${chatId}`, { name: newName }, 200)
      expect(updated.name).toBe(newName)
      const noChanges = await api.put(u(`/chats/${chatId}`), { data: {} })
      expect(noChanges.status()).toBe(422)
      expect((await noChanges.json()).code).toBe('validation_failed')

      // list → contains the chat, pagination envelope present
      const list = await apiJson<{ chats: ChatDTO[]; page: number; page_size: number; has_more: boolean }>(
        api, 'get', `/chats?page=1&page_size=50`, undefined, 200,
      )
      expect(list.page).toBe(1)
      expect(list.page_size).toBe(50)
      expect(list.chats.some((c) => c.id === chatId)).toBe(true)

      // messages happy path: empty page (no messages are ever sent in this tier)
      const msgs = await apiJson<{ messages: unknown[]; has_more: boolean }>(
        api, 'get', `/chats/${chatId}/messages?limit=10`, undefined, 200,
      )
      expect(msgs.messages).toEqual([])
      expect(msgs.has_more).toBe(false)

      // cursor param is `before` (24-hex message id — see useChatApi.listMessages).
      // A malformed cursor fails DTO validation → 422 validation_failed{fields.before};
      // a WELL-FORMED but unknown/foreign cursor → 400 invalid_cursor (invalid_cursor
      // stays 400; only request-schema validation moved to 422).
      const malformed = await api.get(u(`/chats/${chatId}/messages?before=garbage`))
      expect(malformed.status()).toBe(422)
      const malformedBody = await malformed.json()
      expect(malformedBody.code).toBe('validation_failed')
      expect(Object.keys(malformedBody.fields ?? {})).toContain('before')

      const unknownCursor = await api.get(u(`/chats/${chatId}/messages?before=aaaaaaaaaaaaaaaaaaaaaaaa`))
      expect(unknownCursor.status()).toBe(400)
      expect((await unknownCursor.json()).code).toBe('invalid_cursor')

      // limit is capped at 100 → over-cap fails validation with 422
      const badLimit = await api.get(u(`/chats/${chatId}/messages?limit=101`))
      expect(badLimit.status()).toBe(422)
      expect((await badLimit.json()).code).toBe('validation_failed')

      // delete → 204; get → 404 not_found; double delete → 404
      await apiJson(api, 'delete', `/chats/${chatId}`, undefined, 204)
      const gone = await api.get(u(`/chats/${chatId}`))
      expect(gone.status()).toBe(404)
      expect((await gone.json()).code).toBe('not_found')
      const again = await api.delete(u(`/chats/${chatId}`))
      expect(again.status()).toBe(404)
      chatId = ''
    } finally {
      if (chatId) await deleteQuietly(api, `/chats/${chatId}`)
      await api.dispose()
    }
  })
})
