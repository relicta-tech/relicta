<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import * as api from '@/api/client'
import type {
  Granularity,
  AnalyticsRiskTrendPoint,
  AnalyticsDecisionCounts,
  AnalyticsTeamMember,
} from '@/types/api'

const loading = ref(true)
const error = ref<string | null>(null)

// Date range
const dateFrom = ref('')
const dateTo = ref('')
const granularity = ref<Granularity>('day')

// Data
const riskTrends = ref<AnalyticsRiskTrendPoint[]>([])
const decisionCounts = ref<AnalyticsDecisionCounts>({ approve: 0, deny: 0, require_review: 0, total: 0 })
const decisionsByPeriod = ref<{ date: string; approve: number; deny: number; require_review: number }[]>([])
const teamMembers = ref<AnalyticsTeamMember[]>([])
const totalReleases = ref(0)
const avgVelocity = ref(0)

// Initialize date range to last 30 days
function initDateRange() {
  const now = new Date()
  const thirtyDaysAgo = new Date(now)
  thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30)
  dateTo.value = now.toISOString().split('T')[0]
  dateFrom.value = thirtyDaysAgo.toISOString().split('T')[0]
}

onMounted(() => {
  initDateRange()
  loadData()
})

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const params = {
      from: dateFrom.value,
      to: dateTo.value,
      granularity: granularity.value,
    }
    const [trendsRes, decisionsRes, teamRes] = await Promise.all([
      api.getAnalyticsRiskTrends(params),
      api.getAnalyticsDecisions(params),
      api.getAnalyticsTeam({ from: dateFrom.value, to: dateTo.value }),
    ])
    riskTrends.value = trendsRes.trends
    decisionCounts.value = decisionsRes.counts
    decisionsByPeriod.value = decisionsRes.by_period
    teamMembers.value = teamRes.members
    totalReleases.value = teamRes.total_releases
    avgVelocity.value = teamRes.avg_velocity
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load analytics data'
    console.error('Failed to load analytics data:', err)
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  loadData()
}

// Risk trend SVG chart helpers
const trendChartWidth = 600
const trendChartHeight = 120
const trendChartPadding = 8

function getRiskTrendLinePath(): string {
  if (riskTrends.value.length < 2) return ''
  const maxScore = Math.max(...riskTrends.value.map(t => t.avg_score), 100)
  const w = trendChartWidth - trendChartPadding * 2
  const h = trendChartHeight - trendChartPadding * 2

  const points = riskTrends.value.map((t, i) => {
    const x = trendChartPadding + (i / (riskTrends.value.length - 1)) * w
    const y = trendChartPadding + h - (t.avg_score / maxScore) * h
    return `${x},${y}`
  })
  return `M ${points.join(' L ')}`
}

function getRiskTrendAreaPath(): string {
  const line = getRiskTrendLinePath()
  if (!line) return ''
  const bottomRight = `${trendChartWidth - trendChartPadding},${trendChartHeight - trendChartPadding}`
  const bottomLeft = `${trendChartPadding},${trendChartHeight - trendChartPadding}`
  return `${line} L ${bottomRight} L ${bottomLeft} Z`
}

function getRiskTrendDataPoints(): { x: number; y: number; point: AnalyticsRiskTrendPoint }[] {
  if (riskTrends.value.length < 2) return []
  const maxScore = Math.max(...riskTrends.value.map(t => t.avg_score), 100)
  const w = trendChartWidth - trendChartPadding * 2
  const h = trendChartHeight - trendChartPadding * 2

  return riskTrends.value.map((point, i) => ({
    x: trendChartPadding + (i / (riskTrends.value.length - 1)) * w,
    y: trendChartPadding + h - (point.avg_score / maxScore) * h,
    point,
  }))
}

// Decision donut helpers
const decisionSegments = computed(() => {
  const total = decisionCounts.value.total || 1
  return [
    { label: 'Approved', count: decisionCounts.value.approve, pct: (decisionCounts.value.approve / total) * 100, color: '#22c55e' },
    { label: 'Required Review', count: decisionCounts.value.require_review, pct: (decisionCounts.value.require_review / total) * 100, color: '#eab308' },
    { label: 'Denied', count: decisionCounts.value.deny, pct: (decisionCounts.value.deny / total) * 100, color: '#ef4444' },
  ]
})

function getDonutPath(startAngle: number, endAngle: number, radius = 40, cx = 50, cy = 50): string {
  const innerRadius = radius * 0.6
  const startRad = (startAngle - 90) * (Math.PI / 180)
  const endRad = (endAngle - 90) * (Math.PI / 180)

  const x1 = cx + radius * Math.cos(startRad)
  const y1 = cy + radius * Math.sin(startRad)
  const x2 = cx + radius * Math.cos(endRad)
  const y2 = cy + radius * Math.sin(endRad)
  const x3 = cx + innerRadius * Math.cos(endRad)
  const y3 = cy + innerRadius * Math.sin(endRad)
  const x4 = cx + innerRadius * Math.cos(startRad)
  const y4 = cy + innerRadius * Math.sin(startRad)

  const largeArc = endAngle - startAngle > 180 ? 1 : 0
  return `M ${x1} ${y1} A ${radius} ${radius} 0 ${largeArc} 1 ${x2} ${y2} L ${x3} ${y3} A ${innerRadius} ${innerRadius} 0 ${largeArc} 0 ${x4} ${y4} Z`
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
  })
}

// Sort helpers for team table
const sortField = ref<keyof AnalyticsTeamMember>('releases')
const sortDir = ref<'asc' | 'desc'>('desc')

function toggleSort(field: keyof AnalyticsTeamMember) {
  if (sortField.value === field) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortField.value = field
    sortDir.value = 'desc'
  }
}

const sortedTeamMembers = computed(() => {
  const sorted = [...teamMembers.value]
  sorted.sort((a, b) => {
    const aVal = a[sortField.value]
    const bVal = b[sortField.value]
    if (typeof aVal === 'number' && typeof bVal === 'number') {
      return sortDir.value === 'asc' ? aVal - bVal : bVal - aVal
    }
    return sortDir.value === 'asc'
      ? String(aVal).localeCompare(String(bVal))
      : String(bVal).localeCompare(String(aVal))
  })
  return sorted
})
</script>

<template>
  <div class="space-y-6">
    <!-- Filters -->
    <div class="card">
      <div class="card-content pt-6">
        <div class="flex flex-wrap items-end gap-4">
          <div>
            <label class="mb-1 block text-sm font-medium text-muted-foreground">From</label>
            <input
              v-model="dateFrom"
              type="date"
              class="input"
            >
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium text-muted-foreground">To</label>
            <input
              v-model="dateTo"
              type="date"
              class="input"
            >
          </div>
          <div>
            <label class="mb-1 block text-sm font-medium text-muted-foreground">Granularity</label>
            <div class="flex gap-1">
              <button
                v-for="g in (['day', 'week', 'month'] as Granularity[])"
                :key="g"
                :class="['btn-sm', granularity === g ? 'btn-primary' : 'btn-ghost']"
                @click="granularity = g"
              >
                {{ g }}
              </button>
            </div>
          </div>
          <button
            class="btn-primary btn-sm"
            @click="applyFilters"
          >
            Apply
          </button>
        </div>
      </div>
    </div>

    <!-- Error state -->
    <div
      v-if="error"
      class="card border-red-200 dark:border-red-800"
    >
      <div class="card-content pt-6">
        <p class="text-red-600 dark:text-red-400">
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

    <!-- Summary cards -->
    <div class="grid gap-4 md:grid-cols-4">
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold">
            {{ totalReleases }}
          </div>
          <p class="text-sm text-muted-foreground">
            Total Releases
          </p>
        </div>
      </div>
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold text-green-600">
            {{ decisionCounts.approve }}
          </div>
          <p class="text-sm text-muted-foreground">
            Approved
          </p>
        </div>
      </div>
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold text-yellow-600">
            {{ decisionCounts.require_review }}
          </div>
          <p class="text-sm text-muted-foreground">
            Required Review
          </p>
        </div>
      </div>
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold text-red-600">
            {{ decisionCounts.deny }}
          </div>
          <p class="text-sm text-muted-foreground">
            Denied
          </p>
        </div>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-2">
      <!-- Risk trend line chart -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">
            Risk Trends
          </h2>
          <p class="card-description">
            Average risk score over time ({{ granularity }})
          </p>
        </div>
        <div class="card-content">
          <div
            v-if="loading"
            class="flex items-center justify-center py-8"
          >
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          </div>
          <div
            v-else-if="riskTrends.length === 0"
            class="py-8 text-center text-muted-foreground"
          >
            No trend data available for this range
          </div>
          <div v-else>
            <svg
              class="w-full"
              :viewBox="`0 0 ${trendChartWidth} ${trendChartHeight}`"
              preserveAspectRatio="xMidYMid meet"
            >
              <defs>
                <linearGradient
                  id="analyticsRiskGradient"
                  x1="0%"
                  y1="0%"
                  x2="0%"
                  y2="100%"
                >
                  <stop
                    offset="0%"
                    style="stop-color: var(--color-primary); stop-opacity: 0.3"
                  />
                  <stop
                    offset="100%"
                    style="stop-color: var(--color-primary); stop-opacity: 0"
                  />
                </linearGradient>
              </defs>
              <path
                v-if="getRiskTrendAreaPath()"
                :d="getRiskTrendAreaPath()"
                fill="url(#analyticsRiskGradient)"
              />
              <path
                v-if="getRiskTrendLinePath()"
                :d="getRiskTrendLinePath()"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                class="text-primary"
              />
              <circle
                v-for="(dp, i) in getRiskTrendDataPoints()"
                :key="i"
                :cx="dp.x"
                :cy="dp.y"
                r="3"
                class="fill-primary"
              >
                <title>{{ formatDate(dp.point.date) }}: avg {{ dp.point.avg_score.toFixed(1) }}, max {{ dp.point.max_score }}, {{ dp.point.release_count }} releases</title>
              </circle>
            </svg>
            <div class="mt-2 flex justify-between text-xs text-muted-foreground">
              <span>{{ formatDate(riskTrends[0]?.date) }}</span>
              <span>{{ formatDate(riskTrends[riskTrends.length - 1]?.date) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Decision distribution -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">
            Decision Distribution
          </h2>
          <p class="card-description">
            Breakdown of governance outcomes
          </p>
        </div>
        <div class="card-content">
          <div
            v-if="loading"
            class="flex items-center justify-center py-8"
          >
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          </div>
          <div
            v-else-if="decisionCounts.total === 0"
            class="py-8 text-center text-muted-foreground"
          >
            No decisions in this range
          </div>
          <div
            v-else
            class="flex flex-col items-center gap-6 md:flex-row md:justify-around"
          >
            <div class="relative">
              <svg
                viewBox="0 0 100 100"
                class="h-40 w-40"
              >
                <template
                  v-for="(seg, i) in decisionSegments"
                  :key="seg.label"
                >
                  <path
                    v-if="seg.pct > 0"
                    :d="getDonutPath(
                      decisionSegments.slice(0, i).reduce((s, v) => s + v.pct * 3.6, 0),
                      decisionSegments.slice(0, i + 1).reduce((s, v) => s + v.pct * 3.6, 0)
                    )"
                    :fill="seg.color"
                    class="transition-all hover:opacity-80"
                  >
                    <title>{{ seg.label }}: {{ seg.count }} ({{ seg.pct.toFixed(1) }}%)</title>
                  </path>
                </template>
                <text
                  x="50"
                  y="48"
                  text-anchor="middle"
                  class="fill-foreground text-lg font-bold"
                >
                  {{ decisionCounts.total }}
                </text>
                <text
                  x="50"
                  y="58"
                  text-anchor="middle"
                  class="fill-muted-foreground text-[8px]"
                >
                  decisions
                </text>
              </svg>
            </div>
            <div class="space-y-3">
              <div
                v-for="seg in decisionSegments"
                :key="seg.label"
                class="flex items-center gap-3"
              >
                <div
                  class="h-3 w-3 rounded-full"
                  :style="{ backgroundColor: seg.color }"
                />
                <div>
                  <div class="text-sm font-medium">
                    {{ seg.label }}
                  </div>
                  <div class="text-xs text-muted-foreground">
                    {{ seg.count }} ({{ seg.pct.toFixed(1) }}%)
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Team metrics table -->
    <div class="card">
      <div class="card-header">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="card-title">
              Team Metrics
            </h2>
            <p class="card-description">
              Approvals per actor and release velocity
              <span
                v-if="avgVelocity > 0"
                class="ml-2 text-primary"
              >
                (avg {{ avgVelocity.toFixed(1) }} releases/week)
              </span>
            </p>
          </div>
        </div>
      </div>
      <div class="card-content p-0">
        <div
          v-if="loading"
          class="flex items-center justify-center py-8"
        >
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
        </div>
        <div
          v-else-if="teamMembers.length === 0"
          class="py-8 text-center text-muted-foreground"
        >
          No team data available
        </div>
        <table
          v-else
          class="table"
        >
          <thead class="table-header">
            <tr>
              <th
                class="table-head cursor-pointer select-none"
                @click="toggleSort('actor_name')"
              >
                Actor
                <span
                  v-if="sortField === 'actor_name'"
                  class="ml-1"
                >{{ sortDir === 'asc' ? '↑' : '↓' }}</span>
              </th>
              <th
                class="table-head cursor-pointer select-none"
                @click="toggleSort('releases')"
              >
                Releases
                <span
                  v-if="sortField === 'releases'"
                  class="ml-1"
                >{{ sortDir === 'asc' ? '↑' : '↓' }}</span>
              </th>
              <th
                class="table-head cursor-pointer select-none"
                @click="toggleSort('approvals')"
              >
                Approvals
                <span
                  v-if="sortField === 'approvals'"
                  class="ml-1"
                >{{ sortDir === 'asc' ? '↑' : '↓' }}</span>
              </th>
              <th
                class="table-head cursor-pointer select-none"
                @click="toggleSort('denials')"
              >
                Denials
                <span
                  v-if="sortField === 'denials'"
                  class="ml-1"
                >{{ sortDir === 'asc' ? '↑' : '↓' }}</span>
              </th>
              <th
                class="table-head cursor-pointer select-none"
                @click="toggleSort('avg_cycle_time_hours')"
              >
                Avg Cycle Time
                <span
                  v-if="sortField === 'avg_cycle_time_hours'"
                  class="ml-1"
                >{{ sortDir === 'asc' ? '↑' : '↓' }}</span>
              </th>
              <th
                class="table-head cursor-pointer select-none"
                @click="toggleSort('success_rate')"
              >
                Success Rate
                <span
                  v-if="sortField === 'success_rate'"
                  class="ml-1"
                >{{ sortDir === 'asc' ? '↑' : '↓' }}</span>
              </th>
            </tr>
          </thead>
          <tbody class="table-body">
            <tr
              v-for="member in sortedTeamMembers"
              :key="member.actor_id"
              class="table-row"
            >
              <td class="table-cell font-medium">
                {{ member.actor_name }}
                <div class="text-xs text-muted-foreground">
                  {{ member.actor_id.substring(0, 12) }}
                </div>
              </td>
              <td class="table-cell">
                {{ member.releases }}
              </td>
              <td class="table-cell text-green-600">
                {{ member.approvals }}
              </td>
              <td class="table-cell text-red-600">
                {{ member.denials }}
              </td>
              <td class="table-cell">
                {{ member.avg_cycle_time_hours.toFixed(1) }}h
              </td>
              <td class="table-cell">
                <div class="flex items-center gap-2">
                  <div class="h-2 w-16 overflow-hidden rounded-full bg-muted">
                    <div
                      class="h-full transition-all"
                      :class="member.success_rate >= 90 ? 'bg-green-500' : member.success_rate >= 70 ? 'bg-yellow-500' : 'bg-red-500'"
                      :style="{ width: `${member.success_rate}%` }"
                    />
                  </div>
                  <span class="text-sm">{{ member.success_rate.toFixed(0) }}%</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
