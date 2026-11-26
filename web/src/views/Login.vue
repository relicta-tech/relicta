<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { beginOIDCLogin, exchangeOIDCLoginTicket, getAuthMode, loginWithPassword, verifyApiKey } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const mode = ref('')
const loading = ref(false)
const error = ref('')
const username = ref('')
const password = ref('')
const apiKey = ref('')

onMounted(async () => {
  try {
    const fragment = new URLSearchParams(window.location.hash.slice(1))
    const oidcTicket = fragment.get('oidc_ticket')
    if (oidcTicket) {
      // Remove the one-use ticket from browser history before exchanging it.
      window.history.replaceState(window.history.state, '', `${window.location.pathname}${window.location.search}`)
      mode.value = 'oidc'
      loading.value = true
      const tokens = await exchangeOIDCLoginTicket(oidcTicket)
      authStore.setSession(tokens)
      authStore.setUserName(tokens.user_name || 'Signed in user')
      const requested = sessionStorage.getItem('relicta_oidc_redirect') || '/'
      sessionStorage.removeItem('relicta_oidc_redirect')
      const destination = requested.startsWith('/') && !requested.startsWith('//') ? requested : '/'
      await router.replace(destination)
      return
    }
    mode.value = await getAuthMode()
    if (mode.value === 'none') await continueToDashboard()
  } catch (err) {
    loading.value = false
    sessionStorage.removeItem('relicta_oidc_redirect')
    error.value = err instanceof Error ? err.message : 'Could not load sign-in settings'
  }
})

async function continueToDashboard() {
  const requested = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
  const destination = requested.startsWith('/') && !requested.startsWith('//') ? requested : '/'
  await router.replace(destination)
}

async function signInWithApiKey() {
  loading.value = true
  error.value = ''
  try {
    await verifyApiKey(apiKey.value)
    authStore.setApiKey(apiKey.value)
    await continueToDashboard()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Sign-in failed'
  } finally {
    loading.value = false
  }
}

async function signInWithPassword() {
  loading.value = true
  error.value = ''
  try {
    const tokens = await loginWithPassword(username.value, password.value)
    authStore.setSession(tokens)
    authStore.setUserName(username.value)
    await continueToDashboard()
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Sign-in failed'
  } finally {
    loading.value = false
  }
}

function signInWithOIDC() {
  error.value = ''
  const requested = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
  sessionStorage.setItem('relicta_oidc_redirect', requested.startsWith('/') && !requested.startsWith('//') ? requested : '/')
  beginOIDCLogin()
}
</script>

<template>
  <main class="flex min-h-screen items-center justify-center bg-background px-4 py-10">
    <section class="card w-full max-w-md">
      <div class="card-header">
        <div class="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-primary text-lg font-bold text-primary-foreground">
          R
        </div>
        <h1 class="card-title">
          Sign in to Relicta
        </h1>
        <p class="card-description">
          Use the credentials configured by your dashboard administrator.
        </p>
      </div>
      <div class="card-content space-y-4">
        <p
          v-if="error"
          role="alert"
          class="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive"
        >
          {{ error }}
        </p>

        <form
          v-if="mode === 'api_key'"
          class="space-y-4"
          @submit.prevent="signInWithApiKey"
        >
          <label class="block space-y-1 text-sm font-medium">
            <span>Dashboard API key</span>
            <input
              v-model="apiKey"
              class="input"
              type="password"
              autocomplete="current-password"
              required
              autofocus
            >
          </label>
          <button
            class="btn-primary w-full"
            type="submit"
            :disabled="loading || !apiKey"
          >
            {{ loading ? 'Connecting…' : 'Continue' }}
          </button>
        </form>

        <form
          v-else-if="mode === 'session'"
          class="space-y-4"
          @submit.prevent="signInWithPassword"
        >
          <label class="block space-y-1 text-sm font-medium">
            <span>Username</span>
            <input
              v-model="username"
              class="input"
              type="text"
              autocomplete="username"
              required
              autofocus
            >
          </label>
          <label class="block space-y-1 text-sm font-medium">
            <span>Password</span>
            <input
              v-model="password"
              class="input"
              type="password"
              autocomplete="current-password"
              required
            >
          </label>
          <button
            class="btn-primary w-full"
            type="submit"
            :disabled="loading || !username || !password"
          >
            {{ loading ? 'Signing in…' : 'Sign in' }}
          </button>
        </form>

        <div
          v-else-if="mode === 'oidc'"
          class="space-y-3 text-sm text-muted-foreground"
        >
          <p>This dashboard uses your organization’s identity provider.</p>
          <button
            class="btn-primary w-full"
            type="button"
            :disabled="loading"
            @click="signInWithOIDC"
          >
            Continue with single sign-on
          </button>
        </div>

        <p
          v-else-if="!mode"
          class="text-sm text-muted-foreground"
        >
          Loading sign-in settings…
        </p>
        <p
          v-else
          role="alert"
          class="text-sm text-destructive"
        >
          Unsupported dashboard authentication mode: {{ mode }}
        </p>
      </div>
    </section>
  </main>
</template>
