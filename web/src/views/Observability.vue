<script setup lang="ts">
import { ref, onMounted } from 'vue'
import * as api from '@/api/client'
import type {
  DeploymentHealth,
  ObservabilityProvider,
  IncidentCorrelation,
} from '@/types/api'

const loading = ref(true)
const error = ref<string | null>(null)

const overallHealth = ref<'healthy' | 'degraded' | 'unhealthy'>('healthy')
const deployments = ref<DeploymentHealth[]>([])
const providers = ref<ObservabilityProvider[]>([])
const correlations = ref<IncidentCorrelation[]>([])

onMounted(() => {
  loadData()
})

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const [healthRes, providersRes, correlationsRes] = await Promise.all([
      api.getObservabilityHealth(),
      api.getObservabilityProviders(),
      api.getObservabilityCorrelations(),
    ])
    overallHealth.value = healthRes.overall
    deployments.value = healthRes.deployments
    providers.value = providersRes.providers
    correlations.value = correlationsRes.correlations
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load observability data'
    console.error('Failed to load observability data:', err)
  } finally {
    loading.value = false
  }
}

function getOverallHealthColor(): string {
  switch (overallHealth.value) {
    case 'healthy': return 'text-green-500'
    case 'degraded': return 'text-yellow-500'
    case 'unhealthy': return 'text-red-500'
    default: return 'text-gray-600 dark:text-gray-400'
  }
}

function getOverallHealthBg(): string {
  switch (overallHealth.value) {
    case 'healthy': return 'bg-green-100 dark:bg-green-900/30'
    case 'degraded': return 'bg-yellow-100 dark:bg-yellow-900/30'
    case 'unhealthy': return 'bg-red-100 dark:bg-red-900/30'
    default: return 'bg-gray-100 dark:bg-gray-800'
  }
}

function getDeploymentStatusColor(status: string): string {
  switch (status) {
    case 'healthy': return 'bg-green-500'
    case 'degraded': return 'bg-yellow-500'
    case 'unhealthy': return 'bg-red-500'
    default: return 'bg-gray-500'
  }
}

function getDeploymentBorderClass(dep: DeploymentHealth): string {
  if (dep.error_rate > dep.thresholds.error_rate_max) return 'border-red-300 dark:border-red-700'
  if (dep.latency_p99_ms > dep.thresholds.latency_p99_max_ms) return 'border-yellow-300 dark:border-yellow-700'
  return ''
}

function getProviderStatusIcon(status: string): string {
  switch (status) {
    case 'connected': return 'text-green-500'
    case 'disconnected': return 'text-gray-600 dark:text-gray-400'
    case 'error': return 'text-red-500'
    default: return 'text-gray-600 dark:text-gray-400'
  }
}

function getConfidenceColor(confidence: number): string {
  if (confidence >= 0.8) return 'bg-red-500'
  if (confidence >= 0.5) return 'bg-yellow-500'
  return 'bg-blue-500'
}

function getCorrelationStatusClass(status: string): string {
  switch (status) {
    case 'open': return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
    case 'investigating': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
    case 'resolved': return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    default: return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400'
  }
}

function formatDateTime(dateString: string): string {
  return new Date(dateString).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

// Correlation timeline helpers
function getTimelinePosition(dateString: string): number {
  if (correlations.value.length === 0) return 0
  const dates = correlations.value.map(c => new Date(c.detected_at).getTime())
  const min = Math.min(...dates)
  const max = Math.max(...dates)
  const range = max - min || 1
  return ((new Date(dateString).getTime() - min) / range) * 100
}
</script>

<template>
  <div class="space-y-6">
    <!-- Overall health banner -->
    <div :class="['card', getOverallHealthBg()]">
      <div class="card-content flex items-center justify-between pt-6">
        <div class="flex items-center gap-4">
          <svg
            class="h-8 w-8 shrink-0"
            :class="getOverallHealthColor()"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          <div>
            <h2
              class="text-lg font-semibold"
              :class="getOverallHealthColor()"
            >
              System {{ overallHealth }}
            </h2>
            <p class="text-sm text-muted-foreground">
              {{ deployments.length }} deployments monitored, {{ providers.length }} providers connected
            </p>
          </div>
        </div>
        <button
          class="btn-ghost btn-sm"
          @click="loadData"
        >
          Refresh
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div
      v-if="loading"
      class="flex items-center justify-center py-12"
    >
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent" />
    </div>

    <!-- Error -->
    <div
      v-else-if="error"
      class="card border-red-200 dark:border-red-800"
    >
      <div class="card-content pt-6">
        <p class="text-red-600 dark:text-red-300">
          {{ error }}
        </p>
        <button
          class="btn-primary btn-sm mt-2"
          @click="loadData"
        >
          Retry
        </button>
      </div>
    </div>

    <template v-else>
      <!-- Deployment health cards -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">
            Deployment Health
          </h2>
          <p class="card-description">
            Health status with threshold indicators for recent deployments
          </p>
        </div>
        <div class="card-content">
          <div
            v-if="deployments.length === 0"
            class="py-8 text-center text-muted-foreground"
          >
            No active deployments being monitored
          </div>
          <div
            v-else
            class="grid gap-4 md:grid-cols-2"
          >
            <div
              v-for="dep in deployments"
              :key="dep.release_id"
              :class="['rounded-lg border p-4', getDeploymentBorderClass(dep)]"
            >
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span :class="['h-2.5 w-2.5 rounded-full', getDeploymentStatusColor(dep.status)]" />
                  <span class="font-medium">{{ dep.version }}</span>
                </div>
                <span class="text-xs text-muted-foreground">{{ formatDateTime(dep.deployed_at) }}</span>
              </div>
              <div class="mt-3 grid grid-cols-2 gap-4">
                <div>
                  <div class="text-xs text-muted-foreground">
                    Error Rate
                  </div>
                  <div class="flex items-center gap-2">
                    <span
                      class="text-sm font-medium"
                      :class="dep.error_rate > dep.thresholds.error_rate_max ? 'text-red-600' : 'text-foreground'"
                    >
                      {{ (dep.error_rate * 100).toFixed(2) }}%
                    </span>
                    <span class="text-xs text-muted-foreground">
                      / {{ (dep.thresholds.error_rate_max * 100).toFixed(1) }}%
                    </span>
                  </div>
                  <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-muted">
                    <div
                      class="h-full transition-all"
                      :class="dep.error_rate > dep.thresholds.error_rate_max ? 'bg-red-500' : 'bg-green-500'"
                      :style="{ width: `${Math.min((dep.error_rate / dep.thresholds.error_rate_max) * 100, 100)}%` }"
                    />
                  </div>
                </div>
                <div>
                  <div class="text-xs text-muted-foreground">
                    P99 Latency
                  </div>
                  <div class="flex items-center gap-2">
                    <span
                      class="text-sm font-medium"
                      :class="dep.latency_p99_ms > dep.thresholds.latency_p99_max_ms ? 'text-yellow-600' : 'text-foreground'"
                    >
                      {{ dep.latency_p99_ms }}ms
                    </span>
                    <span class="text-xs text-muted-foreground">
                      / {{ dep.thresholds.latency_p99_max_ms }}ms
                    </span>
                  </div>
                  <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-muted">
                    <div
                      class="h-full transition-all"
                      :class="dep.latency_p99_ms > dep.thresholds.latency_p99_max_ms ? 'bg-yellow-500' : 'bg-green-500'"
                      :style="{ width: `${Math.min((dep.latency_p99_ms / dep.thresholds.latency_p99_max_ms) * 100, 100)}%` }"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="grid gap-6 lg:grid-cols-2">
        <!-- Provider status -->
        <div class="card">
          <div class="card-header">
            <h2 class="card-title">
              Provider Connections
            </h2>
            <p class="card-description">
              Observability data source status
            </p>
          </div>
          <div class="card-content">
            <div
              v-if="providers.length === 0"
              class="py-8 text-center text-muted-foreground"
            >
              No providers configured
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="provider in providers"
                :key="provider.name"
                class="flex items-center justify-between rounded-lg border p-3"
              >
                <div class="flex items-center gap-3">
                  <svg
                    class="h-5 w-5 shrink-0"
                    :class="getProviderStatusIcon(provider.status)"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      v-if="provider.status === 'connected'"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M5 13l4 4L19 7"
                    />
                    <path
                      v-else-if="provider.status === 'error'"
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.834-1.964-.834-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"
                    />
                    <path
                      v-else
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M20 12H4"
                    />
                  </svg>
                  <div>
                    <div class="font-medium">
                      {{ provider.name }}
                    </div>
                    <div class="text-xs text-muted-foreground">
                      {{ provider.type }}
                    </div>
                  </div>
                </div>
                <div class="text-right">
                  <div
                    class="text-sm capitalize"
                    :class="getProviderStatusIcon(provider.status)"
                  >
                    {{ provider.status }}
                  </div>
                  <div class="text-xs text-muted-foreground">
                    {{ formatDateTime(provider.last_check) }}
                  </div>
                  <div
                    v-if="provider.error_message"
                    class="mt-1 text-xs text-red-500"
                  >
                    {{ provider.error_message }}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Incident-release correlations -->
        <div class="card">
          <div class="card-header">
            <h2 class="card-title">
              Incident Correlations
            </h2>
            <p class="card-description">
              Recent incident-release correlations with confidence scores
            </p>
          </div>
          <div class="card-content">
            <div
              v-if="correlations.length === 0"
              class="py-8 text-center text-muted-foreground"
            >
              No incident correlations detected
            </div>
            <div
              v-else
              class="space-y-3"
            >
              <div
                v-for="corr in correlations"
                :key="corr.incident_id"
                class="rounded-lg border p-3"
              >
                <div class="flex items-start justify-between">
                  <div>
                    <div class="flex items-center gap-2">
                      <span :class="['badge', getCorrelationStatusClass(corr.status)]">
                        {{ corr.status }}
                      </span>
                      <span class="font-medium">{{ corr.version }}</span>
                    </div>
                    <p class="mt-1 text-sm text-muted-foreground">
                      {{ corr.description }}
                    </p>
                  </div>
                  <div class="text-right">
                    <div class="text-xs text-muted-foreground">
                      Confidence
                    </div>
                    <div class="flex items-center gap-1">
                      <div class="h-2 w-12 overflow-hidden rounded-full bg-muted">
                        <div
                          :class="['h-full', getConfidenceColor(corr.confidence)]"
                          :style="{ width: `${corr.confidence * 100}%` }"
                        />
                      </div>
                      <span class="text-xs font-medium">{{ (corr.confidence * 100).toFixed(0) }}%</span>
                    </div>
                  </div>
                </div>
                <div class="mt-2 text-xs text-muted-foreground">
                  Detected {{ formatDateTime(corr.detected_at) }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Correlation timeline -->
      <div
        v-if="correlations.length > 1"
        class="card"
      >
        <div class="card-header">
          <h2 class="card-title">
            Release Correlation Timeline
          </h2>
          <p class="card-description">
            Incident-release correlations plotted over time
          </p>
        </div>
        <div class="card-content">
          <div class="relative h-20 rounded-lg border bg-muted/30 px-4 py-2">
            <!-- Timeline bar -->
            <div class="absolute inset-x-4 top-1/2 h-0.5 -translate-y-1/2 bg-border" />
            <!-- Timeline points -->
            <div
              v-for="corr in correlations"
              :key="corr.incident_id"
              class="absolute top-1/2 -translate-x-1/2 -translate-y-1/2"
              :style="{ left: `${Math.max(4, Math.min(96, getTimelinePosition(corr.detected_at)))}%` }"
            >
              <div
                :class="[
                  'h-4 w-4 rounded-full border-2 border-card',
                  corr.status === 'open' ? 'bg-red-500' : corr.status === 'investigating' ? 'bg-yellow-500' : 'bg-green-500',
                ]"
                :title="`${corr.version} - ${corr.description} (${(corr.confidence * 100).toFixed(0)}% confidence)`"
              />
              <div class="mt-2 whitespace-nowrap text-center text-[10px] text-muted-foreground">
                {{ corr.version }}
              </div>
            </div>
          </div>
          <div class="mt-1 flex justify-between text-xs text-muted-foreground">
            <span>{{ formatDate(correlations[correlations.length - 1]?.detected_at) }}</span>
            <span>{{ formatDate(correlations[0]?.detected_at) }}</span>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
