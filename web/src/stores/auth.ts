import { ref, computed } from 'vue'
import { defineStore } from 'pinia'

export const useAuthStore = defineStore('auth', () => {
  const apiKey = ref<string | null>(localStorage.getItem('relicta_api_key'))
  const userName = ref<string>(localStorage.getItem('relicta_user_name') || 'Anonymous')

  const isAuthenticated = computed(() => !!apiKey.value)

  function setApiKey(key: string) {
    apiKey.value = key
    localStorage.setItem('relicta_api_key', key)
  }

  function setUserName(name: string) {
    userName.value = name
    localStorage.setItem('relicta_user_name', name)
  }

  function logout() {
    apiKey.value = null
    localStorage.removeItem('relicta_api_key')
  }

  return {
    apiKey,
    userName,
    isAuthenticated,
    setApiKey,
    setUserName,
    logout,
  }
})
