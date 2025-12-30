import { ref, computed } from 'vue'
import { defineStore } from 'pinia'
import type { Release, ReleaseDetails, WebSocketMessage } from '@/types/api'
import * as api from '@/api/client'

export const useReleasesStore = defineStore('releases', () => {
  // State
  const releases = ref<Release[]>([])
  const activeRelease = ref<Release | null>(null)
  const selectedRelease = ref<ReleaseDetails | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  const totalReleases = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(20)

  // Getters
  const hasActiveRelease = computed(() => !!activeRelease.value)
  const totalPages = computed(() => Math.ceil(totalReleases.value / pageSize.value))

  // Actions
  async function fetchReleases(page = 1) {
    loading.value = true
    error.value = null
    try {
      const response = await api.listReleases(page, pageSize.value)
      releases.value = response.data
      totalReleases.value = response.total
      currentPage.value = response.page
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch releases'
    } finally {
      loading.value = false
    }
  }

  async function fetchActiveRelease() {
    try {
      activeRelease.value = await api.getActiveRelease()
    } catch (err) {
      // Active release may not exist
      activeRelease.value = null
    }
  }

  async function fetchRelease(id: string) {
    loading.value = true
    error.value = null
    try {
      selectedRelease.value = await api.getRelease(id)
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch release'
      selectedRelease.value = null
    } finally {
      loading.value = false
    }
  }

  function handleWebSocketMessage(message: WebSocketMessage) {
    const runId = message.payload.run_id as string | undefined

    switch (message.type) {
      case 'release.created':
        // Refresh releases list
        fetchReleases(currentPage.value)
        fetchActiveRelease()
        break

      case 'release.state_changed':
      case 'release.versioned':
      case 'release.approved':
      case 'release.published':
      case 'release.failed':
      case 'release.canceled':
        // Update the specific release if we have it
        if (runId) {
          const releaseIndex = releases.value.findIndex((r) => r.id === runId)
          if (releaseIndex >= 0) {
            fetchReleases(currentPage.value)
          }
          if (activeRelease.value?.id === runId) {
            fetchActiveRelease()
          }
          if (selectedRelease.value?.id === runId) {
            fetchRelease(runId)
          }
        }
        break

      case 'release.step_completed':
      case 'release.plugin_executed':
        // Update selected release if viewing
        if (runId && selectedRelease.value?.id === runId) {
          fetchRelease(runId)
        }
        break
    }
  }

  function clearSelection() {
    selectedRelease.value = null
  }

  return {
    // State
    releases,
    activeRelease,
    selectedRelease,
    loading,
    error,
    totalReleases,
    currentPage,
    pageSize,
    // Getters
    hasActiveRelease,
    totalPages,
    // Actions
    fetchReleases,
    fetchActiveRelease,
    fetchRelease,
    handleWebSocketMessage,
    clearSelection,
  }
})
