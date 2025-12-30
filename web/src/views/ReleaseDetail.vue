<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useReleasesStore } from '@/stores/releases'
import * as api from '@/api/client'
import type { ReleaseEvent } from '@/types/api'

const route = useRoute()
const router = useRouter()
const releasesStore = useReleasesStore()

const events = ref<ReleaseEvent[]>([])
const eventsLoading = ref(false)
const activeTab = ref<'overview' | 'commits' | 'events'>('overview')

const releaseId = computed(() => route.params.id as string)
const release = computed(() => releasesStore.selectedRelease)

onMounted(() => {
  loadRelease()
})

watch(releaseId, () => {
  loadRelease()
})

async function loadRelease() {
  await releasesStore.fetchRelease(releaseId.value)
  loadEvents()
}

async function loadEvents() {
  eventsLoading.value = true
  try {
    const response = await api.getReleaseEvents(releaseId.value)
    events.value = response.data
  } catch (error) {
    console.error('Failed to load events:', error)
  } finally {
    eventsLoading.value = false
  }
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

function getStateClass(state: string): string {
  return `badge-state-${state}`
}

function getStepStateClass(state: string): string {
  const classes: Record<string, string> = {
    pending: 'text-gray-500',
    running: 'text-blue-500',
    done: 'text-green-500',
    failed: 'text-red-500',
    skipped: 'text-gray-400',
  }
  return classes[state] || 'text-gray-500'
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString()
}

function goBack() {
  router.back()
}
</script>

<template>
  <div class="space-y-6">
    <!-- Back button -->
    <button @click="goBack" class="btn-ghost btn-sm">
      <svg class="mr-2 h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
      </svg>
      Back
    </button>

    <div v-if="releasesStore.loading" class="flex items-center justify-center py-12">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
    </div>

    <div v-else-if="releasesStore.error" class="card border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-950">
      <div class="card-content pt-6">
        <p class="text-red-600 dark:text-red-400">{{ releasesStore.error }}</p>
      </div>
    </div>

    <template v-else-if="release">
      <!-- Header -->
      <div class="card">
        <div class="card-content pt-6">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div>
              <div class="flex items-center gap-3">
                <h1 class="text-3xl font-bold">{{ release.version_next }}</h1>
                <span v-if="release.is_active" class="flex items-center gap-1 rounded-full bg-blue-100 px-2 py-0.5 text-xs text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                  <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-blue-500"></span>
                  Active
                </span>
              </div>
              <p class="mt-1 text-muted-foreground">
                from {{ release.version_current || 'initial' }} ·
                {{ release.bump_kind }} bump ·
                {{ release.commit_count }} commits
              </p>
            </div>
            <div class="flex items-center gap-3">
              <span :class="['badge', getRiskLevelClass(release.risk_level)]">
                Risk: {{ release.risk_score }} ({{ release.risk_level }})
              </span>
              <span :class="['badge', getStateClass(release.state)]">
                {{ release.state }}
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <div class="border-b">
        <nav class="flex gap-4">
          <button
            @click="activeTab = 'overview'"
            :class="[
              'border-b-2 px-4 py-2 text-sm font-medium transition-colors',
              activeTab === 'overview'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            ]"
          >
            Overview
          </button>
          <button
            @click="activeTab = 'commits'"
            :class="[
              'border-b-2 px-4 py-2 text-sm font-medium transition-colors',
              activeTab === 'commits'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            ]"
          >
            Commits ({{ release.commits?.length || 0 }})
          </button>
          <button
            @click="activeTab = 'events'"
            :class="[
              'border-b-2 px-4 py-2 text-sm font-medium transition-colors',
              activeTab === 'events'
                ? 'border-primary text-primary'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            ]"
          >
            Events ({{ events.length }})
          </button>
        </nav>
      </div>

      <!-- Overview tab -->
      <div v-if="activeTab === 'overview'" class="grid gap-6 lg:grid-cols-2">
        <!-- Steps progress -->
        <div class="card">
          <div class="card-header">
            <h2 class="card-title">Release Steps</h2>
          </div>
          <div class="card-content">
            <div class="space-y-4">
              <div
                v-for="step in release.steps"
                :key="step.name"
                class="flex items-center gap-4"
              >
                <div :class="['flex h-8 w-8 items-center justify-center rounded-full border-2', getStepStateClass(step.state)]">
                  <svg v-if="step.state === 'done'" class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  </svg>
                  <svg v-else-if="step.state === 'running'" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  <svg v-else-if="step.state === 'failed'" class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd" />
                  </svg>
                  <span v-else class="h-2 w-2 rounded-full bg-current"></span>
                </div>
                <div class="flex-1">
                  <div class="font-medium">{{ step.name }}</div>
                  <div v-if="step.error" class="text-sm text-red-500">{{ step.error }}</div>
                  <div v-else-if="step.completed_at" class="text-xs text-muted-foreground">
                    Completed {{ formatDate(step.completed_at) }}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Approval info -->
        <div class="card">
          <div class="card-header">
            <h2 class="card-title">Approval</h2>
          </div>
          <div class="card-content">
            <div v-if="release.approval" class="space-y-4">
              <div class="flex items-center gap-2">
                <svg v-if="release.approval.approved" class="h-5 w-5 text-green-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
                <svg v-else class="h-5 w-5 text-yellow-500" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-11a1 1 0 10-2 0v2H7a1 1 0 100 2h2v2a1 1 0 102 0v-2h2a1 1 0 100-2h-2V7z" clip-rule="evenodd" />
                </svg>
                <span class="font-medium">
                  {{ release.approval.approved ? 'Approved' : 'Pending' }}
                </span>
                <span v-if="release.approval.auto_approved" class="badge bg-muted">
                  Auto-approved
                </span>
              </div>
              <div v-if="release.approval.approved_by" class="text-sm text-muted-foreground">
                By: {{ release.approval.approved_by }}
              </div>
              <div v-if="release.approval.approved_at" class="text-sm text-muted-foreground">
                At: {{ formatDate(release.approval.approved_at) }}
              </div>
              <div v-if="release.approval.justification" class="rounded-md bg-muted p-3 text-sm">
                {{ release.approval.justification }}
              </div>
            </div>
            <div v-else class="text-muted-foreground">
              Not yet approved
            </div>
          </div>
        </div>

        <!-- Release notes -->
        <div class="card lg:col-span-2">
          <div class="card-header">
            <h2 class="card-title">Release Notes</h2>
          </div>
          <div class="card-content">
            <div v-if="release.notes" class="prose prose-sm max-w-none dark:prose-invert">
              <pre class="whitespace-pre-wrap font-sans">{{ release.notes }}</pre>
            </div>
            <div v-else class="text-muted-foreground">
              No release notes generated yet
            </div>
          </div>
        </div>
      </div>

      <!-- Commits tab -->
      <div v-if="activeTab === 'commits'" class="card">
        <div class="card-content p-0">
          <div v-if="!release.commits?.length" class="py-8 text-center text-muted-foreground">
            No commits in this release
          </div>
          <div v-else class="divide-y">
            <div
              v-for="commit in release.commits"
              :key="commit.sha"
              class="flex items-start gap-4 p-4"
            >
              <div class="flex-1">
                <div class="flex items-center gap-2">
                  <span v-if="commit.breaking" class="badge bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400">
                    BREAKING
                  </span>
                  <span class="badge bg-muted">{{ commit.type }}</span>
                  <span v-if="commit.scope" class="text-sm text-muted-foreground">({{ commit.scope }})</span>
                </div>
                <p class="mt-1 font-medium">{{ commit.message }}</p>
                <div class="mt-2 flex items-center gap-4 text-sm text-muted-foreground">
                  <span>{{ commit.author }}</span>
                  <span>{{ formatDate(commit.date) }}</span>
                  <code class="rounded bg-muted px-1.5 py-0.5 text-xs">{{ commit.sha.substring(0, 7) }}</code>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Events tab -->
      <div v-if="activeTab === 'events'" class="card">
        <div class="card-content p-0">
          <div v-if="eventsLoading" class="flex items-center justify-center py-8">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
          </div>
          <div v-else-if="!events.length" class="py-8 text-center text-muted-foreground">
            No events recorded
          </div>
          <div v-else class="divide-y">
            <div
              v-for="event in events"
              :key="event.id"
              class="flex items-start gap-4 p-4"
            >
              <div class="flex-1">
                <div class="font-medium">{{ event.type }}</div>
                <div class="mt-1 text-sm text-muted-foreground">
                  {{ formatDate(event.timestamp) }} · Actor: {{ event.actor_id.substring(0, 8) }}...
                </div>
                <pre v-if="Object.keys(event.data).length > 0" class="mt-2 rounded bg-muted p-2 text-xs">{{ JSON.stringify(event.data, null, 2) }}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
