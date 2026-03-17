<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import * as api from '@/api/client'
import type { AuditEvent } from '@/types/api'

const events = ref<AuditEvent[]>([])
const loading = ref(true)
const totalEvents = ref(0)
const currentPage = ref(1)
const pageSize = ref(50)

// Filters
const filterType = ref('')
const filterActor = ref('')
const filterRelease = ref('')
const dateFrom = ref('')
const dateTo = ref('')

onMounted(() => {
  loadEvents()
})

watch([filterType, filterActor, filterRelease, dateFrom, dateTo], () => {
  currentPage.value = 1
  loadEvents()
})

async function loadEvents() {
  loading.value = true
  try {
    const response = await api.listAuditEvents({
      page: currentPage.value,
      limit: pageSize.value,
      event_type: filterType.value || undefined,
      actor: filterActor.value || undefined,
      release_id: filterRelease.value || undefined,
      from: dateFrom.value || undefined,
      to: dateTo.value || undefined,
    })
    events.value = response.data
    totalEvents.value = response.total
  } catch (error) {
    console.error('Failed to load audit events:', error)
  } finally {
    loading.value = false
  }
}

function nextPage() {
  const totalPages = Math.ceil(totalEvents.value / pageSize.value)
  if (currentPage.value < totalPages) {
    currentPage.value++
    loadEvents()
  }
}

function prevPage() {
  if (currentPage.value > 1) {
    currentPage.value--
    loadEvents()
  }
}

function clearFilters() {
  filterType.value = ''
  filterActor.value = ''
  filterRelease.value = ''
  dateFrom.value = ''
  dateTo.value = ''
}

function formatDateTime(dateString: string): string {
  return new Date(dateString).toLocaleString()
}

function getEventTypeColor(type: string): string {
  if (type.includes('created')) return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
  if (type.includes('approved')) return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
  if (type.includes('published')) return 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400'
  if (type.includes('failed') || type.includes('denied')) return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
  if (type.includes('canceled')) return 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-400'
  return 'bg-muted text-muted-foreground'
}

function exportEvents() {
  const data = events.value.map(e => ({
    id: e.id,
    type: e.type,
    release_id: e.release_id,
    actor_id: e.actor_id,
    timestamp: e.timestamp,
    data: JSON.stringify(e.data),
  }))

  const csv = [
    Object.keys(data[0]).join(','),
    ...data.map(row => Object.values(row).map(v => `"${v}"`).join(',')),
  ].join('\n')

  const blob = new Blob([csv], { type: 'text/csv' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `audit-trail-${new Date().toISOString().split('T')[0]}.csv`
  a.click()
  URL.revokeObjectURL(url)
}

const totalPages = () => Math.ceil(totalEvents.value / pageSize.value)
</script>

<template>
  <div class="space-y-6">
    <!-- Filters -->
    <div class="card">
      <div class="card-header">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="card-title">Filters</h2>
            <p class="card-description">Narrow down audit events</p>
          </div>
          <div class="flex gap-2">
            <button @click="clearFilters" class="btn-ghost btn-sm">
              Clear Filters
            </button>
            <button @click="exportEvents" :disabled="events.length === 0" class="btn-outline btn-sm">
              <svg class="mr-2 h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              Export CSV
            </button>
          </div>
        </div>
      </div>
      <div class="card-content">
        <div class="grid gap-4 md:grid-cols-5">
          <div>
            <label class="text-sm font-medium">Event Type</label>
            <input
              v-model="filterType"
              type="text"
              class="input mt-1"
              placeholder="e.g., release.created"
            />
          </div>
          <div>
            <label class="text-sm font-medium">Actor ID</label>
            <input
              v-model="filterActor"
              type="text"
              class="input mt-1"
              placeholder="Actor ID..."
            />
          </div>
          <div>
            <label class="text-sm font-medium">Release ID</label>
            <input
              v-model="filterRelease"
              type="text"
              class="input mt-1"
              placeholder="Release ID..."
            />
          </div>
          <div>
            <label class="text-sm font-medium">From Date</label>
            <input
              v-model="dateFrom"
              type="datetime-local"
              class="input mt-1"
            />
          </div>
          <div>
            <label class="text-sm font-medium">To Date</label>
            <input
              v-model="dateTo"
              type="datetime-local"
              class="input mt-1"
            />
          </div>
        </div>
      </div>
    </div>

    <!-- Events list -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Audit Events</h2>
        <p class="card-description">
          {{ totalEvents }} total event{{ totalEvents !== 1 ? 's' : '' }}
        </p>
      </div>
      <div class="card-content p-0">
        <div v-if="loading" class="flex items-center justify-center py-12">
          <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        </div>
        <div v-else-if="events.length === 0" class="py-12 text-center text-muted-foreground">
          No audit events found
        </div>
        <div v-else class="divide-y">
          <div
            v-for="event in events"
            :key="event.id"
            class="p-4 hover:bg-muted/50"
          >
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="flex-1 space-y-2">
                <div class="flex items-center gap-3">
                  <span :class="['badge', getEventTypeColor(event.type)]">
                    {{ event.type }}
                  </span>
                  <span class="text-sm text-muted-foreground">
                    {{ formatDateTime(event.timestamp) }}
                  </span>
                </div>
                <div class="text-sm">
                  <span class="text-muted-foreground">Release:</span>
                  <RouterLink
                    :to="`/releases/${event.release_id}`"
                    class="ml-1 text-primary hover:underline"
                  >
                    {{ event.release_id.substring(0, 12) }}...
                  </RouterLink>
                </div>
                <div class="text-sm">
                  <span class="text-muted-foreground">Actor:</span>
                  <code class="ml-1 rounded bg-muted px-1.5 py-0.5 text-xs">
                    {{ event.actor_id }}
                  </code>
                </div>
              </div>
              <div v-if="Object.keys(event.data).length > 0" class="w-full md:w-auto md:max-w-md">
                <details class="rounded-md bg-muted">
                  <summary class="cursor-pointer px-3 py-2 text-sm font-medium">
                    Event Data
                  </summary>
                  <pre class="overflow-auto px-3 py-2 text-xs">{{ JSON.stringify(event.data, null, 2) }}</pre>
                </details>
              </div>
            </div>
          </div>
        </div>
      </div>
      <div class="card-footer justify-between border-t pt-4">
        <div class="text-sm text-muted-foreground">
          Page {{ currentPage }} of {{ totalPages() }}
        </div>
        <div class="flex gap-2">
          <button
            @click="prevPage"
            :disabled="currentPage <= 1"
            class="btn-outline btn-sm"
          >
            Previous
          </button>
          <button
            @click="nextPage"
            :disabled="currentPage >= totalPages()"
            class="btn-outline btn-sm"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
