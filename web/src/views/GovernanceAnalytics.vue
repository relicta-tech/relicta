<script setup lang="ts">
import { ref, onMounted } from 'vue'
import * as api from '@/api/client'
import type { GovernanceDecision, RiskTrend, FactorDistribution } from '@/types/api'

const decisions = ref<GovernanceDecision[]>([])
const riskTrends = ref<RiskTrend[]>([])
const factors = ref<FactorDistribution[]>([])
const loading = ref(true)
const daysRange = ref(30)

onMounted(() => {
  loadData()
})

async function loadData() {
  loading.value = true
  try {
    const [decisionsRes, trendsRes, factorsRes] = await Promise.all([
      api.listGovernanceDecisions(),
      api.getRiskTrends(daysRange.value),
      api.getFactorDistribution(),
    ])
    decisions.value = decisionsRes.data
    riskTrends.value = trendsRes.trends
    factors.value = factorsRes.factors
  } catch (error) {
    console.error('Failed to load governance data:', error)
  } finally {
    loading.value = false
  }
}

function changeDaysRange(days: number) {
  daysRange.value = days
  loadData()
}

function getDecisionClass(decision: string): string {
  const classes: Record<string, string> = {
    approve: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
    deny: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
    require_review: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
    pending: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400',
  }
  return classes[decision] || classes.pending
}

function getRiskLevelClass(level: string): string {
  const classes: Record<string, string> = {
    low: 'badge-risk-low',
    medium: 'badge-risk-medium',
    high: 'badge-risk-high',
    critical: 'badge-risk-critical',
  }
  return classes[level] || 'badge-risk-low'
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

function formatDateTime(dateString: string): string {
  return new Date(dateString).toLocaleString()
}

// Simple sparkline calculation
function getRiskTrendPath(): string {
  if (riskTrends.value.length < 2) return ''

  const maxRisk = Math.max(...riskTrends.value.map(t => t.risk_score), 100)
  const width = 300
  const height = 60
  const padding = 4

  const points = riskTrends.value.map((trend, i) => {
    const x = padding + (i / (riskTrends.value.length - 1)) * (width - padding * 2)
    const y = height - padding - (trend.risk_score / maxRisk) * (height - padding * 2)
    return `${x},${y}`
  })

  return `M ${points.join(' L ')}`
}

// Get trend data points for dots
function getRiskTrendPoints(): { x: number; y: number; score: number; date: string }[] {
  if (riskTrends.value.length < 2) return []

  const maxRisk = Math.max(...riskTrends.value.map(t => t.risk_score), 100)
  const width = 300
  const height = 60
  const padding = 4

  return riskTrends.value.map((trend, i) => ({
    x: padding + (i / (riskTrends.value.length - 1)) * (width - padding * 2),
    y: height - padding - (trend.risk_score / maxRisk) * (height - padding * 2),
    score: trend.risk_score,
    date: trend.date,
  }))
}

// Decision distribution for donut chart
function getDecisionDistribution() {
  const approved = decisions.value.filter(d => d.decision === 'approve').length
  const denied = decisions.value.filter(d => d.decision === 'deny').length
  const review = decisions.value.filter(d => d.decision === 'require_review').length
  const total = decisions.value.length || 1

  return [
    { label: 'Approved', count: approved, percentage: (approved / total) * 100, color: '#22c55e' },
    { label: 'Required Review', count: review, percentage: (review / total) * 100, color: '#eab308' },
    { label: 'Denied', count: denied, percentage: (denied / total) * 100, color: '#ef4444' },
  ]
}

// Calculate donut chart segments
function getDonutPath(startAngle: number, endAngle: number, radius: number = 40, cx: number = 50, cy: number = 50): string {
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
</script>

<template>
  <div class="space-y-6">
    <!-- Summary cards -->
    <div class="grid gap-4 md:grid-cols-4">
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold">{{ decisions.length }}</div>
          <p class="text-sm text-muted-foreground">Total Decisions</p>
        </div>
      </div>
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold text-green-600">
            {{ decisions.filter(d => d.decision === 'approve').length }}
          </div>
          <p class="text-sm text-muted-foreground">Approved</p>
        </div>
      </div>
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold text-yellow-600">
            {{ decisions.filter(d => d.decision === 'require_review').length }}
          </div>
          <p class="text-sm text-muted-foreground">Required Review</p>
        </div>
      </div>
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold text-red-600">
            {{ decisions.filter(d => d.decision === 'deny').length }}
          </div>
          <p class="text-sm text-muted-foreground">Denied</p>
        </div>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-2">
      <!-- Risk trends -->
      <div class="card">
        <div class="card-header">
          <div class="flex items-center justify-between">
            <div>
              <h2 class="card-title">Risk Trends</h2>
              <p class="card-description">Average risk score over time</p>
            </div>
            <div class="flex gap-2">
              <button
                v-for="days in [7, 14, 30, 90]"
                :key="days"
                @click="changeDaysRange(days)"
                :class="[
                  'btn-sm',
                  daysRange === days ? 'btn-primary' : 'btn-ghost',
                ]"
              >
                {{ days }}d
              </button>
            </div>
          </div>
        </div>
        <div class="card-content">
          <div v-if="loading" class="flex items-center justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
          </div>
          <div v-else-if="riskTrends.length === 0" class="py-8 text-center text-muted-foreground">
            No trend data available
          </div>
          <div v-else>
            <!-- Enhanced SVG sparkline with gradient and data points -->
            <svg class="w-full" viewBox="0 0 300 80" preserveAspectRatio="xMidYMid meet">
              <defs>
                <linearGradient id="riskGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                  <stop offset="0%" style="stop-color: var(--color-primary); stop-opacity: 0.3" />
                  <stop offset="100%" style="stop-color: var(--color-primary); stop-opacity: 0" />
                </linearGradient>
              </defs>
              <!-- Area fill under line -->
              <path
                v-if="getRiskTrendPath()"
                :d="getRiskTrendPath() + ` L ${300 - 4},${80 - 4} L 4,${80 - 4} Z`"
                fill="url(#riskGradient)"
              />
              <!-- Main line -->
              <path
                :d="getRiskTrendPath()"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                class="text-primary"
              />
              <!-- Data points -->
              <g>
                <circle
                  v-for="(point, i) in getRiskTrendPoints()"
                  :key="i"
                  :cx="point.x"
                  :cy="point.y"
                  r="3"
                  class="fill-primary"
                >
                  <title>{{ formatDate(point.date) }}: {{ point.score }}</title>
                </circle>
              </g>
            </svg>
            <div class="mt-4 flex justify-between text-xs text-muted-foreground">
              <span>{{ formatDate(riskTrends[0]?.date) }}</span>
              <span>{{ formatDate(riskTrends[riskTrends.length - 1]?.date) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Risk factors -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">Risk Factors</h2>
          <p class="card-description">Distribution of contributing factors</p>
        </div>
        <div class="card-content">
          <div v-if="loading" class="flex items-center justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
          </div>
          <div v-else-if="factors.length === 0" class="py-8 text-center text-muted-foreground">
            No factor data available
          </div>
          <div v-else class="space-y-4">
            <div
              v-for="factor in factors"
              :key="factor.factor"
              class="space-y-2"
            >
              <div class="flex justify-between text-sm">
                <span>{{ factor.factor }}</span>
                <span class="text-muted-foreground">{{ factor.count }} ({{ factor.percentage.toFixed(1) }}%)</span>
              </div>
              <div class="h-2 overflow-hidden rounded-full bg-muted">
                <div
                  class="h-full bg-primary transition-all"
                  :style="{ width: `${factor.percentage}%` }"
                ></div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Decision distribution donut chart -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Decision Distribution</h2>
        <p class="card-description">Breakdown of governance decisions by outcome</p>
      </div>
      <div class="card-content">
        <div v-if="loading" class="flex items-center justify-center py-8">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        </div>
        <div v-else-if="decisions.length === 0" class="py-8 text-center text-muted-foreground">
          No decision data available
        </div>
        <div v-else class="flex flex-col items-center gap-6 md:flex-row md:justify-around">
          <!-- Donut chart -->
          <div class="relative">
            <svg viewBox="0 0 100 100" class="h-40 w-40">
              <template v-for="(segment, i) in getDecisionDistribution()" :key="segment.label">
                <path
                  v-if="segment.percentage > 0"
                  :d="getDonutPath(
                    getDecisionDistribution().slice(0, i).reduce((sum, s) => sum + s.percentage * 3.6, 0),
                    getDecisionDistribution().slice(0, i + 1).reduce((sum, s) => sum + s.percentage * 3.6, 0)
                  )"
                  :fill="segment.color"
                  class="transition-all hover:opacity-80"
                >
                  <title>{{ segment.label }}: {{ segment.count }} ({{ segment.percentage.toFixed(1) }}%)</title>
                </path>
              </template>
              <!-- Center text -->
              <text x="50" y="48" text-anchor="middle" class="fill-foreground text-lg font-bold">
                {{ decisions.length }}
              </text>
              <text x="50" y="58" text-anchor="middle" class="fill-muted-foreground text-[8px]">
                decisions
              </text>
            </svg>
          </div>
          <!-- Legend -->
          <div class="space-y-3">
            <div
              v-for="segment in getDecisionDistribution()"
              :key="segment.label"
              class="flex items-center gap-3"
            >
              <div
                class="h-3 w-3 rounded-full"
                :style="{ backgroundColor: segment.color }"
              ></div>
              <div class="flex-1">
                <div class="text-sm font-medium">{{ segment.label }}</div>
                <div class="text-xs text-muted-foreground">
                  {{ segment.count }} ({{ segment.percentage.toFixed(1) }}%)
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Decision history -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Decision History</h2>
        <p class="card-description">Recent governance decisions</p>
      </div>
      <div class="card-content p-0">
        <div v-if="loading" class="flex items-center justify-center py-8">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        </div>
        <div v-else-if="decisions.length === 0" class="py-8 text-center text-muted-foreground">
          No decisions recorded
        </div>
        <table v-else class="table">
          <thead class="table-header">
            <tr>
              <th class="table-head">Release</th>
              <th class="table-head">Decision</th>
              <th class="table-head">Risk</th>
              <th class="table-head">Factors</th>
              <th class="table-head">Actor</th>
              <th class="table-head">Time</th>
            </tr>
          </thead>
          <tbody class="table-body">
            <tr
              v-for="decision in decisions"
              :key="decision.id"
              class="table-row"
            >
              <td class="table-cell">
                <RouterLink
                  :to="`/releases/${decision.release_id}`"
                  class="text-primary hover:underline"
                >
                  {{ decision.release_id.substring(0, 8) }}...
                </RouterLink>
              </td>
              <td class="table-cell">
                <span :class="['badge', getDecisionClass(decision.decision)]">
                  {{ decision.decision }}
                </span>
              </td>
              <td class="table-cell">
                <span :class="['badge', getRiskLevelClass(decision.risk_level)]">
                  {{ decision.risk_score }} ({{ decision.risk_level }})
                </span>
              </td>
              <td class="table-cell">
                <div class="flex flex-wrap gap-1">
                  <span
                    v-for="factor in decision.factors.slice(0, 3)"
                    :key="factor"
                    class="badge bg-muted text-xs"
                  >
                    {{ factor }}
                  </span>
                  <span v-if="decision.factors.length > 3" class="text-xs text-muted-foreground">
                    +{{ decision.factors.length - 3 }}
                  </span>
                </div>
              </td>
              <td class="table-cell text-sm">
                {{ decision.actor_id.substring(0, 8) }}...
                <div class="text-xs text-muted-foreground">{{ decision.actor_kind }}</div>
              </td>
              <td class="table-cell text-sm text-muted-foreground">
                {{ formatDateTime(decision.timestamp) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
