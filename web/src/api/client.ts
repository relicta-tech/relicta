import type {
  Release,
  ReleaseDetails,
  ReleaseEvent,
  ApprovalRequest,
  GovernanceDecision,
  RiskTrend,
  FactorDistribution,
  Actor,
  AuditEvent,
  PaginatedResponse,
  HealthResponse,
  Granularity,
  AnalyticsRiskTrendsResponse,
  AnalyticsDecisionsResponse,
  AnalyticsTeamResponse,
  RepoGroup,
  RepoGroupStatus,
  RepoGroupGraph,
  ObservabilityHealth,
  ObservabilityProvider,
  IncidentCorrelation,
  MemoryTrends,
  MemoryInsightsResponse,
} from '@/types/api'

const API_BASE = '/api/v1'

class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    message: string
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const apiKey = localStorage.getItem('relicta_api_key')

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...options.headers,
  }

  if (apiKey) {
    ;(headers as Record<string, string>)['Authorization'] = `Bearer ${apiKey}`
  }

  const response = await fetch(`${API_BASE}${endpoint}`, {
    ...options,
    headers,
  })

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }))
    throw new ApiError(response.status, response.statusText, error.error || error.message || 'Unknown error')
  }

  return response.json()
}

// Health
export async function getHealth(): Promise<HealthResponse> {
  return request<HealthResponse>('/health')
}

// Releases
export async function listReleases(
  page = 1,
  pageSize = 20
): Promise<PaginatedResponse<Release>> {
  return request<PaginatedResponse<Release>>(
    `/releases?page=${page}&page_size=${pageSize}`
  )
}

export async function getActiveRelease(): Promise<Release | null> {
  try {
    return await request<Release>('/releases/active')
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) {
      return null
    }
    throw error
  }
}

export async function getRelease(id: string): Promise<ReleaseDetails> {
  return request<ReleaseDetails>(`/releases/${id}`)
}

export async function getReleaseEvents(
  id: string,
  page = 1,
  pageSize = 50
): Promise<PaginatedResponse<ReleaseEvent>> {
  return request<PaginatedResponse<ReleaseEvent>>(
    `/releases/${id}/events?page=${page}&page_size=${pageSize}`
  )
}

// Approvals
export async function listPendingApprovals(): Promise<
  PaginatedResponse<ApprovalRequest>
> {
  return request<PaginatedResponse<ApprovalRequest>>('/approvals/pending')
}

export async function approveRelease(
  id: string,
  options?: { justification?: string; force?: boolean }
): Promise<{ approved: boolean; run_id: string }> {
  return request(`/approvals/${id}/approve`, {
    method: 'POST',
    body: JSON.stringify(options || {}),
  })
}

export async function rejectRelease(
  id: string,
  reason: string
): Promise<{ rejected: boolean; run_id: string }> {
  return request(`/approvals/${id}/reject`, {
    method: 'POST',
    body: JSON.stringify({ reason }),
  })
}

// Governance
export async function listGovernanceDecisions(
  page = 1,
  pageSize = 20
): Promise<PaginatedResponse<GovernanceDecision>> {
  return request<PaginatedResponse<GovernanceDecision>>(
    `/governance/decisions?page=${page}&page_size=${pageSize}`
  )
}

export async function getRiskTrends(days = 30): Promise<{ trends: RiskTrend[] }> {
  return request<{ trends: RiskTrend[] }>(`/governance/risk-trends?days=${days}`)
}

export async function getFactorDistribution(): Promise<{
  factors: FactorDistribution[]
}> {
  return request<{ factors: FactorDistribution[] }>('/governance/factors')
}

// Actors
export async function listActors(): Promise<PaginatedResponse<Actor>> {
  return request<PaginatedResponse<Actor>>('/actors')
}

export async function getActor(id: string): Promise<Actor> {
  return request<Actor>(`/actors/${id}`)
}

// Audit
export async function listAuditEvents(options?: {
  page?: number
  limit?: number
  from?: string
  to?: string
  release_id?: string
  event_type?: string
  actor?: string
}): Promise<PaginatedResponse<AuditEvent>> {
  const params = new URLSearchParams()
  if (options?.page) params.set('page', String(options.page))
  if (options?.limit) params.set('limit', String(options.limit))
  if (options?.from) params.set('from', options.from)
  if (options?.to) params.set('to', options.to)
  if (options?.release_id) params.set('release_id', options.release_id)
  if (options?.event_type) params.set('event_type', options.event_type)
  if (options?.actor) params.set('actor', options.actor)

  const query = params.toString()
  return request<PaginatedResponse<AuditEvent>>(`/audit${query ? `?${query}` : ''}`)
}

// Analytics
export async function getAnalyticsRiskTrends(params: {
  from?: string
  to?: string
  granularity?: Granularity
}): Promise<AnalyticsRiskTrendsResponse> {
  const qs = new URLSearchParams()
  if (params.from) qs.set('from', params.from)
  if (params.to) qs.set('to', params.to)
  if (params.granularity) qs.set('granularity', params.granularity)
  const query = qs.toString()
  return request<AnalyticsRiskTrendsResponse>(`/analytics/risk-trends${query ? `?${query}` : ''}`)
}

export async function getAnalyticsDecisions(params: {
  from?: string
  to?: string
  granularity?: Granularity
}): Promise<AnalyticsDecisionsResponse> {
  const qs = new URLSearchParams()
  if (params.from) qs.set('from', params.from)
  if (params.to) qs.set('to', params.to)
  if (params.granularity) qs.set('granularity', params.granularity)
  const query = qs.toString()
  return request<AnalyticsDecisionsResponse>(`/analytics/decisions${query ? `?${query}` : ''}`)
}

export async function getAnalyticsTeam(params: {
  from?: string
  to?: string
}): Promise<AnalyticsTeamResponse> {
  const qs = new URLSearchParams()
  if (params.from) qs.set('from', params.from)
  if (params.to) qs.set('to', params.to)
  const query = qs.toString()
  return request<AnalyticsTeamResponse>(`/analytics/team${query ? `?${query}` : ''}`)
}

// Multi-repo groups
export async function getGroups(): Promise<{ groups: RepoGroup[] }> {
  return request<{ groups: RepoGroup[] }>('/groups')
}

export async function getGroupStatus(name: string): Promise<RepoGroupStatus> {
  return request<RepoGroupStatus>(`/groups/${encodeURIComponent(name)}/status`)
}

export async function getGroupGraph(name: string): Promise<RepoGroupGraph> {
  return request<RepoGroupGraph>(`/groups/${encodeURIComponent(name)}/graph`)
}

// Observability
export async function getObservabilityHealth(): Promise<ObservabilityHealth> {
  return request<ObservabilityHealth>('/observability/health')
}

export async function getObservabilityProviders(): Promise<{ providers: ObservabilityProvider[] }> {
  return request<{ providers: ObservabilityProvider[] }>('/observability/providers')
}

export async function getObservabilityCorrelations(
  releaseId?: string
): Promise<{ correlations: IncidentCorrelation[] }> {
  const query = releaseId ? `?release_id=${encodeURIComponent(releaseId)}` : ''
  return request<{ correlations: IncidentCorrelation[] }>(`/observability/correlations${query}`)
}

// Release Memory
export async function getMemoryTrends(
  window?: string
): Promise<MemoryTrends> {
  const query = window ? `?window=${encodeURIComponent(window)}` : ''
  return request<MemoryTrends>(`/memory/trends${query}`)
}

export async function getMemoryInsights(
  releaseId?: string
): Promise<MemoryInsightsResponse> {
  const query = releaseId ? `?release_id=${encodeURIComponent(releaseId)}` : ''
  return request<MemoryInsightsResponse>(`/memory/insights${query}`)
}

export { ApiError }
