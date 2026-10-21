import { test, expect, type APIRequestContext, type Page, type Locator } from '@playwright/test'
import { apiContext, apiJson, u } from '../_helpers/api'
import { uniqueName, deleteQuietly } from '../_helpers/fixtures'
import { login } from '../_helpers/login'

// CHAT @llm suite — plan §5.2, CHAT-07…11. These are the ONLY chat cases that
// call the REAL OpenAI provider. Assertions are STRUCTURAL (a reply bubble
// arrived, status/URL transitions) — never on LLM content. Model is always the
// cheapest chat-capable one (gpt-4.1-nano). Messages are short. Total LLM sends
// across the whole suite: CHAT-07=1, 08=1, 09=1, 10=2, 11=1 = 6 (≤ 8 budget).

interface ChatDTO {
  id: string
  name: string
  llm_target: { provider_uid: string; model_uid: string; effort?: string }
  status: string
}

interface LlmsResponse {
  list: {
    uid: string
    name: string
    models: { uid: string; name: string; capabilities: string[] }[]
  }[]
}

// Mirrors _helpers/llm.ts CHEAP_ORDER so [0] == pickCheapModel('chat').
const CHEAP_ORDER = ['gpt-4.1-nano', 'gpt-4o-mini', 'gpt-4.1-mini', 'o4-mini', 'gpt-4.1', 'gpt-4o', 'gpt-3.5-turbo', 'o3']

interface ChatModel {
  provider_uid: string
  model_uid: string
  label: string // picker option label — `${providerName} · ${modelName}`
}

// Cheapest-first chat-capable models from the live registry (provider must be up).
async function cheapChatModels(api: APIRequestContext): Promise<ChatModel[]> {
  const body = await apiJson<LlmsResponse>(api, 'get', '/llms', undefined, 200)
  const out: ChatModel[] = []
  for (const p of body.list ?? [])
    for (const m of p.models)
      if (m.capabilities.includes('chat'))
        out.push({ provider_uid: p.uid, model_uid: m.uid, label: `${p.name} · ${m.name}` })
  out.sort((a, b) => {
    const ia = CHEAP_ORDER.indexOf(a.model_uid)
    const ib = CHEAP_ORDER.indexOf(b.model_uid)
    return (ia === -1 ? 99 : ia) - (ib === -1 ? 99 : ib)
  })
  if (!out.length) throw new Error('no chat-capable model advertised on GET /llms — is the provider connected?')
  return out
}

// Open a PrimeVue Select under `root` and choose the option whose accessible
// name EXACTLY equals `label`. exact:true matters: "OpenAI · GPT-4o Mini" would
// otherwise be matched by the "OpenAI · GPT-4o" prefix.
async function pickModel(page: Page, root: Page | Locator, label: string): Promise<void> {
  const select = root.locator('.p-select').first()
  await expect(select).toBeVisible()
  await select.scrollIntoViewIfNeeded().catch(() => {})
  await select.click()
  await page.getByRole('option', { name: label, exact: true }).click()
  await expect(select).toContainText(label)
}

// Wait until the assistant-reply bubble count reaches `expected` (each assistant
// message renders one `.prose` div; user bubbles use `.whitespace-pre-wrap`).
// This distinguishes the SECOND reply from the first (CHAT-10).
async function waitForReplyCount(scope: Page | Locator, expected: number, timeout = 150_000): Promise<void> {
  await expect(scope.locator('.prose')).toHaveCount(expected, { timeout })
  await expect(scope.locator('.prose').last()).not.toBeEmpty()
}

function chatIdFromUrl(page: Page): string {
  const m = page.url().match(/\/chat\/([a-f0-9]{24})/)
  if (!m) throw new Error(`no chat id in url: ${page.url()}`)
  return m[1]
}

async function listChatIds(api: APIRequestContext): Promise<string[]> {
  const r = await apiJson<{ chats: { id: string }[] }>(api, 'get', '/chats?page=1&page_size=100', undefined, 200)
  return (r.chats ?? []).map((c) => c.id)
}

// Every chat created (via UI or API) is tracked and deleted in afterAll so no
// leftovers remain even if a test throws mid-way.
const createdChatIds = new Set<string>()
let sweepApi: APIRequestContext

test.beforeAll(async () => {
  sweepApi = await apiContext()
})
test.afterAll(async () => {
  for (const id of createdChatIds) await deleteQuietly(sweepApi, `/chats/${id}`)
  await sweepApi.dispose()
})

test.describe('chats (LLM)', () => {
  test('CHAT-07 @llm first send from /chat/new creates chat, streams reply via SSE', async ({ page }) => {
    const api = await apiContext()
    try {
      const cheap = (await cheapChatModels(api))[0]
      await login(page)
      await page.goto('/chat/new')
      await expect(page.getByText('IntelliSim Chat')).toBeVisible()

      await pickModel(page, page, cheap.label)

      const input = page.getByPlaceholder('Ask anything...')
      await input.fill('Reply with the single word: pong')
      await input.press('Enter')

      // URL flips to /chat/:id (router.replace after createChat + send).
      await page.waitForURL(/\/chat\/[a-f0-9]{24}/, { timeout: 30_000 })
      const id = chatIdFromUrl(page)
      createdChatIds.add(id)

      // User bubble is immediate (optimistic upsert from the 202 response).
      await expect(page.getByText('Reply with the single word: pong').last()).toBeVisible()
      // Thinking indicator while the provider generates.
      await expect(page.locator('.thinking-dot').first()).toBeVisible({ timeout: 30_000 })
      // Assistant reply arrives via SSE without a reload.
      await waitForReplyCount(page, 1)

      // The chat was created with the cheap target.
      const chat = await apiJson<ChatDTO>(api, 'get', `/chats/${id}`, undefined, 200)
      expect(chat.llm_target.model_uid).toBe(cheap.model_uid)
    } finally {
      await api.dispose()
    }
  })

  test('CHAT-08 @llm suggested-prompt chip sends immediately and gets a reply', async ({ page }) => {
    const api = await apiContext()
    try {
      const cheap = (await cheapChatModels(api))[0]
      await login(page)
      await page.goto('/chat/new')
      await expect(page.getByText('IntelliSim Chat')).toBeVisible()

      await pickModel(page, page, cheap.label)

      const prompt = 'What is agent-based modeling?'
      await page.getByRole('button', { name: prompt }).click()

      await page.waitForURL(/\/chat\/[a-f0-9]{24}/, { timeout: 30_000 })
      const id = chatIdFromUrl(page)
      createdChatIds.add(id)

      // The chip text now lives in the user bubble (empty-state chips are gone).
      await expect(page.getByText(prompt).last()).toBeVisible()
      await waitForReplyCount(page, 1)
    } finally {
      await api.dispose()
    }
  })

  test('CHAT-09 @llm Quick Chat popup: send, reply, open full page, minimize', async ({ page }) => {
    const api = await apiContext()
    try {
      const cheap = (await cheapChatModels(api))[0]
      await login(page)
      // Launcher is hidden on /chat/* → drive it from /models.
      await page.goto('/models')
      const before = await listChatIds(api)

      const launcher = page.locator('button[title="Quick Chat"]')
      await expect(launcher).toBeVisible()
      await launcher.click()

      const popup = page.locator('.fixed.bottom-20.right-6')
      await expect(popup).toBeVisible()

      await pickModel(page, popup, cheap.label)

      const input = popup.getByPlaceholder('Ask a quick question...')
      await input.fill('Reply with the single word: pong')
      await input.press('Enter')
      await waitForReplyCount(popup, 1)

      // Capture the freshly-created quick chat (name defaults to "Quick Chat")
      // by diffing the chat list — record before we touch navigation.
      const after = await listChatIds(api)
      for (const cid of after.filter((x) => !before.includes(x))) createdChatIds.add(cid)

      // Minimize collapses the popup.
      await popup.locator('button[title="Minimize"]').click()
      await expect(popup).toBeHidden()

      // Reopen the same quick chat, then "open full page" navigates to /chat/:id.
      await launcher.click()
      await expect(popup).toBeVisible()
      await popup.locator('button[title="Open full page"]').click()
      await page.waitForURL(/\/chat\/[a-f0-9]{24}/, { timeout: 15_000 })
      createdChatIds.add(chatIdFromUrl(page))
    } finally {
      await api.dispose()
    }
  })

  test('CHAT-10 @llm change chat LLM target between messages', async ({ page }) => {
    const api = await apiContext()
    try {
      const models = await cheapChatModels(api)
      const first = models[0]
      const second = models.find((m) => m.model_uid !== first.model_uid)

      const name = uniqueName('chat-llm10')
      const chat = await apiJson<ChatDTO>(
        api, 'post', '/chats',
        { name, llm_target: { provider_uid: first.provider_uid, model_uid: first.model_uid } }, 201,
      )
      createdChatIds.add(chat.id)

      await login(page)
      await page.goto(`/chat/${chat.id}`)
      await expect(page.getByRole('heading', { name })).toBeVisible()

      const input = page.getByPlaceholder('Ask anything...')
      await input.fill('Reply with the single word: pong')
      await input.press('Enter')
      await waitForReplyCount(page, 1)

      // Switch the header target picker to a DIFFERENT cheap chat model.
      expect(second, 'need a second distinct cheap chat model for CHAT-10').toBeTruthy()
      await pickModel(page, page, second!.label)

      // [API] GET /chats/:id reflects the new llm_target.
      await expect
        .poll(async () => (await apiJson<ChatDTO>(api, 'get', `/chats/${chat.id}`, undefined, 200)).llm_target.model_uid, { timeout: 10_000 })
        .toBe(second!.model_uid)

      // A second message under the new target still gets a reply.
      await input.fill('Reply with the single word: ping')
      await input.press('Enter')
      await waitForReplyCount(page, 2)
    } finally {
      await api.dispose()
    }
  })

  test('CHAT-11 @llm second send while a reply is generating', async ({ page }) => {
    const api = await apiContext()
    try {
      const first = (await cheapChatModels(api))[0]
      const name = uniqueName('chat-llm11')
      const chat = await apiJson<ChatDTO>(
        api, 'post', '/chats',
        { name, llm_target: { provider_uid: first.provider_uid, model_uid: first.model_uid } }, 201,
      )
      createdChatIds.add(chat.id)

      await login(page)
      await page.goto(`/chat/${chat.id}`)
      await expect(page.getByRole('heading', { name })).toBeVisible()

      const input = page.getByPlaceholder('Ask anything...')
      await input.fill('Reply with the single word: pong')
      await input.press('Enter')

      // Generation is now in flight (thinking indicator visible).
      await expect(page.locator('.thinking-dot').first()).toBeVisible({ timeout: 30_000 })

      // [API] Race a second POST while generating ⇒ 409 generation_in_progress.
      const res = await api.post(u(`/chats/${chat.id}/messages`), { data: { content: 'racing second send while generating' } })
      expect(res.status()).toBe(409)
      expect((await res.json()).code).toBe('generation_in_progress')

      // UI guard: the send button is DISABLED while a reply is generating —
      // ChatPanel.canSend is false when `thinking` is true, so a concurrent send
      // is blocked in the UI too (reinforcing the server-side 409 above). The
      // input itself stays enabled (its disable gate is model-unavailability only),
      // so a draft can be typed but not sent while thinking.
      await expect(page.locator('.thinking-dot').first()).toBeVisible()
      await expect(input).toBeEnabled()
      await input.fill('draft while generating')
      const sendBtn = page.locator('button', { has: page.locator('.pi-send') }).first()
      await expect(sendBtn).toBeDisabled()
      await input.fill('') // do not actually send a second UI message

      // Let the first generation settle so cleanup is clean (best-effort).
      await waitForReplyCount(page, 1).catch(() => {})
    } finally {
      await api.dispose()
    }
  })
})
