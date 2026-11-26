import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import { revokeDashboardSession } from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const apiKey = ref<string | null>(localStorage.getItem('relicta_api_key'))
  const accessToken = ref<string | null>(sessionStorage.getItem('relicta_access_token'))
  const refreshToken = ref<string | null>(sessionStorage.getItem('relicta_refresh_token'))
  const expiresAt = ref<string | null>(sessionStorage.getItem('relicta_expires_at'))
  const userName = ref<string>(localStorage.getItem('relicta_user_name') || 'Anonymous')

  const isAuthenticated = computed(() => !!apiKey.value || !!accessToken.value)

  function revokeCurrentSession() {
    const currentAccessToken = sessionStorage.getItem('relicta_access_token')
    const currentRefreshToken = sessionStorage.getItem('relicta_refresh_token')
    if (currentAccessToken && currentRefreshToken) {
      void revokeDashboardSession(currentAccessToken, currentRefreshToken).catch(() => undefined)
    }
  }

  function setApiKey(key: string) {
    revokeCurrentSession()
    clearSession()
    apiKey.value = key
    localStorage.setItem('relicta_api_key', key)
  }

  function setSession(tokens: { access_token: string; refresh_token: string; expires_at: string }) {
    localStorage.removeItem('relicta_api_key')
    apiKey.value = null
    accessToken.value = tokens.access_token
    refreshToken.value = tokens.refresh_token
    expiresAt.value = tokens.expires_at
    sessionStorage.setItem('relicta_access_token', tokens.access_token)
    sessionStorage.setItem('relicta_refresh_token', tokens.refresh_token)
    sessionStorage.setItem('relicta_expires_at', tokens.expires_at)
  }

  function clearSession() {
    accessToken.value = null
    refreshToken.value = null
    expiresAt.value = null
    sessionStorage.removeItem('relicta_access_token')
    sessionStorage.removeItem('relicta_refresh_token')
    sessionStorage.removeItem('relicta_expires_at')
  }

  function setUserName(name: string) {
    userName.value = name
    localStorage.setItem('relicta_user_name', name)
  }

  function logout() {
    revokeCurrentSession()
    apiKey.value = null
    localStorage.removeItem('relicta_api_key')
    clearSession()
  }

  return {
    apiKey,
    accessToken,
    refreshToken,
    expiresAt,
    userName,
    isAuthenticated,
    setApiKey,
    setSession,
    clearSession,
    setUserName,
    logout,
  }
})
