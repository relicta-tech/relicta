<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useReleasesStore } from '@/stores/releases'
import { useAuthStore } from '@/stores/auth'
import { useWebSocket } from '@/api/websocket'
import { useTheme } from '@/composables/useTheme'
import { getHealth } from '@/api/client'

const route = useRoute()
const releasesStore = useReleasesStore()
const authStore = useAuthStore()
const { isDark, toggleTheme } = useTheme()

const sidebarOpen = ref(true)
const mobileMenuOpen = ref(false)
const healthStatus = ref<'healthy' | 'degraded' | 'unhealthy' | 'unknown'>('unknown')

// WebSocket connection
const { status: wsStatus, reconnectCount, maxReconnects, subscribe, connect: wsConnect } = useWebSocket()

// Subscribe to all events and delegate to stores
const unsubscribe = subscribe('*' as any, (message) => {
  releasesStore.handleWebSocketMessage(message)
})

// Check health on mount
onMounted(async () => {
  try {
    const health = await getHealth()
    healthStatus.value = health.status
  } catch {
    healthStatus.value = 'unhealthy'
  }

  // Fetch initial data
  releasesStore.fetchActiveRelease()
})

onUnmounted(() => {
  unsubscribe()
})

// Navigation items
const navItems = [
  { path: '/', name: 'Dashboard', icon: 'home' },
  { path: '/releases', name: 'Release Pipeline', icon: 'rocket' },
  { path: '/governance', name: 'Governance', icon: 'shield' },
  { path: '/team', name: 'Team Performance', icon: 'users' },
  { path: '/approvals', name: 'Approvals', icon: 'check-circle' },
  { path: '/audit', name: 'Audit Trail', icon: 'scroll' },
]

function isActive(path: string): boolean {
  if (path === '/') {
    return route.path === '/'
  }
  return route.path.startsWith(path)
}

function toggleSidebar() {
  sidebarOpen.value = !sidebarOpen.value
}

function getWsStatusColor(): string {
  switch (wsStatus.value) {
    case 'connected':
      return 'bg-green-500'
    case 'connecting':
      return 'bg-yellow-500 animate-pulse'
    case 'error':
      return 'bg-red-500'
    default:
      return 'bg-gray-500'
  }
}

function getWsStatusText(): string {
  if (wsStatus.value === 'connecting' && reconnectCount.value > 0) {
    return `reconnecting (${reconnectCount.value}/${maxReconnects})`
  }
  if (wsStatus.value === 'disconnected' && reconnectCount.value >= maxReconnects) {
    return 'disconnected (retry failed)'
  }
  return wsStatus.value
}

function handleReconnect(): void {
  reconnectCount.value = 0
  wsConnect()
}

function getHealthStatusColor(): string {
  switch (healthStatus.value) {
    case 'healthy':
      return 'text-green-500'
    case 'degraded':
      return 'text-yellow-500'
    case 'unhealthy':
      return 'text-red-500'
    default:
      return 'text-gray-500'
  }
}
</script>

<template>
  <div class="flex h-screen bg-background">
    <!-- Mobile menu overlay -->
    <div
      v-if="mobileMenuOpen"
      class="fixed inset-0 z-40 bg-black/50 lg:hidden"
      @click="mobileMenuOpen = false"
    ></div>

    <!-- Sidebar -->
    <aside
      :class="[
        'fixed inset-y-0 left-0 z-50 flex flex-col border-r bg-card transition-all duration-300 lg:relative lg:translate-x-0',
        sidebarOpen ? 'w-64' : 'w-16',
        mobileMenuOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0',
      ]"
    >
      <!-- Logo -->
      <div class="flex h-16 items-center justify-between border-b px-4">
        <RouterLink
          to="/"
          class="flex items-center gap-2"
          :class="{ 'justify-center': !sidebarOpen }"
          @click="mobileMenuOpen = false"
        >
          <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground font-bold">
            R
          </div>
          <span v-if="sidebarOpen" class="text-lg font-semibold">Relicta</span>
        </RouterLink>
        <div class="flex items-center gap-1">
          <!-- Close mobile menu -->
          <button
            @click="mobileMenuOpen = false"
            class="rounded-md p-1 hover:bg-muted lg:hidden"
          >
            <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
          <!-- Collapse sidebar (desktop only) -->
          <button
            v-if="sidebarOpen"
            @click="toggleSidebar"
            class="hidden rounded-md p-1 hover:bg-muted lg:block"
          >
            <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 19l-7-7 7-7m8 14l-7-7 7-7" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Navigation -->
      <nav class="flex-1 space-y-1 p-2">
        <RouterLink
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          :class="[
            'nav-link',
            isActive(item.path) && 'nav-link-active',
            !sidebarOpen && 'justify-center px-2',
          ]"
          :title="!sidebarOpen ? item.name : undefined"
          @click="mobileMenuOpen = false"
        >
          <!-- Icons -->
          <svg v-if="item.icon === 'home'" class="h-5 w-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
          </svg>
          <svg v-else-if="item.icon === 'rocket'" class="h-5 w-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
          <svg v-else-if="item.icon === 'shield'" class="h-5 w-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
          </svg>
          <svg v-else-if="item.icon === 'users'" class="h-5 w-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
          </svg>
          <svg v-else-if="item.icon === 'check-circle'" class="h-5 w-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <svg v-else-if="item.icon === 'scroll'" class="h-5 w-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <span v-if="sidebarOpen">{{ item.name }}</span>
        </RouterLink>
      </nav>

      <!-- Bottom section -->
      <div class="border-t p-2">
        <!-- Expand button when collapsed -->
        <button
          v-if="!sidebarOpen"
          @click="toggleSidebar"
          class="nav-link justify-center w-full"
        >
          <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 5l7 7-7 7M5 5l7 7-7 7" />
          </svg>
        </button>

        <!-- Settings link -->
        <RouterLink
          to="/settings"
          :class="[
            'nav-link',
            isActive('/settings') && 'nav-link-active',
            !sidebarOpen && 'justify-center px-2',
          ]"
          title="Settings"
          @click="mobileMenuOpen = false"
        >
          <svg class="h-5 w-5 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <span v-if="sidebarOpen">Settings</span>
        </RouterLink>

        <!-- Status indicators -->
        <div
          v-if="sidebarOpen"
          class="mt-2 space-y-2 px-3 py-2 text-xs text-muted-foreground"
        >
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <span :class="['h-2 w-2 rounded-full', getWsStatusColor()]"></span>
              <span>{{ getWsStatusText() }}</span>
            </div>
            <button
              v-if="wsStatus === 'disconnected' && reconnectCount >= maxReconnects"
              @click="handleReconnect"
              class="text-primary hover:underline"
            >
              retry
            </button>
          </div>
          <div class="flex items-center justify-between">
            <span>API</span>
            <span :class="getHealthStatusColor()">{{ healthStatus }}</span>
          </div>
        </div>
        <!-- Compact status when collapsed -->
        <div
          v-else
          class="mt-2 flex justify-center py-2"
          :title="`WS: ${getWsStatusText()}, API: ${healthStatus}`"
        >
          <span :class="['h-2 w-2 rounded-full', getWsStatusColor()]"></span>
        </div>
      </div>
    </aside>

    <!-- Main content -->
    <main class="flex-1 overflow-auto lg:ml-0">
      <!-- Header -->
      <header class="sticky top-0 z-10 flex h-16 items-center justify-between border-b bg-card px-4 lg:px-6">
        <div class="flex items-center gap-4">
          <!-- Mobile menu button -->
          <button
            @click="mobileMenuOpen = !mobileMenuOpen"
            class="btn-ghost btn-icon lg:hidden"
          >
            <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
          <h1 class="text-lg font-semibold lg:text-xl">{{ route.meta.title }}</h1>

          <!-- Active release indicator -->
          <div
            v-if="releasesStore.hasActiveRelease"
            class="flex items-center gap-2 rounded-full bg-blue-100 px-3 py-1 text-sm text-blue-800 dark:bg-blue-900/30 dark:text-blue-400"
          >
            <span class="h-2 w-2 animate-pulse rounded-full bg-blue-500"></span>
            <span>{{ releasesStore.activeRelease?.version_next }} in progress</span>
          </div>
        </div>

        <div class="flex items-center gap-4">
          <!-- User info -->
          <div class="text-sm text-muted-foreground">
            {{ authStore.userName }}
          </div>

          <!-- Theme toggle -->
          <button @click="toggleTheme" class="btn-ghost btn-icon" title="Toggle theme">
            <svg v-if="isDark" class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
            </svg>
            <svg v-else class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
            </svg>
          </button>
        </div>
      </header>

      <!-- Page content -->
      <div class="p-6">
        <RouterView />
      </div>
    </main>
  </div>
</template>
