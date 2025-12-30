import { ref, onUnmounted, type Ref } from 'vue'
import type { WebSocketMessage, WebSocketEventType } from '@/types/api'

export interface UseWebSocketOptions {
  autoConnect?: boolean
  reconnect?: boolean
  reconnectInterval?: number
  maxReconnectAttempts?: number
}

export interface UseWebSocketReturn {
  status: Ref<'connecting' | 'connected' | 'disconnected' | 'error'>
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

  let ws: WebSocket | null = null
  let reconnectAttempts = 0
  let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  const handlers = new Map<string, Set<(message: WebSocketMessage) => void>>()

  function getWebSocketUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const apiKey = localStorage.getItem('relicta_api_key')
    let url = `${protocol}//${host}/api/v1/ws`
    if (apiKey) {
      url += `?api_key=${encodeURIComponent(apiKey)}`
    }
    return url
  }

  function connect() {
    if (ws?.readyState === WebSocket.OPEN || ws?.readyState === WebSocket.CONNECTING) {
      return
    }

    status.value = 'connecting'

    try {
      ws = new WebSocket(getWebSocketUrl())

      ws.onopen = () => {
        status.value = 'connected'
        reconnectAttempts = 0
      }

      ws.onclose = () => {
        status.value = 'disconnected'
        ws = null

        if (reconnect && reconnectAttempts < maxReconnectAttempts) {
          reconnectAttempts++
          reconnectTimeout = setTimeout(connect, reconnectInterval)
        }
      }

      ws.onerror = () => {
        status.value = 'error'
      }

      ws.onmessage = (event) => {
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
      status.value = 'error'
      console.error('Failed to create WebSocket connection:', error)
    }
  }

  function disconnect() {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
    reconnectAttempts = maxReconnectAttempts // Prevent auto-reconnect

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
    connect()
  }

  // Cleanup on unmount
  onUnmounted(() => {
    disconnect()
  })

  return {
    status,
    lastMessage,
    connect,
    disconnect,
    subscribe,
  }
}
