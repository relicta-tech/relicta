<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useReleasesStore } from '@/stores/releases'

const releasesStore = useReleasesStore()
const filterState = ref<string>('all')
const filterRisk = ref<string>('all')

onMounted(() => {
  releasesStore.fetchReleases()
})

const filteredReleases = computed(() => {
  let releases = releasesStore.releases

  if (filterState.value !== 'all') {
    releases = releases.filter((r) => r.state === filterState.value)
  }

  if (filterRisk.value !== 'all') {
    releases = releases.filter((r) => r.risk_level === filterRisk.value)
  }

  return releases
})

const states = ['all', 'draft', 'planned', 'versioned', 'notes_ready', 'approved', 'publishing', 'published', 'failed', 'canceled']
const riskLevels = ['all', 'low', 'medium', 'high', 'critical']

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
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function nextPage() {
  if (releasesStore.currentPage < releasesStore.totalPages) {
    releasesStore.fetchReleases(releasesStore.currentPage + 1)
  }
}

function prevPage() {
  if (releasesStore.currentPage > 1) {
    releasesStore.fetchReleases(releasesStore.currentPage - 1)
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Filters -->
    <div class="card">
      <div class="card-content pt-6">
        <div class="flex flex-wrap gap-4">
          <div class="flex items-center gap-2">
            <label class="text-sm font-medium">State:</label>
            <select v-model="filterState" class="input w-40">
              <option v-for="state in states" :key="state" :value="state">
                {{ state === 'all' ? 'All States' : state }}
              </option>
            </select>
          </div>
          <div class="flex items-center gap-2">
            <label class="text-sm font-medium">Risk:</label>
            <select v-model="filterRisk" class="input w-40">
              <option v-for="level in riskLevels" :key="level" :value="level">
                {{ level === 'all' ? 'All Levels' : level }}
              </option>
            </select>
          </div>
        </div>
      </div>
    </div>

    <!-- Active release highlight -->
    <div v-if="releasesStore.activeRelease" class="card border-blue-200 bg-blue-50 dark:border-blue-800 dark:bg-blue-950">
      <div class="card-header">
        <div class="flex items-center gap-3">
          <span class="h-3 w-3 animate-pulse rounded-full bg-blue-500"></span>
          <h2 class="card-title">Active Release</h2>
        </div>
      </div>
      <div class="card-content">
        <RouterLink
          :to="`/releases/${releasesStore.activeRelease.id}`"
          class="flex items-center justify-between"
        >
          <div>
            <div class="text-2xl font-bold">{{ releasesStore.activeRelease.version_next }}</div>
            <div class="text-sm text-muted-foreground">
              {{ releasesStore.activeRelease.commit_count }} commits ·
              Started {{ formatDate(releasesStore.activeRelease.created_at) }}
            </div>
          </div>
          <div class="flex items-center gap-3">
            <span :class="['badge', getRiskLevelClass(releasesStore.activeRelease.risk_level)]">
              Risk: {{ releasesStore.activeRelease.risk_score }}
            </span>
            <span :class="['badge', getStateClass(releasesStore.activeRelease.state)]">
              {{ releasesStore.activeRelease.state }}
            </span>
          </div>
        </RouterLink>
      </div>
    </div>

    <!-- Releases table -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">All Releases</h2>
        <p class="card-description">
          {{ releasesStore.totalReleases }} total releases
        </p>
      </div>
      <div class="card-content p-0">
        <div v-if="releasesStore.loading" class="flex items-center justify-center py-12">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        </div>
        <div v-else-if="filteredReleases.length === 0" class="py-12 text-center text-muted-foreground">
          No releases found
        </div>
        <table v-else class="table">
          <thead class="table-header">
            <tr>
              <th class="table-head">Version</th>
              <th class="table-head">Commits</th>
              <th class="table-head">Risk</th>
              <th class="table-head">State</th>
              <th class="table-head">Actor</th>
              <th class="table-head">Updated</th>
              <th class="table-head"></th>
            </tr>
          </thead>
          <tbody class="table-body">
            <tr
              v-for="release in filteredReleases"
              :key="release.id"
              class="table-row"
            >
              <td class="table-cell font-medium">
                <div>{{ release.version_next }}</div>
                <div v-if="release.version_current" class="text-xs text-muted-foreground">
                  from {{ release.version_current }}
                </div>
              </td>
              <td class="table-cell">{{ release.commit_count }}</td>
              <td class="table-cell">
                <span :class="['badge', getRiskLevelClass(release.risk_level)]">
                  {{ release.risk_score }} ({{ release.risk_level }})
                </span>
              </td>
              <td class="table-cell">
                <span :class="['badge', getStateClass(release.state)]">
                  {{ release.state }}
                </span>
              </td>
              <td class="table-cell">
                <div class="text-sm">
                  <div>{{ release.actor_id.substring(0, 8) }}...</div>
                  <div class="text-xs text-muted-foreground">{{ release.actor_kind }}</div>
                </div>
              </td>
              <td class="table-cell text-sm text-muted-foreground">
                {{ formatDate(release.updated_at) }}
              </td>
              <td class="table-cell">
                <RouterLink :to="`/releases/${release.id}`" class="btn-ghost btn-sm">
                  View
                </RouterLink>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="card-footer justify-between border-t pt-4">
        <div class="text-sm text-muted-foreground">
          Page {{ releasesStore.currentPage }} of {{ releasesStore.totalPages }}
        </div>
        <div class="flex gap-2">
          <button
            @click="prevPage"
            :disabled="releasesStore.currentPage <= 1"
            class="btn-outline btn-sm"
          >
            Previous
          </button>
          <button
            @click="nextPage"
            :disabled="releasesStore.currentPage >= releasesStore.totalPages"
            class="btn-outline btn-sm"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
