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
            <!-- Simple SVG chart -->
            <svg class="w-full" viewBox="0 0 300 80" preserveAspectRatio="xMidYMid meet">
              <path
                :d="getRiskTrendPath()"
                fill="none"
                stroke="currentColor"
                stroke-width="2"
                class="text-primary"
              />
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
