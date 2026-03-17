<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useReleasesStore } from '@/stores/releases'
import * as api from '@/api/client'
import type { RiskTrend, FactorDistribution, ApprovalRequest } from '@/types/api'

const releasesStore = useReleasesStore()

const riskTrends = ref<RiskTrend[]>([])
const factors = ref<FactorDistribution[]>([])
const pendingApprovals = ref<ApprovalRequest[]>([])
const loading = ref(true)

onMounted(async () => {
  try {
    const [trendsRes, factorsRes, approvalsRes] = await Promise.all([
      api.getRiskTrends(14),
      api.getFactorDistribution(),
      api.listPendingApprovals(),
    ])
    riskTrends.value = trendsRes.trends
    factors.value = factorsRes.factors
    pendingApprovals.value = approvalsRes.data
    await releasesStore.fetchReleases()
  } catch (error) {
    console.error('Failed to load dashboard data:', error)
  } finally {
    loading.value = false
  }
})

const recentReleases = computed(() => releasesStore.releases.slice(0, 5))

function getRiskLevelClass(level: string): string {
  const classes: Record<string, string> = {
    low: 'badge-risk-low',
    medium: 'badge-risk-medium',
    high: 'badge-risk-high',
    critical: 'badge-risk-critical',
  }
  return classes[level] || 'badge-risk-low'
}

function getStateClass(state: string): string {
  return `badge-state-${state}`
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <div class="space-y-6">
    <!-- Stats cards -->
    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      <div class="card">
        <div class="card-header pb-2">
          <h3 class="text-sm font-medium text-muted-foreground">Total Releases</h3>
        </div>
        <div class="card-content">
          <div class="text-2xl font-bold">{{ releasesStore.totalReleases }}</div>
          <p class="text-xs text-muted-foreground">All time</p>
        </div>
      </div>

      <div class="card">
        <div class="card-header pb-2">
          <h3 class="text-sm font-medium text-muted-foreground">Pending Approvals</h3>
        </div>
        <div class="card-content">
          <div class="text-2xl font-bold">{{ pendingApprovals.length }}</div>
          <p class="text-xs text-muted-foreground">Awaiting review</p>
        </div>
      </div>

      <div class="card">
        <div class="card-header pb-2">
          <h3 class="text-sm font-medium text-muted-foreground">Avg Risk Score</h3>
        </div>
        <div class="card-content">
          <div class="text-2xl font-bold">
            {{ riskTrends.length > 0 ? (riskTrends.reduce((sum, t) => sum + t.risk_score, 0) / riskTrends.length).toFixed(1) : '—' }}
          </div>
          <p class="text-xs text-muted-foreground">Last 14 days</p>
        </div>
      </div>

      <div class="card">
        <div class="card-header pb-2">
          <h3 class="text-sm font-medium text-muted-foreground">Active Release</h3>
        </div>
        <div class="card-content">
          <div class="text-2xl font-bold">
            {{ releasesStore.activeRelease?.version_next || '—' }}
          </div>
          <p class="text-xs text-muted-foreground">
            {{ releasesStore.activeRelease?.state || 'None in progress' }}
          </p>
        </div>
      </div>
    </div>

    <div class="grid gap-6 lg:grid-cols-2">
      <!-- Recent releases -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">Recent Releases</h2>
          <p class="card-description">Latest release activity</p>
        </div>
        <div class="card-content">
          <div v-if="loading" class="flex items-center justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
          </div>
          <div v-else-if="recentReleases.length === 0" class="py-8 text-center text-muted-foreground">
            No releases yet
          </div>
          <div v-else class="space-y-4">
            <RouterLink
              v-for="release in recentReleases"
              :key="release.id"
              :to="`/releases/${release.id}`"
              class="flex items-center justify-between rounded-lg border p-4 transition-colors hover:bg-muted/50"
            >
              <div class="flex items-center gap-4">
                <div>
                  <div class="font-medium">{{ release.version_next }}</div>
                  <div class="text-sm text-muted-foreground">
                    {{ release.commit_count }} commits
                  </div>
                </div>
              </div>
              <div class="flex items-center gap-3">
                <span :class="['badge', getRiskLevelClass(release.risk_level)]">
                  {{ release.risk_level }}
                </span>
                <span :class="['badge', getStateClass(release.state)]">
                  {{ release.state }}
                </span>
              </div>
            </RouterLink>
          </div>
        </div>
      </div>

      <!-- Pending approvals -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">Pending Approvals</h2>
          <p class="card-description">Releases awaiting review</p>
        </div>
        <div class="card-content">
          <div v-if="loading" class="flex items-center justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
          </div>
          <div v-else-if="pendingApprovals.length === 0" class="py-8 text-center text-muted-foreground">
            No pending approvals
          </div>
          <div v-else class="space-y-4">
            <div
              v-for="approval in pendingApprovals"
              :key="approval.release_id"
              class="flex items-center justify-between rounded-lg border p-4"
            >
              <div>
                <div class="font-medium">{{ approval.version }}</div>
                <div class="text-sm text-muted-foreground">
                  {{ approval.commit_count }} commits · {{ formatDate(approval.submitted_at) }}
                </div>
                <div v-if="approval.review_reason" class="mt-1 text-sm text-yellow-600 dark:text-yellow-400">
                  {{ approval.review_reason }}
                </div>
              </div>
              <div class="flex items-center gap-2">
                <span :class="['badge', getRiskLevelClass(approval.risk_level)]">
                  {{ approval.risk_level }}
                </span>
                <RouterLink :to="`/approvals`" class="btn-primary btn-sm">
                  Review
                </RouterLink>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Risk factors distribution -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Risk Factors</h2>
        <p class="card-description">Distribution of risk factors across releases</p>
      </div>
      <div class="card-content">
        <div v-if="loading" class="flex items-center justify-center py-8">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        </div>
        <div v-else-if="factors.length === 0" class="py-8 text-center text-muted-foreground">
          No factor data available
        </div>
        <div v-else class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          <div
            v-for="factor in factors"
            :key="factor.factor"
            class="rounded-lg border p-4"
          >
            <div class="flex items-center justify-between">
              <span class="font-medium">{{ factor.factor }}</span>
              <span class="text-sm text-muted-foreground">{{ factor.count }}</span>
            </div>
            <div class="mt-2">
              <div class="h-2 overflow-hidden rounded-full bg-muted">
                <div
                  class="h-full bg-primary transition-all"
                  :style="{ width: `${factor.percentage}%` }"
                ></div>
              </div>
            </div>
            <div class="mt-1 text-right text-xs text-muted-foreground">
              {{ factor.percentage.toFixed(1) }}%
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
