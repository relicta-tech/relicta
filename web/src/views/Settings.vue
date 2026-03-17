<script setup lang="ts">
import { ref, computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'

const authStore = useAuthStore()
const { theme, setTheme } = useTheme()

const editingName = ref(false)
const newName = ref(authStore.userName)
const editingKey = ref(false)
const newApiKey = ref('')
const showApiKey = ref(false)

function saveName() {
  if (newName.value.trim()) {
    authStore.setUserName(newName.value.trim())
  }
  editingName.value = false
}

function saveApiKey() {
  if (newApiKey.value.trim()) {
    authStore.setApiKey(newApiKey.value.trim())
    newApiKey.value = ''
  }
  editingKey.value = false
}

function clearApiKey() {
  if (confirm('Are you sure you want to remove your API key?')) {
    authStore.logout()
  }
}

const maskedApiKey = computed(() => {
  if (!authStore.apiKey) return ''
  if (showApiKey.value) return authStore.apiKey
  return authStore.apiKey.substring(0, 4) + '••••••••' + authStore.apiKey.substring(authStore.apiKey.length - 4)
})
</script>

<template>
  <div class="mx-auto max-w-2xl space-y-6">
    <!-- Profile -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Profile</h2>
        <p class="card-description">Your display name for audit trails</p>
      </div>
      <div class="card-content">
        <div v-if="editingName" class="flex gap-2">
          <input
            v-model="newName"
            type="text"
            class="input flex-1"
            placeholder="Your name"
            @keyup.enter="saveName"
          />
          <button @click="saveName" class="btn-primary btn-sm">Save</button>
          <button @click="editingName = false" class="btn-ghost btn-sm">Cancel</button>
        </div>
        <div v-else class="flex items-center justify-between">
          <div>
            <div class="font-medium">{{ authStore.userName }}</div>
            <div class="text-sm text-muted-foreground">Display name</div>
          </div>
          <button @click="editingName = true; newName = authStore.userName" class="btn-outline btn-sm">
            Edit
          </button>
        </div>
      </div>
    </div>

    <!-- API Key -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">API Key</h2>
        <p class="card-description">Authentication for dashboard access</p>
      </div>
      <div class="card-content">
        <div v-if="editingKey" class="space-y-4">
          <input
            v-model="newApiKey"
            type="password"
            class="input"
            placeholder="Enter new API key"
          />
          <div class="flex gap-2">
            <button @click="saveApiKey" class="btn-primary btn-sm">Save</button>
            <button @click="editingKey = false" class="btn-ghost btn-sm">Cancel</button>
          </div>
        </div>
        <div v-else class="space-y-4">
          <div v-if="authStore.apiKey" class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <code class="rounded bg-muted px-2 py-1 text-sm">{{ maskedApiKey }}</code>
              <button @click="showApiKey = !showApiKey" class="btn-ghost btn-icon btn-sm">
                <svg v-if="showApiKey" class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                </svg>
                <svg v-else class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </svg>
              </button>
            </div>
            <div class="flex gap-2">
              <button @click="editingKey = true" class="btn-outline btn-sm">Change</button>
              <button @click="clearApiKey" class="btn-destructive btn-sm">Remove</button>
            </div>
          </div>
          <div v-else class="flex items-center justify-between rounded-lg border border-dashed p-4">
            <div class="text-sm text-muted-foreground">No API key configured</div>
            <button @click="editingKey = true" class="btn-primary btn-sm">Add API Key</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Appearance -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Appearance</h2>
        <p class="card-description">Customize the dashboard theme</p>
      </div>
      <div class="card-content">
        <div class="flex gap-2">
          <button
            @click="setTheme('light')"
            :class="[
              'flex flex-1 flex-col items-center gap-2 rounded-lg border p-4 transition-colors',
              theme === 'light' ? 'border-primary bg-primary/5' : 'hover:bg-muted',
            ]"
          >
            <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
            </svg>
            <span class="text-sm font-medium">Light</span>
          </button>
          <button
            @click="setTheme('dark')"
            :class="[
              'flex flex-1 flex-col items-center gap-2 rounded-lg border p-4 transition-colors',
              theme === 'dark' ? 'border-primary bg-primary/5' : 'hover:bg-muted',
            ]"
          >
            <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
            </svg>
            <span class="text-sm font-medium">Dark</span>
          </button>
          <button
            @click="setTheme('system')"
            :class="[
              'flex flex-1 flex-col items-center gap-2 rounded-lg border p-4 transition-colors',
              theme === 'system' ? 'border-primary bg-primary/5' : 'hover:bg-muted',
            ]"
          >
            <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
            </svg>
            <span class="text-sm font-medium">System</span>
          </button>
        </div>
      </div>
    </div>

    <!-- About -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">About</h2>
      </div>
      <div class="card-content">
        <div class="space-y-2 text-sm">
          <div class="flex justify-between">
            <span class="text-muted-foreground">Application</span>
            <span>Relicta Dashboard</span>
          </div>
          <div class="flex justify-between">
            <span class="text-muted-foreground">Documentation</span>
            <a href="https://github.com/relicta-tech/relicta" target="_blank" class="text-primary hover:underline">
              GitHub
            </a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
