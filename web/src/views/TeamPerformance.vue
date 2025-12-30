<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import * as api from '@/api/client'
import type { Actor } from '@/types/api'

const actors = ref<Actor[]>([])
const loading = ref(true)
const sortField = ref<'release_count' | 'success_rate' | 'reliability_score'>('reliability_score')
const sortOrder = ref<'asc' | 'desc'>('desc')

onMounted(async () => {
  try {
    const response = await api.listActors()
    actors.value = response.data
  } catch (error) {
    console.error('Failed to load actors:', error)
  } finally {
    loading.value = false
  }
})

const sortedActors = computed(() => {
  return [...actors.value].sort((a, b) => {
    const aVal = a[sortField.value]
    const bVal = b[sortField.value]
    const multiplier = sortOrder.value === 'asc' ? 1 : -1
    return (aVal - bVal) * multiplier
  })
})

function toggleSort(field: 'release_count' | 'success_rate' | 'reliability_score') {
  if (sortField.value === field) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortField.value = field
    sortOrder.value = 'desc'
  }
}

function getTrustLevelClass(level: string): string {
  const classes: Record<string, string> = {
    trusted: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
    standard: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
    probation: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
  }
  return classes[level] || classes.standard
}

function getSuccessRateColor(rate: number): string {
  if (rate >= 90) return 'text-green-600'
  if (rate >= 70) return 'text-yellow-600'
  return 'text-red-600'
}

function getReliabilityColor(score: number): string {
  if (score >= 0.8) return 'bg-green-500'
  if (score >= 0.6) return 'bg-yellow-500'
  return 'bg-red-500'
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

// Stats calculations
const totalReleases = computed(() => actors.value.reduce((sum, a) => sum + a.release_count, 0))
const avgSuccessRate = computed(() => {
  if (actors.value.length === 0) return 0
  return actors.value.reduce((sum, a) => sum + a.success_rate, 0) / actors.value.length
})
const avgReliability = computed(() => {
  if (actors.value.length === 0) return 0
  return actors.value.reduce((sum, a) => sum + a.reliability_score, 0) / actors.value.length
})
</script>

<template>
  <div class="space-y-6">
    <!-- Summary stats -->
    <div class="grid gap-4 md:grid-cols-4">
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold">{{ actors.length }}</div>
          <p class="text-sm text-muted-foreground">Total Actors</p>
        </div>
      </div>
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold">{{ totalReleases }}</div>
          <p class="text-sm text-muted-foreground">Total Releases</p>
        </div>
      </div>
      <div class="card">
        <div class="card-content pt-6">
          <div :class="['text-2xl font-bold', getSuccessRateColor(avgSuccessRate)]">
            {{ avgSuccessRate.toFixed(1) }}%
          </div>
          <p class="text-sm text-muted-foreground">Avg Success Rate</p>
        </div>
      </div>
      <div class="card">
        <div class="card-content pt-6">
          <div class="text-2xl font-bold">{{ (avgReliability * 100).toFixed(0) }}%</div>
          <p class="text-sm text-muted-foreground">Avg Reliability</p>
        </div>
      </div>
    </div>

    <!-- Trust level breakdown -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Trust Level Distribution</h2>
      </div>
      <div class="card-content">
        <div class="flex gap-8">
          <div class="flex items-center gap-3">
            <div class="h-4 w-4 rounded-full bg-green-500"></div>
            <span>Trusted: {{ actors.filter(a => a.trust_level === 'trusted').length }}</span>
          </div>
          <div class="flex items-center gap-3">
            <div class="h-4 w-4 rounded-full bg-blue-500"></div>
            <span>Standard: {{ actors.filter(a => a.trust_level === 'standard').length }}</span>
          </div>
          <div class="flex items-center gap-3">
            <div class="h-4 w-4 rounded-full bg-yellow-500"></div>
            <span>Probation: {{ actors.filter(a => a.trust_level === 'probation').length }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Actors table -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Actor Performance</h2>
        <p class="card-description">Individual actor metrics and trust levels</p>
      </div>
      <div class="card-content p-0">
        <div v-if="loading" class="flex items-center justify-center py-12">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        </div>
        <div v-else-if="actors.length === 0" class="py-12 text-center text-muted-foreground">
          No actors found
        </div>
        <table v-else class="table">
          <thead class="table-header">
            <tr>
              <th class="table-head">Actor</th>
              <th class="table-head">Kind</th>
              <th class="table-head">Trust Level</th>
              <th
                class="table-head cursor-pointer hover:text-foreground"
                @click="toggleSort('release_count')"
              >
                Releases
                <span v-if="sortField === 'release_count'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
              </th>
              <th
                class="table-head cursor-pointer hover:text-foreground"
                @click="toggleSort('success_rate')"
              >
                Success Rate
                <span v-if="sortField === 'success_rate'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
              </th>
              <th class="table-head">Avg Risk</th>
              <th
                class="table-head cursor-pointer hover:text-foreground"
                @click="toggleSort('reliability_score')"
              >
                Reliability
                <span v-if="sortField === 'reliability_score'">{{ sortOrder === 'asc' ? '↑' : '↓' }}</span>
              </th>
              <th class="table-head">Last Seen</th>
            </tr>
          </thead>
          <tbody class="table-body">
            <tr
              v-for="actor in sortedActors"
              :key="actor.id"
              class="table-row"
            >
              <td class="table-cell">
                <div class="font-medium">{{ actor.name }}</div>
                <code class="text-xs text-muted-foreground">{{ actor.id.substring(0, 12) }}...</code>
              </td>
              <td class="table-cell">
                <span class="badge bg-muted">{{ actor.kind }}</span>
              </td>
              <td class="table-cell">
                <span :class="['badge', getTrustLevelClass(actor.trust_level)]">
                  {{ actor.trust_level }}
                </span>
              </td>
              <td class="table-cell font-medium">{{ actor.release_count }}</td>
              <td class="table-cell">
                <span :class="getSuccessRateColor(actor.success_rate)">
                  {{ actor.success_rate.toFixed(1) }}%
                </span>
              </td>
              <td class="table-cell">{{ actor.average_risk_score.toFixed(1) }}</td>
              <td class="table-cell">
                <div class="flex items-center gap-2">
                  <div class="h-2 w-20 overflow-hidden rounded-full bg-muted">
                    <div
                      :class="['h-full', getReliabilityColor(actor.reliability_score)]"
                      :style="{ width: `${actor.reliability_score * 100}%` }"
                    ></div>
                  </div>
                  <span class="text-sm">{{ (actor.reliability_score * 100).toFixed(0) }}%</span>
                </div>
              </td>
              <td class="table-cell text-sm text-muted-foreground">
                {{ formatDate(actor.last_seen) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
