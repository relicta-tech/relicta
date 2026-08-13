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

// Analytics types
export type Granularity = 'day' | 'week' | 'month'

export interface AnalyticsRiskTrendPoint {
  date: string
  avg_score: number
  max_score: number
  release_count: number
}

export interface AnalyticsRiskTrendsResponse {
  trends: AnalyticsRiskTrendPoint[]
  granularity: Granularity
}

export interface AnalyticsDecisionCounts {
  approve: number
  deny: number
  require_review: number
  total: number
}

export interface AnalyticsDecisionsResponse {
  counts: AnalyticsDecisionCounts
  by_period: { date: string; approve: number; deny: number; require_review: number }[]
}

export interface AnalyticsTeamMember {
  actor_id: string
  actor_name: string
  approvals: number
  denials: number
  releases: number
  avg_cycle_time_hours: number
  success_rate: number
}

export interface AnalyticsTeamResponse {
  members: AnalyticsTeamMember[]
  total_releases: number
  avg_velocity: number
}

// Multi-repo types
export interface RepoGroup {
  name: string
  description: string
  repo_count: number
  last_release: string
  status: 'healthy' | 'warning' | 'error'
}

export interface RepoGroupStatus {
  name: string
  repos: RepoStatus[]
}

export interface RepoStatus {
  name: string
  version: string
  state: ReleaseState
  risk_level: RiskLevel
  last_release: string
  has_pending_changes: boolean
}

export interface RepoGroupGraph {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface GraphNode {
  id: string
  name: string
  version: string
  state: ReleaseState
}

export interface GraphEdge {
  from: string
  to: string
  label?: string
}

// Observability types
export interface ObservabilityHealth {
  overall: 'healthy' | 'degraded' | 'unhealthy'
  deployments: DeploymentHealth[]
}

export interface DeploymentHealth {
  release_id: string
  version: string
  status: 'healthy' | 'degraded' | 'unhealthy'
  error_rate: number
  latency_p99_ms: number
  deployed_at: string
  thresholds: {
    error_rate_max: number
    latency_p99_max_ms: number
  }
}

export interface ObservabilityProvider {
  name: string
  type: string
  status: 'connected' | 'disconnected' | 'error'
  last_check: string
  error_message?: string
}

export interface IncidentCorrelation {
  incident_id: string
  release_id: string
  version: string
  confidence: number
  detected_at: string
  description: string
  status: 'open' | 'resolved' | 'investigating'
}

// Release Memory types
export interface MemoryTrends {
  success_rate: number
  mtbf_hours: number
  total_releases: number
  rollbacks: number
  incidents: number
  trend_direction: 'improving' | 'stable' | 'declining'
  history: MemoryTrendPoint[]
}

export interface MemoryTrendPoint {
  date: string
  success_rate: number
  releases: number
}

export interface ReleaseInsight {
  id: string
  type: 'risk_pattern' | 'recommendation' | 'anomaly'
  title: string
  description: string
  confidence: number
  related_releases: string[]
  detected_at: string
}

export interface MemoryInsightsResponse {
  insights: ReleaseInsight[]
  recent_outcomes: ReleaseOutcome[]
}

export interface ReleaseOutcome {
  release_id: string
  version: string
  outcome: 'success' | 'rollback' | 'incident'
  completed_at: string
  notes?: string
}

// WebSocket message types
export interface WebSocketMessage {
  type: string
  payload: Record<string, unknown>
}

export type WebSocketEventType =
  // The wildcard is a supported subscription target — useWebSocket dispatches to
  // handlers.get('*') for every message — and leaving it out of the union is why App.vue
  // subscribed with `'*' as any`. A cast that exists because the type is wrong teaches the
  // next reader that casts are normal here.
  | '*'
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
