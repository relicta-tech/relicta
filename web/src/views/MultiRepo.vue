<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import * as api from '@/api/client'
import type {
  RepoGroup,
  RepoGroupStatus,
  RepoGroupGraph,
  GraphNode,
} from '@/types/api'

const loading = ref(true)
const error = ref<string | null>(null)

const groups = ref<RepoGroup[]>([])
const selectedGroup = ref<string | null>(null)
const groupStatus = ref<RepoGroupStatus | null>(null)
const groupGraph = ref<RepoGroupGraph | null>(null)
const loadingDetail = ref(false)

onMounted(async () => {
  try {
    const res = await api.getGroups()
    groups.value = res.groups
  } catch (err) {
    error.value = err instanceof Error ? err.message : 'Failed to load repository groups'
    console.error('Failed to load groups:', err)
  } finally {
    loading.value = false
  }
})

async function selectGroup(name: string) {
  if (selectedGroup.value === name) {
    selectedGroup.value = null
    groupStatus.value = null
    groupGraph.value = null
    return
  }
  selectedGroup.value = name
  loadingDetail.value = true
  try {
    const [statusRes, graphRes] = await Promise.all([
      api.getGroupStatus(name),
      api.getGroupGraph(name),
    ])
    groupStatus.value = statusRes
    groupGraph.value = graphRes
  } catch (err) {
    console.error('Failed to load group details:', err)
  } finally {
    loadingDetail.value = false
  }
}

function getGroupStatusColor(status: string): string {
  switch (status) {
    case 'healthy': return 'bg-green-500'
    case 'warning': return 'bg-yellow-500'
    case 'error': return 'bg-red-500'
    default: return 'bg-gray-500'
  }
}

function getGroupStatusBorder(status: string): string {
  switch (status) {
    case 'healthy': return 'border-green-200 dark:border-green-800'
    case 'warning': return 'border-yellow-200 dark:border-yellow-800'
    case 'error': return 'border-red-200 dark:border-red-800'
    default: return ''
  }
}

function getRepoStateClass(state: string): string {
  return `badge-state-${state}`
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
  if (!dateString) return 'Never'
  return new Date(dateString).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

// Graph layout: simple left-to-right box layout using topological-ish ordering
const graphLayout = computed(() => {
  if (!groupGraph.value) return { nodes: [], edges: [] }

  const g = groupGraph.value
  const nodeMap = new Map<string, GraphNode>()
  g.nodes.forEach(n => nodeMap.set(n.id, n))

  // Determine depth using edges (from -> to means dependency)
  const depths = new Map<string, number>()
  const inDegree = new Map<string, number>()
  g.nodes.forEach(n => {
    depths.set(n.id, 0)
    inDegree.set(n.id, 0)
  })
  g.edges.forEach(e => {
    inDegree.set(e.to, (inDegree.get(e.to) || 0) + 1)
  })

  // BFS for depth
  const queue: string[] = []
  g.nodes.forEach(n => {
    if ((inDegree.get(n.id) || 0) === 0) queue.push(n.id)
  })
  while (queue.length > 0) {
    const id = queue.shift()!
    const d = depths.get(id) || 0
    g.edges
      .filter(e => e.from === id)
      .forEach(e => {
        const newDepth = d + 1
        if (newDepth > (depths.get(e.to) || 0)) {
          depths.set(e.to, newDepth)
        }
        queue.push(e.to)
      })
  }

  // Group by depth
  const columns = new Map<number, string[]>()
  depths.forEach((d, id) => {
    if (!columns.has(d)) columns.set(d, [])
    columns.get(d)!.push(id)
  })

  const colWidth = 200
  const rowHeight = 80
  const boxWidth = 160
  const boxHeight = 56
  const offsetX = 20
  const offsetY = 20

  const positions = new Map<string, { x: number; y: number }>()
  columns.forEach((ids, col) => {
    ids.forEach((id, row) => {
      positions.set(id, {
        x: offsetX + col * colWidth,
        y: offsetY + row * rowHeight,
      })
    })
  })

  const maxCol = Math.max(...Array.from(columns.keys()), 0)
  const maxRow = Math.max(...Array.from(columns.values()).map(ids => ids.length), 0)
  const svgWidth = offsetX * 2 + (maxCol + 1) * colWidth
  const svgHeight = offsetY * 2 + maxRow * rowHeight

  const layoutNodes = g.nodes.map(n => {
    const pos = positions.get(n.id) || { x: 0, y: 0 }
    return { ...n, x: pos.x, y: pos.y, w: boxWidth, h: boxHeight }
  })

  const layoutEdges = g.edges.map(e => {
    const fromPos = positions.get(e.from) || { x: 0, y: 0 }
    const toPos = positions.get(e.to) || { x: 0, y: 0 }
    return {
      ...e,
      x1: fromPos.x + boxWidth,
      y1: fromPos.y + boxHeight / 2,
      x2: toPos.x,
      y2: toPos.y + boxHeight / 2,
    }
  })

  return { nodes: layoutNodes, edges: layoutEdges, width: svgWidth, height: svgHeight }
})

function getNodeStateColor(state: string): string {
  switch (state) {
    case 'published': return '#22c55e'
    case 'approved': return '#3b82f6'
    case 'failed': return '#ef4444'
    case 'canceled': return '#6b7280'
    default: return 'var(--color-primary)'
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h2 class="text-lg font-semibold">
          Repository Groups
        </h2>
        <p class="text-sm text-muted-foreground">
          Multi-repo coordination and dependency management
        </p>
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
          @click="loading = true; api.getGroups().then(r => { groups = r.groups; error = null }).catch(e => { error = e.message }).finally(() => loading = false)"
        >
          Retry
        </button>
      </div>
    </div>

    <!-- Empty state -->
    <div
      v-else-if="groups.length === 0"
      class="card"
    >
      <div class="card-content py-12 text-center text-muted-foreground">
        No repository groups configured. Add groups in your configuration to enable multi-repo coordination.
      </div>
    </div>

    <!-- Group cards -->
    <div
      v-else
      class="grid gap-4 md:grid-cols-2 lg:grid-cols-3"
    >
      <button
        v-for="group in groups"
        :key="group.name"
        :class="[
          'card cursor-pointer text-left transition-all hover:shadow-md',
          selectedGroup === group.name ? 'ring-2 ring-primary' : '',
          getGroupStatusBorder(group.status),
        ]"
        @click="selectGroup(group.name)"
      >
        <div class="card-content pt-6">
          <div class="flex items-center justify-between">
            <h3 class="font-semibold">
              {{ group.name }}
            </h3>
            <span :class="['h-2.5 w-2.5 rounded-full', getGroupStatusColor(group.status)]" />
          </div>
          <p class="mt-1 text-sm text-muted-foreground">
            {{ group.description }}
          </p>
          <div class="mt-3 flex items-center gap-4 text-xs text-muted-foreground">
            <span>{{ group.repo_count }} repos</span>
            <span>Last release: {{ formatDate(group.last_release) }}</span>
          </div>
        </div>
      </button>
    </div>

    <!-- Selected group detail -->
    <div
      v-if="selectedGroup"
      class="space-y-6"
    >
      <!-- Dependency graph -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">
            Dependency Graph: {{ selectedGroup }}
          </h2>
          <p class="card-description">
            Repository dependencies and release state
          </p>
        </div>
        <div class="card-content">
          <div
            v-if="loadingDetail"
            class="flex items-center justify-center py-8"
          >
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          </div>
          <div
            v-else-if="!groupGraph || groupGraph.nodes.length === 0"
            class="py-8 text-center text-muted-foreground"
          >
            No dependency graph data available
          </div>
          <div
            v-else
            class="overflow-x-auto"
          >
            <svg
              :viewBox="`0 0 ${graphLayout.width} ${graphLayout.height}`"
              :style="{ minWidth: `${Math.min(graphLayout.width, 800)}px` }"
              class="w-full"
              preserveAspectRatio="xMinYMin meet"
            >
              <defs>
                <marker
                  id="arrowhead"
                  markerWidth="10"
                  markerHeight="7"
                  refX="10"
                  refY="3.5"
                  orient="auto"
                >
                  <polygon
                    points="0 0, 10 3.5, 0 7"
                    fill="currentColor"
                    class="text-muted-foreground"
                  />
                </marker>
              </defs>
              <!-- Edges -->
              <line
                v-for="(edge, i) in graphLayout.edges"
                :key="`edge-${i}`"
                :x1="edge.x1"
                :y1="edge.y1"
                :x2="edge.x2"
                :y2="edge.y2"
                stroke="currentColor"
                stroke-width="1.5"
                class="text-muted-foreground"
                marker-end="url(#arrowhead)"
              >
                <title v-if="edge.label">{{ edge.label }}</title>
              </line>
              <!-- Nodes -->
              <g
                v-for="node in graphLayout.nodes"
                :key="node.id"
              >
                <rect
                  :x="node.x"
                  :y="node.y"
                  :width="node.w"
                  :height="node.h"
                  rx="8"
                  class="fill-card stroke-border"
                  stroke-width="1.5"
                />
                <rect
                  :x="node.x"
                  :y="node.y"
                  width="4"
                  :height="node.h"
                  rx="2"
                  :fill="getNodeStateColor(node.state)"
                />
                <text
                  :x="node.x + 14"
                  :y="node.y + 22"
                  class="fill-foreground text-[13px] font-medium"
                >
                  {{ node.name }}
                </text>
                <text
                  :x="node.x + 14"
                  :y="node.y + 40"
                  class="fill-muted-foreground text-[11px]"
                >
                  {{ node.version }} - {{ node.state }}
                </text>
              </g>
            </svg>
          </div>
        </div>
      </div>

      <!-- Per-repo status -->
      <div class="card">
        <div class="card-header">
          <h2 class="card-title">
            Repository Status
          </h2>
          <p class="card-description">
            Current release state for each repository in the group
          </p>
        </div>
        <div class="card-content p-0">
          <div
            v-if="loadingDetail"
            class="flex items-center justify-center py-8"
          >
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          </div>
          <div
            v-else-if="!groupStatus || groupStatus.repos.length === 0"
            class="py-8 text-center text-muted-foreground"
          >
            No repository data available
          </div>
          <table
            v-else
            class="table"
          >
            <thead class="table-header">
              <tr>
                <th class="table-head">
                  Repository
                </th>
                <th class="table-head">
                  Version
                </th>
                <th class="table-head">
                  State
                </th>
                <th class="table-head">
                  Risk
                </th>
                <th class="table-head">
                  Last Release
                </th>
                <th class="table-head">
                  Pending
                </th>
              </tr>
            </thead>
            <tbody class="table-body">
              <tr
                v-for="repo in groupStatus.repos"
                :key="repo.name"
                class="table-row"
              >
                <td class="table-cell font-medium">
                  {{ repo.name }}
                </td>
                <td class="table-cell font-mono text-sm">
                  {{ repo.version }}
                </td>
                <td class="table-cell">
                  <span :class="['badge', getRepoStateClass(repo.state)]">{{ repo.state }}</span>
                </td>
                <td class="table-cell">
                  <span :class="['badge', getRiskLevelClass(repo.risk_level)]">{{ repo.risk_level }}</span>
                </td>
                <td class="table-cell text-sm text-muted-foreground">
                  {{ formatDate(repo.last_release) }}
                </td>
                <td class="table-cell">
                  <span
                    v-if="repo.has_pending_changes"
                    class="inline-flex h-2 w-2 rounded-full bg-yellow-500"
                    title="Has pending changes"
                  />
                  <span
                    v-else
                    class="text-muted-foreground"
                  >--</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Coordinated release action -->
      <div class="card">
        <div class="card-content flex items-center justify-between pt-6">
          <div>
            <h3 class="font-medium">
              Coordinated Release
            </h3>
            <p class="text-sm text-muted-foreground">
              Plan a release across all repositories in this group
            </p>
          </div>
          <button class="btn-primary">
            Plan Coordinated Release
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
