import { ref, onUnmounted } from 'vue'

export interface UseWebSocketOptions {
  onMessage?: (data: unknown) => void
  onOpen?: () => void
  onClose?: () => void
  onError?: (error: Event) => void
  maxRetries?: number
  baseDelay?: number
}

export type ConnectionStatus = 'connected' | 'disconnected' | 'reconnecting'

export function useWebSocket(url: string, options: UseWebSocketOptions = {}) {
  const connected = ref(false)
  const connectionStatus = ref<ConnectionStatus>('disconnected')
  const retryCount = ref(0)
  const maxRetries = options.maxRetries ?? 10
  const baseDelay = options.baseDelay ?? 1000

  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let intentionalClose = false

  function connect() {
    intentionalClose = false
    ws = new WebSocket(url)

    ws.onopen = () => {
      connected.value = true
      connectionStatus.value = 'connected'
      retryCount.value = 0
      options.onOpen?.()
    }

    ws.onmessage = (event) => {
      try {
        const parts = (event.data as string).split('\n').filter(Boolean)
        for (const part of parts) {
          const data = JSON.parse(part)
          options.onMessage?.(data)
        }
      } catch {
        // Non-JSON message, ignore
      }
    }

    ws.onclose = () => {
      connected.value = false
      options.onClose?.()
      if (!intentionalClose && retryCount.value < maxRetries) {
        const delay = Math.min(baseDelay * Math.pow(2, retryCount.value), 30000)
        retryCount.value++
        connectionStatus.value = 'reconnecting'
        reconnectTimer = setTimeout(connect, delay)
      } else {
        connectionStatus.value = 'disconnected'
      }
    }

    ws.onerror = (error) => {
      options.onError?.(error)
    }
  }

  function disconnect() {
    intentionalClose = true
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    ws?.close()
    ws = null
    connected.value = false
    connectionStatus.value = 'disconnected'
  }

  onUnmounted(disconnect)

  return { connected, connectionStatus, retryCount, connect, disconnect }
}
