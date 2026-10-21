import { request, type APIRequestContext } from '@playwright/test'

// Direct-to-core API base (bypasses the Vite proxy for speed/robustness).
export const API_BASE = 'http://localhost:8080/api/v1/intellisim'
const AUTH_TOKEN_URL = 'http://localhost:9090/token'

let cachedToken: { value: string; fetchedAt: number } | null = null

// Browserless bearer via the auth-mock client_credentials grant. The token
// carries the active persona's claims (MOCK_ACTIVE_USER at mock startup).
export async function getApiToken(): Promise<string> {
  if (cachedToken && Date.now() - cachedToken.fetchedAt < 30 * 60_000) return cachedToken.value
  const ctx = await request.newContext()
  try {
    const res = await ctx.post(AUTH_TOKEN_URL, {
      form: { grant_type: 'client_credentials', client_id: 'e2e', scope: 'openid' },
    })
    if (!res.ok()) throw new Error(`auth-mock token grant failed: ${res.status()}`)
    const body = (await res.json()) as { access_token: string }
    cachedToken = { value: body.access_token, fetchedAt: Date.now() }
    return body.access_token
  } finally {
    await ctx.dispose()
  }
}

// Authenticated APIRequestContext for the core API. Callers own dispose().
// NOTE: no baseURL — Playwright resolves leading-slash paths against the host
// root, silently dropping the /api/v1/intellisim prefix. Always build URLs
// with u(path) (or use apiJson, which does it for you).
export async function apiContext(): Promise<APIRequestContext> {
  const token = await getApiToken()
  return request.newContext({
    extraHTTPHeaders: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
  })
}

// u('/models/123') → full core API URL.
export function u(path: string): string {
  return `${API_BASE}${path}`
}

// Poll `fn` until it returns a truthy value or `timeoutMs` elapses.
export async function pollUntil<T>(
  fn: () => Promise<T | null | undefined | false>,
  opts: { timeoutMs?: number; intervalMs?: number; label?: string } = {},
): Promise<T> {
  const { timeoutMs = 15_000, intervalMs = 500, label = 'condition' } = opts
  const deadline = Date.now() + timeoutMs
  let last: unknown
  while (Date.now() < deadline) {
    last = await fn()
    if (last) return last as T
    await new Promise((r) => setTimeout(r, intervalMs))
  }
  throw new Error(`pollUntil timed out after ${timeoutMs}ms waiting for ${label}`)
}

export interface ApiErrorBody {
  code: string
  message?: string
  fields?: Record<string, string>
}

// Convenience wrappers returning parsed JSON and asserting the status.
export async function apiJson<T = any>(
  api: APIRequestContext,
  method: 'get' | 'post' | 'put' | 'delete',
  path: string,
  body?: unknown,
  expectStatus?: number,
): Promise<T> {
  const res = await api[method](u(path), body !== undefined ? { data: body } : undefined)
  if (expectStatus !== undefined && res.status() !== expectStatus) {
    const text = await res.text().catch(() => '<unreadable>')
    throw new Error(`${method.toUpperCase()} ${path} → ${res.status()} (expected ${expectStatus}): ${text}`)
  }
  const text = await res.text()
  return text ? (JSON.parse(text) as T) : (undefined as T)
}
