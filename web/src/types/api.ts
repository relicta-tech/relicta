// API response types matching backend DTOs

export type ReleaseState =
  | 'draft'
  | 'planned'
  | 'versioned'
  | 'notes_ready'
  | 'approved'
  | 'publishing'
  | 'published'
  | 'failed'
  | 'canceled'

export type RiskLevel = 'low' | 'medium' | 'high' | 'critical'
export type BumpKind = 'major' | 'minor' | 'patch' | 'none'

export interface Release {
  id: string
  version_current: string
  version_next: string
  state: ReleaseState
  bump_kind: BumpKind
  risk_score: number
  risk_level: RiskLevel
  commit_count: number
  created_at: string
  updated_at: string
  actor_id: string
  actor_kind: string
  is_active: boolean
}

export interface ReleaseDetails extends Release {
  commits: Commit[]
  reasons: string[]
  notes: string
  approval?: ApprovalInfo
  steps: ReleaseStep[]
}

export interface Commit {
  sha: string
  message: string
  author: string
  date: string
  type: string
  scope?: string
  breaking: boolean
}

export interface ApprovalInfo {
  approved: boolean
  plan_hash: string
  approved_by: string
  auto_approved: boolean
  justification?: string
  approved_at: string
}

export interface ReleaseStep {
  name: string
  state: 'pending' | 'running' | 'done' | 'failed' | 'skipped'
  started_at?: string
  completed_at?: string
  error?: string
}

export interface ReleaseEvent {
  id: string
  type: string
  release_id: string
  actor_id: string
  timestamp: string
  data: Record<string, unknown>
}

export interface ApprovalRequest {
  release_id: string
  version: string
  risk_score: number
  risk_level: RiskLevel
  requires_review: boolean
  review_reason?: string
  submitted_at: string
  submitted_by: string
  commit_count: number
  changes: string[]
}

export interface GovernanceDecision {
  id: string
  release_id: string
  decision: 'approve' | 'deny' | 'require_review' | 'pending'
  risk_score: number
  risk_level: RiskLevel
  factors: string[]
  requires_review: boolean
  review_reason?: string
  timestamp: string
  actor_id: string
  actor_kind: string
}

export interface RiskTrend {
  date: string
  risk_score: number
  releases: number
}

export interface FactorDistribution {
  factor: string
  count: number
  percentage: number
}

export interface Actor {
  id: string
  kind: string
  name: string
  release_count: number
  success_rate: number
  average_risk_score: number
  reliability_score: number
  last_seen: string
  trust_level: 'trusted' | 'standard' | 'probation'
}

export interface AuditEvent {
  id: string
  type: string
  release_id: string
  actor_id: string
  timestamp: string
  data: Record<string, unknown>
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface HealthResponse {
  status: 'healthy' | 'degraded' | 'unhealthy'
  version: string
  timestamp: string
}

// WebSocket message types
export interface WebSocketMessage {
  type: string
  payload: Record<string, unknown>
}

export type WebSocketEventType =
  | 'release.created'
  | 'release.state_changed'
  | 'release.versioned'
  | 'release.approved'
  | 'release.published'
  | 'release.failed'
  | 'release.canceled'
  | 'release.retried'
  | 'release.step_completed'
  | 'release.plugin_executed'
  | 'release.notes_updated'
  | 'release.event'
