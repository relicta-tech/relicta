// ApprovalCard TypeScript bindings — single source of truth for the
// release-approval view across CLI / web / MCP / Hub surfaces.
//
// MUST stay structurally identical to `pkg/cgp/approval_card.go`. When the Go
// schema changes, mirror the change here. A future codegen step
// (openapi-typescript or jsonschema-to-ts) can replace this hand-maintained
// shim once the Go side exposes a schema endpoint.

export type RiskTier = 'low' | 'medium' | 'high' | 'critical'

/** Maps a numeric risk score to a tier — must match cgp.RiskTierForScore. */
export function riskTierForScore(score: number): RiskTier {
  if (score >= 0.85) return 'critical'
  if (score >= 0.7) return 'high'
  if (score >= 0.4) return 'medium'
  return 'low'
}

/** Returns the canonical severity glyph — must match cgp.RiskGlyphForTier. */
export function riskGlyphForTier(tier: RiskTier): string {
  switch (tier) {
    case 'critical':
      return '▲▲▲▲'
    case 'high':
      return '▲▲▲'
    case 'medium':
      return '▲▲'
    default:
      return '▲'
  }
}

/** Severity label as upper-case string for rendering. */
export function riskSeverityLabel(tier: RiskTier): string {
  return tier.toUpperCase()
}

export interface CardActor {
  kind: string
  id: string
  name?: string
}

export interface RiskFactor {
  category: string
  description: string
  score: number
  severity?: string
}

export interface RiskBlock {
  score: number
  tier: RiskTier
  severity: string
  factors?: RiskFactor[]
  glyph?: string
}

export type ApprovalActionType = 'primary' | 'secondary' | 'danger'

export interface ApprovalAction {
  id: string
  label: string
  description?: string
  type: ApprovalActionType
  requiresConfirmation?: boolean
  requiresCosigner?: boolean
}

export interface ApprovalCard {
  cgpVersion: string
  cardId: string
  releaseId: string
  version?: string
  repository?: string
  risk: RiskBlock
  diffSummary?: string
  actor: CardActor
  verifiers?: CardActor[]
  decision: string
  rationale?: string[]
  requiredActions?: ApprovalAction[]
  availableActions: ApprovalAction[]
  frameworks?: string[]
  auditChainHash?: string
  createdAt: string
}

/**
 * Converts a legacy ApprovalRequest API DTO into the canonical ApprovalCard
 * shape so the same UI component renders both legacy + new endpoints.
 *
 * Once the server returns ApprovalCard directly (when Hub adds the endpoint
 * planned in roady), this adapter goes away. Until then it bridges the
 * pre-canonical API surface to the canonical UI.
 */
export function approvalRequestToCard(req: {
  release_id: string
  version: string
  risk_score: number
  risk_level: RiskTier | string
  submitted_by: string
  submitted_at: string
  changes: string[]
  review_reason?: string
}): ApprovalCard {
  const score = req.risk_score
  const tier = (['low', 'medium', 'high', 'critical'].includes(req.risk_level)
    ? (req.risk_level as RiskTier)
    : riskTierForScore(score))

  return {
    cgpVersion: '0.1',
    cardId: `card:${req.release_id}`,
    releaseId: req.release_id,
    version: req.version,
    risk: {
      score,
      tier,
      severity: riskSeverityLabel(tier),
      glyph: riskGlyphForTier(tier),
      factors: [],
    },
    diffSummary: req.changes && req.changes.length > 0 ? req.changes.join('\n') : undefined,
    actor: {
      kind: 'human',
      id: req.submitted_by,
    },
    decision: 'approval_required',
    rationale: req.review_reason ? [req.review_reason] : undefined,
    availableActions: [
      { id: 'approve', label: 'Approve', type: 'primary' },
      { id: 'reject', label: 'Reject', type: 'danger', requiresConfirmation: true },
    ],
    createdAt: req.submitted_at,
  }
}

/** Tailwind utility classes per tier — keeps tier styling DRY across views. */
export function tierClasses(tier: RiskTier): {
  border: string
  text: string
  background: string
  bar: string
} {
  switch (tier) {
    case 'critical':
      return {
        border: 'border-red-700',
        text: 'text-red-700',
        background: 'bg-red-50',
        bar: 'bg-red-600',
      }
    case 'high':
      return {
        border: 'border-orange-600',
        text: 'text-orange-700',
        background: 'bg-orange-50',
        bar: 'bg-orange-500',
      }
    case 'medium':
      return {
        border: 'border-amber-500',
        text: 'text-amber-700',
        background: 'bg-amber-50',
        bar: 'bg-amber-400',
      }
    default:
      return {
        border: 'border-emerald-500',
        text: 'text-emerald-700',
        background: 'bg-emerald-50',
        bar: 'bg-emerald-500',
      }
  }
}
