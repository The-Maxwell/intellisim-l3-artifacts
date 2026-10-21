import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'

// ApiError carries the backend envelope: { code, message, fields? }.
// Legacy { error } / { message } bodies are normalized into it too.
export class ApiError extends Error {
  readonly code: string
  readonly status: number | undefined
  readonly fields: Record<string, string> | undefined

  constructor(
    message: string,
    code: string,
    status?: number,
    fields?: Record<string, string>
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.fields = fields
  }
}

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1/intellisim',
  headers: {
    'Content-Type': 'application/json',
  },
  // Sends are async now (202 + SSE) — no more minutes-long blocking calls.
  timeout: 30000,
})

apiClient.interceptors.request.use(async (config) => {
  const { useAuthStore } = await import('@/modules/auth/stores/authStore')
  const token = await useAuthStore().getValidAccessToken()
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

type RetriableConfig = InternalAxiosRequestConfig & { _retried?: boolean }

apiClient.interceptors.response.use(
  (response) => response,
  async (error: AxiosError) => {
    const original = error.config as RetriableConfig | undefined

    // One silent renew + retry on 401 before surrendering to the login page.
    if (error.response?.status === 401 && original && !original._retried) {
      original._retried = true
      const { useAuthStore } = await import('@/modules/auth/stores/authStore')
      const renewed = await useAuthStore().renew()
      if (renewed) {
        original.headers.Authorization = `Bearer ${renewed.access_token}`
        return apiClient.request(original)
      }
      window.location.assign(
        `/login?redirect=${encodeURIComponent(window.location.pathname + window.location.search)}`
      )
      // Navigation is in flight — suspend the promise chain so no downstream
      // error handler fires while the browser is unloading.
      return new Promise(() => {})
    }

    const data = error.response?.data as
      | { code?: string; message?: string; fields?: Record<string, string>; error?: string }
      | undefined
    const message = data?.message || data?.error || error.message || 'Request failed'
    return Promise.reject(new ApiError(message, data?.code ?? 'unknown', error.response?.status, data?.fields))
  }
)

export default apiClient
