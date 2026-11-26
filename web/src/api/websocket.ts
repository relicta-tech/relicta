import { ref, onUnmounted, type Ref } from 'vue'
import type { WebSocketMessage, WebSocketEventType } from '@/types/api'
import { getDashboardAuthHeaders } from '@/api/client'

export interface UseWebSocketOptions {
  autoConnect?: boolean
  reconnect?: boolean
  reconnectInterval?: number
  maxReconnectAttempts?: number
}

export interface UseWebSocketReturn {
  status: Ref<'connecting' | 'connected' | 'disconnected' | 'error'>
  reconnectCount: Ref<number>
  maxReconnects: number
  lastMessage: Ref<WebSocketMessage | null>
  connect: () => void
  disconnect: () => void
  subscribe: (
    eventType: WebSocketEventType | WebSocketEventType[],
    handler: (message: WebSocketMessage) => void
  ) => () => void
}

export function useWebSocket(options: UseWebSocketOptions = {}): UseWebSocketReturn {
  const {
    autoConnect = true,
    reconnect = true,
    reconnectInterval = 3000,
    maxReconnectAttempts = 5,
  } = options

  const status = ref<'connecting' | 'connected' | 'disconnected' | 'error'>('disconnected')
  const lastMessage = ref<WebSocketMessage | null>(null)
  const reconnectCount = ref(0)

  let ws: WebSocket | null = null
  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  let connectInProgress = false
  let connectGeneration = 0
  const handlers = new Map<string, Set<(message: WebSocketMessage) => void>>()

  async function getWebSocketUrl(): Promise<string> {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const headers = await getDashboardAuthHeaders()
    const response = await fetch('/api/v1/ws/ticket', {
      method: 'POST',
      headers,
      cache: 'no-store',
    })
    if (!response.ok) {
      throw new Error(`WebSocket ticket request failed (${response.status})`)
    }
    const { ticket } = await response.json() as { ticket?: string }
    if (!ticket) throw new Error('WebSocket ticket response was empty')
    return `${protocol}//${host}/api/v1/ws?ticket=${encodeURIComponent(ticket)}`
  }

  async function connect() {
    if (ws?.readyState === WebSocket.OPEN || ws?.readyState === WebSocket.CONNECTING) {
      return
    }
    if (connectInProgress) return

    connectInProgress = true
    const generation = ++connectGeneration
    status.value = 'connecting'

    try {
      const url = await getWebSocketUrl()
      if (generation !== connectGeneration) return
      ws = new WebSocket(url)

      ws.onopen = () => {
        if (generation !== connectGeneration) return
        status.value = 'connected'
        reconnectCount.value = 0
      }

      ws.onclose = () => {
        if (generation !== connectGeneration) return
        status.value = 'disconnected'
        ws = null

        if (reconnect && reconnectCount.value < maxReconnectAttempts) {
          reconnectCount.value++
          reconnectTimeout = setTimeout(() => { void connect() }, reconnectInterval)
        }
      }

      ws.onerror = () => {
        if (generation !== connectGeneration) return
        status.value = 'error'
      }

      ws.onmessage = (event) => {
        if (generation !== connectGeneration) return
        try {
          const message: WebSocketMessage = JSON.parse(event.data)
          lastMessage.value = message

          // Dispatch to type-specific handlers
          const typeHandlers = handlers.get(message.type)
          if (typeHandlers) {
            typeHandlers.forEach((handler) => handler(message))
          }

          // Dispatch to wildcard handlers
          const wildcardHandlers = handlers.get('*')
          if (wildcardHandlers) {
            wildcardHandlers.forEach((handler) => handler(message))
          }
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error)
        }
      }
    } catch (error) {
      if (generation === connectGeneration) {
        status.value = 'error'
        if (reconnect && reconnectCount.value < maxReconnectAttempts) {
          reconnectCount.value++
          reconnectTimeout = setTimeout(() => { void connect() }, reconnectInterval)
        }
      }
      console.error('Failed to establish WebSocket connection:', error)
    } finally {
      if (generation === connectGeneration) connectInProgress = false
    }
  }

  function disconnect() {
    connectGeneration++
    connectInProgress = false
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
    reconnectCount.value = maxReconnectAttempts // Prevent auto-reconnect

    if (ws) {
      ws.close()
      ws = null
    }
    status.value = 'disconnected'
  }

  function subscribe(
    eventType: WebSocketEventType | WebSocketEventType[],
    handler: (message: WebSocketMessage) => void
  ): () => void {
    const types = Array.isArray(eventType) ? eventType : [eventType]

    types.forEach((type) => {
      if (!handlers.has(type)) {
        handlers.set(type, new Set())
      }
      handlers.get(type)!.add(handler)
    })

    // Return unsubscribe function
    return () => {
      types.forEach((type) => {
        handlers.get(type)?.delete(handler)
      })
    }
  }

  // Auto-connect if enabled
  if (autoConnect) {
    void connect()
  }

  // Cleanup on unmount
  onUnmounted(() => {
    disconnect()
  })

  return {
    status,
    reconnectCount,
    maxReconnects: maxReconnectAttempts,
    lastMessage,
    connect,
    disconnect,
    subscribe,
  }
}
