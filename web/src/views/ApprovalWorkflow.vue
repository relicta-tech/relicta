<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import * as api from '@/api/client'
import type { ApprovalRequest, GovernanceDecision } from '@/types/api'

const pendingApprovals = ref<ApprovalRequest[]>([])
const recentDecisions = ref<GovernanceDecision[]>([])
const loading = ref(true)
const approving = ref<string | null>(null)
const rejecting = ref<string | null>(null)
const justification = ref('')
const rejectReason = ref('')
const selectedApproval = ref<ApprovalRequest | null>(null)
const showApproveModal = ref(false)
const showRejectModal = ref(false)

onMounted(async () => {
  await loadData()
})

async function loadData() {
  loading.value = true
  try {
    const [approvalsRes, decisionsRes] = await Promise.all([
      api.listPendingApprovals(),
      api.listGovernanceDecisions(1, 10),
    ])
    pendingApprovals.value = approvalsRes.data
    recentDecisions.value = decisionsRes.data.filter(d => d.decision !== 'pending')
  } catch (error) {
    console.error('Failed to load approval data:', error)
  } finally {
    loading.value = false
  }
}

function openApproveModal(approval: ApprovalRequest) {
  selectedApproval.value = approval
  justification.value = ''
  showApproveModal.value = true
}

function openRejectModal(approval: ApprovalRequest) {
  selectedApproval.value = approval
  rejectReason.value = ''
  showRejectModal.value = true
}

async function handleApprove() {
  if (!selectedApproval.value) return

  approving.value = selectedApproval.value.release_id
  try {
    await api.approveRelease(selectedApproval.value.release_id, {
      justification: justification.value || undefined,
    })
    showApproveModal.value = false
    await loadData()
  } catch (error) {
    console.error('Failed to approve:', error)
  } finally {
    approving.value = null
  }
}

async function handleReject() {
  if (!selectedApproval.value || !rejectReason.value) return

  rejecting.value = selectedApproval.value.release_id
  try {
    await api.rejectRelease(selectedApproval.value.release_id, rejectReason.value)
    showRejectModal.value = false
    await loadData()
  } catch (error) {
    console.error('Failed to reject:', error)
  } finally {
    rejecting.value = null
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

function getDecisionClass(decision: string): string {
  const classes: Record<string, string> = {
    approve: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
    deny: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
    require_review: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
  }
  return classes[decision] || 'bg-gray-100 text-gray-800'
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleString()
}

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString)
  const now = new Date()
  const diff = now.getTime() - date.getTime()
  const minutes = Math.floor(diff / 60000)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (days > 0) return `${days}d ago`
  if (hours > 0) return `${hours}h ago`
  if (minutes > 0) return `${minutes}m ago`
  return 'Just now'
}
</script>

<template>
  <div class="space-y-6">
    <!-- Pending approvals -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Pending Approvals</h2>
        <p class="card-description">
          {{ pendingApprovals.length }} release{{ pendingApprovals.length !== 1 ? 's' : '' }} awaiting review
        </p>
      </div>
      <div class="card-content">
        <div v-if="loading" class="flex items-center justify-center py-8">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        </div>
        <div v-else-if="pendingApprovals.length === 0" class="py-8 text-center text-muted-foreground">
          <svg class="mx-auto h-12 w-12 text-muted-foreground/50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p class="mt-2">All caught up! No pending approvals.</p>
        </div>
        <div v-else class="space-y-4">
          <div
            v-for="approval in pendingApprovals"
            :key="approval.release_id"
            class="rounded-lg border p-6"
          >
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="space-y-2">
                <div class="flex items-center gap-3">
                  <span class="text-xl font-bold">{{ approval.version }}</span>
                  <span :class="['badge', getRiskLevelClass(approval.risk_level)]">
                    Risk: {{ approval.risk_score }} ({{ approval.risk_level }})
                  </span>
                </div>
                <div class="text-sm text-muted-foreground">
                  {{ approval.commit_count }} commits · Submitted {{ formatRelativeTime(approval.submitted_at) }} by {{ approval.submitted_by }}
                </div>
                <div v-if="approval.review_reason" class="flex items-center gap-2 rounded-md bg-yellow-50 px-3 py-2 text-sm text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400">
                  <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
                  </svg>
                  <span>{{ approval.review_reason }}</span>
                </div>
                <div v-if="approval.changes?.length" class="space-y-1">
                  <div class="text-sm font-medium">Changes:</div>
                  <ul class="list-inside list-disc text-sm text-muted-foreground">
                    <li v-for="change in approval.changes.slice(0, 5)" :key="change">{{ change }}</li>
                    <li v-if="approval.changes.length > 5" class="text-xs">
                      +{{ approval.changes.length - 5 }} more...
                    </li>
                  </ul>
                </div>
              </div>
              <div class="flex gap-2">
                <RouterLink :to="`/releases/${approval.release_id}`" class="btn-outline btn-sm">
                  View Details
                </RouterLink>
                <button
                  @click="openRejectModal(approval)"
                  :disabled="rejecting === approval.release_id"
                  class="btn-destructive btn-sm"
                >
                  Reject
                </button>
                <button
                  @click="openApproveModal(approval)"
                  :disabled="approving === approval.release_id"
                  class="btn-primary btn-sm"
                >
                  <span v-if="approving === approval.release_id" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
                  Approve
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Recent decisions -->
    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Recent Decisions</h2>
        <p class="card-description">Latest approval decisions</p>
      </div>
      <div class="card-content p-0">
        <div v-if="recentDecisions.length === 0" class="py-8 text-center text-muted-foreground">
          No recent decisions
        </div>
        <table v-else class="table">
          <thead class="table-header">
            <tr>
              <th class="table-head">Release</th>
              <th class="table-head">Decision</th>
              <th class="table-head">Risk</th>
              <th class="table-head">By</th>
              <th class="table-head">Time</th>
            </tr>
          </thead>
          <tbody class="table-body">
            <tr
              v-for="decision in recentDecisions"
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
                  {{ decision.risk_score }}
                </span>
              </td>
              <td class="table-cell text-sm">
                {{ decision.actor_id.substring(0, 8) }}...
              </td>
              <td class="table-cell text-sm text-muted-foreground">
                {{ formatDate(decision.timestamp) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Approve Modal -->
    <Teleport to="body">
      <div v-if="showApproveModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
        <div class="card w-full max-w-md">
          <div class="card-header">
            <h2 class="card-title">Approve Release</h2>
            <p class="card-description">
              Approve {{ selectedApproval?.version }} for publishing
            </p>
          </div>
          <div class="card-content">
            <label class="block">
              <span class="text-sm font-medium">Justification (optional)</span>
              <textarea
                v-model="justification"
                rows="3"
                class="input mt-1"
                placeholder="Reason for approval..."
              ></textarea>
            </label>
          </div>
          <div class="card-footer justify-end gap-2 border-t pt-4">
            <button @click="showApproveModal = false" class="btn-ghost btn-sm">
              Cancel
            </button>
            <button
              @click="handleApprove"
              :disabled="approving !== null"
              class="btn-primary btn-sm"
            >
              <span v-if="approving" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
              Approve
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Reject Modal -->
    <Teleport to="body">
      <div v-if="showRejectModal" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
        <div class="card w-full max-w-md">
          <div class="card-header">
            <h2 class="card-title">Reject Release</h2>
            <p class="card-description">
              Reject {{ selectedApproval?.version }}
            </p>
          </div>
          <div class="card-content">
            <label class="block">
              <span class="text-sm font-medium">Reason (required)</span>
              <textarea
                v-model="rejectReason"
                rows="3"
                class="input mt-1"
                placeholder="Reason for rejection..."
                required
              ></textarea>
            </label>
          </div>
          <div class="card-footer justify-end gap-2 border-t pt-4">
            <button @click="showRejectModal = false" class="btn-ghost btn-sm">
              Cancel
            </button>
            <button
              @click="handleReject"
              :disabled="rejecting !== null || !rejectReason"
              class="btn-destructive btn-sm"
            >
              <span v-if="rejecting" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
              Reject
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
