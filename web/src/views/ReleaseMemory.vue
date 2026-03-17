<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import * as api from '@/api/client'
import type {
  MemoryTrends,
  MemoryTrendPoint,
  ReleaseInsight,
  ReleaseOutcome,
} from '@/types/api'

const loading = ref(true)
const error = ref<string | null>(null)

// Trend data
const trends = ref<MemoryTrends | null>(null)
const insights = ref<ReleaseInsight[]>([])
const recentOutcomes = ref<ReleaseOutcome[]>([])

// Controls
const trendWindow = ref('30d')
const releaseIdInput = ref('')
const releaseInsights = ref<ReleaseInsight[]>([])
const loadingInsights = ref(false)

onMounted(() => {
  loadData()
})

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const [trendsRes, insightsRes] = await Promise.all([
      api.getMemoryTrends(trendWindow.value),
      api.getMemoryInsights(),
    ])
    trends.value = trendsRes
    insights.value = insightsRes.insights
    recentOutcomes.value = insightsRes.recent_outcomes
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load memory data'
    console.error('Failed to load memory data:', err)
  } finally {
    loading.value = false
  }
}

function changeWindow(w: string) {
  trendWindow.value = w
  loadData()
}

async function lookupReleaseInsights() {
  if (!releaseIdInput.value.trim()) return
  loadingInsights.value = true
  try {
    const res = await api.getMemoryInsights(releaseIdInput.value.trim())
    releaseInsights.value = res.insights
  } catch (err) {
    console.error('Failed to load release insights:', err)
    releaseInsights.value = []
  } finally {
    loadingInsights.value = false
  }
}

// Trend direction styling
function getTrendDirectionClass(): string {
  if (!trends.value) return ''
  switch (trends.value.trend_direction) {
    case 'improving': return 'text-green-600 dark:text-green-400'
    case 'declining': return 'text-red-600 dark:text-red-400'
    default: return 'text-yellow-600 dark:text-yellow-400'
  }
}

function getTrendDirectionIcon(): string {
  if (!trends.value) return ''
  switch (trends.value.trend_direction) {
    case 'improving': return 'up'
    case 'declining': return 'down'
    default: return 'flat'
  }
}

// Success rate trend line
const chartWidth = 500
const chartHeight = 100
const chartPadding = 8

function getSuccessRatePath(): string {
  if (!trends.value || trends.value.history.length < 2) return ''
  const history = trends.value.history
  const w = chartWidth - chartPadding * 2
  const h = chartHeight - chartPadding * 2

  const points = history.map((p, i) => {
    const x = chartPadding + (i / (history.length - 1)) * w
    const y = chartPadding + h - (p.success_rate / 100) * h
    return `${x},${y}`
  })
  return `M ${points.join(' L ')}`
}

function getSuccessRateArea(): string {
  const line = getSuccessRatePath()
  if (!line) return ''
  return `${line} L ${chartWidth - chartPadding},${chartHeight - chartPadding} L ${chartPadding},${chartHeight - chartPadding} Z`
}

function getSuccessRatePoints(): { x: number; y: number; point: MemoryTrendPoint }[] {
  if (!trends.value || trends.value.history.length < 2) return []
  const history = trends.value.history
  const w = chartWidth - chartPadding * 2
  const h = chartHeight - chartPadding * 2

  return history.map((p, i) => ({
    x: chartPadding + (i / (history.length - 1)) * w,
    y: chartPadding + h - (p.success_rate / 100) * h,
    point: p,
  }))
}

// Insight helpers
function getInsightTypeClass(type: string): string {
  switch (type) {
    case 'risk_pattern': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
    case 'recommendation': return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
    case 'anomaly': return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
    default: return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400'
  }
}

function getOutcomeClass(outcome: string): string {
  switch (outcome) {
    case 'success': return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    case 'rollback': return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
    case 'incident': return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
    default: return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400'
  }
}

function getOutcomeIcon(outcome: string): string {
  switch (outcome) {
    case 'success': return 'check'
    case 'rollback': return 'undo'
    case 'incident': return 'alert'
    default: return 'unknown'
  }
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
  })
}

function formatDateTime(dateString: string): string {
  return new Date(dateString).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// Outcome summary
const outcomeSummary = computed(() => {
  const success = recentOutcomes.value.filter(o => o.outcome === 'success').length
  const rollback = recentOutcomes.value.filter(o => o.outcome === 'rollback').length
  const incident = recentOutcomes.value.filter(o => o.outcome === 'incident').length
  return { success, rollback, incident, total: recentOutcomes.value.length }
})
</script>

<template>
  <div class="space-y-6">
    <!-- Summary stats -->
    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      <div class="card">
        <div class="card-content pt-6">
          <div class="flex items-center justify-between">
            <div>
              <div class="text-2xl font-bold">
                {{ trends?.success_rate?.toFixed(1) ?? '--' }}%
              </div>
              <p class="text-sm text-muted-foreground">Success Rate</p>
            </div>
            <div :class="getTrendDirectionClass()">
              <svg v-if="getTrendDirectionIcon() === 'up'" class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 10l7-7m0 0l7 7m-7-7v18" />
              </svg>
              <svg v-else-if="getTrendDirectionIcon() === 'down'" class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3" />
              </svg>
              <svg v-else class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14" />
              </svg>
            </div>
          </div>
          <p class="mt-1 text-xs" :class="getTrendDirectionClass()">
            {{ trends?.trend_direction ?? 'Loading...' }}
          </p>
        </div>
      </div>

      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold">
            {{ trends?.mtbf_hours ? `${trends.mtbf_hours.toFixed(0)}h` : '--' }}
          </div>
          <p class="text-sm text-muted-foreground">Mean Time Between Failures</p>
        </div>
      </div>

      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold">{{ trends?.total_releases ?? '--' }}</div>
          <p class="text-sm text-muted-foreground">Total Releases</p>
        </div>
      </div>

      <div class="card">
        <div class="card-content pt-6">
          <div class="flex gap-4">
            <div>
              <div class="text-2xl font-bold text-yellow-600">{{ trends?.rollbacks ?? 0 }}</div>
              <p class="text-xs text-muted-foreground">Rollbacks</p>
            </div>
            <div>
              <div class="text-2xl font-bold text-red-600">{{ trends?.incidents ?? 0 }}</div>
              <p class="text-xs text-muted-foreground">Incidents</p>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Success rate trend chart -->
    <div class="card">
      <div class="card-header">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="card-title">Success Rate Trend</h2>
            <p class="card-description">Release success rate over time</p>
          </div>
          <div class="flex gap-1">
            <button
              v-for="w in ['7d', '14d', '30d', '90d']"
              :key="w"
              @click="changeWindow(w)"
              :class="['btn-sm', trendWindow === w ? 'btn-primary' : 'btn-ghost']"
            >
              {{ w }}
            </button>
          </div>
        </div>
      </div>
      <div class="card-content">
        <div v-if="loading" class="flex items-center justify-center py-8">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        </div>
        <div v-else-if="!trends || trends.history.length === 0" class="py-8 text-center text-muted-foreground">
          No trend data available
        </div>
        <div v-else>
          <svg class="w-full" :viewBox="`0 0 ${chartWidth} ${chartHeight}`" preserveAspectRatio="xMidYMid meet">
            <defs>
              <linearGradient id="successGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" style="stop-color: #22c55e; stop-opacity: 0.3" />
                <stop offset="100%" style="stop-color: #22c55e; stop-opacity: 0" />
              </linearGradient>
            </defs>
            <!-- 50% threshold line -->
            <line
              :x1="chartPadding"
              :y1="chartPadding + (chartHeight - chartPadding * 2) * 0.5"
              :x2="chartWidth - chartPadding"
              :y2="chartPadding + (chartHeight - chartPadding * 2) * 0.5"
              stroke="currentColor"
              stroke-width="0.5"
              stroke-dasharray="4"
              class="text-muted-foreground/30"
            />
            <path
              v-if="getSuccessRateArea()"
              :d="getSuccessRateArea()"
              fill="url(#successGradient)"
            />
            <path
              v-if="getSuccessRatePath()"
              :d="getSuccessRatePath()"
              fill="none"
              stroke="#22c55e"
              stroke-width="2"
            />
            <circle
              v-for="(dp, i) in getSuccessRatePoints()"
              :key="i"
              :cx="dp.x"
              :cy="dp.y"
              r="3"
              fill="#22c55e"
            >
              <title>{{ formatDate(dp.point.date) }}: {{ dp.point.success_rate.toFixed(1) }}% ({{ dp.point.releases }} releases)</title>
            </circle>
          </svg>
          <div class="mt-2 flex justify-between text-xs text-muted-foreground">
            <span>{{ formatDate(trends.history[0]?.date) }}</span>
            <span>{{ formatDate(trends.history[trends.history.length - 1]?.date) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Error state -->
    <div v-if="error" class="card border-red-200 dark:border-red-800">
      <div class="card-content pt-6">
        <p class="text-red-600 dark:text-red-400">{{ error }}</p>
        <button @click="loadData" class="btn-primary btn-sm mt-2">Retry</button>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-2">
      <!-- Risk patterns / insights -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">Detected Patterns</h2>
          <p class="card-description">Risk patterns identified from release history</p>
        </div>
        <div class="card-content">
          <div v-if="loading" class="flex items-center justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
          </div>
          <div v-else-if="insights.length === 0" class="py-8 text-center text-muted-foreground">
            No patterns detected yet
          </div>
          <div v-else class="space-y-3">
            <div
              v-for="insight in insights"
              :key="insight.id"
              class="rounded-lg border p-3"
            >
              <div class="flex items-start justify-between">
                <div class="flex-1">
                  <div class="flex items-center gap-2">
                    <span :class="['badge text-xs', getInsightTypeClass(insight.type)]">
                      {{ insight.type.replace('_', ' ') }}
                    </span>
                    <span class="font-medium text-sm">{{ insight.title }}</span>
                  </div>
                  <p class="mt-1 text-sm text-muted-foreground">{{ insight.description }}</p>
                </div>
              </div>
              <div class="mt-2 flex items-center gap-3">
                <div class="flex items-center gap-1">
                  <span class="text-xs text-muted-foreground">Confidence:</span>
                  <div class="h-2 w-16 overflow-hidden rounded-full bg-muted">
                    <div
                      class="h-full bg-primary transition-all"
                      :style="{ width: `${insight.confidence * 100}%` }"
                    ></div>
                  </div>
                  <span class="text-xs font-medium">{{ (insight.confidence * 100).toFixed(0) }}%</span>
                </div>
                <span class="text-xs text-muted-foreground">
                  {{ formatDateTime(insight.detected_at) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Recent outcomes -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">Recent Outcomes</h2>
          <p class="card-description">
            Latest release results
            <template v-if="outcomeSummary.total > 0">
              -- {{ outcomeSummary.success }} success, {{ outcomeSummary.rollback }} rollback, {{ outcomeSummary.incident }} incident
            </template>
          </p>
        </div>
        <div class="card-content">
          <div v-if="loading" class="flex items-center justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
          </div>
          <div v-else-if="recentOutcomes.length === 0" class="py-8 text-center text-muted-foreground">
            No outcome data available
          </div>
          <div v-else class="space-y-2">
            <div
              v-for="outcome in recentOutcomes"
              :key="outcome.release_id"
              class="flex items-center justify-between rounded-lg border p-3"
            >
              <div class="flex items-center gap-3">
                <!-- Outcome icon -->
                <svg v-if="getOutcomeIcon(outcome.outcome) === 'check'" class="h-5 w-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <svg v-else-if="getOutcomeIcon(outcome.outcome) === 'undo'" class="h-5 w-5 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6" />
                </svg>
                <svg v-else-if="getOutcomeIcon(outcome.outcome) === 'alert'" class="h-5 w-5 text-red-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.834-1.964-.834-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
                </svg>
                <div>
                  <div class="font-medium font-mono text-sm">{{ outcome.version }}</div>
                  <div v-if="outcome.notes" class="text-xs text-muted-foreground">{{ outcome.notes }}</div>
                </div>
              </div>
              <div class="flex items-center gap-2">
                <span :class="['badge text-xs', getOutcomeClass(outcome.outcome)]">
                  {{ outcome.outcome }}
                </span>
                <span class="text-xs text-muted-foreground">
                  {{ formatDateTime(outcome.completed_at) }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Release-specific insights lookup -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Release Insights Lookup</h2>
        <p class="card-description">Retrieve insights for a specific release by ID</p>
      </div>
      <div class="card-content">
        <div class="flex gap-3">
          <input
            v-model="releaseIdInput"
            type="text"
            placeholder="Enter release ID..."
            class="input flex-1"
            @keyup.enter="lookupReleaseInsights"
          />
          <button
            @click="lookupReleaseInsights"
            :disabled="loadingInsights || !releaseIdInput.trim()"
            class="btn-primary"
          >
            <span v-if="loadingInsights">Loading...</span>
            <span v-else>Lookup</span>
          </button>
        </div>

        <div v-if="loadingInsights" class="mt-4 flex items-center justify-center py-4">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        </div>
        <div v-else-if="releaseInsights.length > 0" class="mt-4 space-y-3">
          <div
            v-for="insight in releaseInsights"
            :key="insight.id"
            class="rounded-lg border p-3"
          >
            <div class="flex items-center gap-2">
              <span :class="['badge text-xs', getInsightTypeClass(insight.type)]">
                {{ insight.type.replace('_', ' ') }}
              </span>
              <span class="font-medium text-sm">{{ insight.title }}</span>
            </div>
            <p class="mt-1 text-sm text-muted-foreground">{{ insight.description }}</p>
            <div class="mt-2 flex items-center gap-1">
              <span class="text-xs text-muted-foreground">Confidence:</span>
              <div class="h-2 w-16 overflow-hidden rounded-full bg-muted">
                <div
                  class="h-full bg-primary transition-all"
                  :style="{ width: `${insight.confidence * 100}%` }"
                ></div>
              </div>
              <span class="text-xs font-medium">{{ (insight.confidence * 100).toFixed(0) }}%</span>
            </div>
          </div>
        </div>
        <div v-else-if="releaseIdInput.trim() && !loadingInsights" class="mt-4 py-4 text-center text-sm text-muted-foreground">
          No insights found for this release. Enter a release ID and click Lookup to search.
        </div>
      </div>
    </div>
  </div>
</template>
