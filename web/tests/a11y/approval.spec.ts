import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

/**
 * a11y gate for ApprovalWorkflow.
 *
 * Approval is the highest-stakes view in the dashboard — risk score, color
 * coding, decision buttons. Compliance/security buyers in regulated
 * industries often use screen readers; an a11y regression here blocks
 * sales motions explicitly.
 *
 * Tags WCAG 2 AA + 2.1 AA + best practices. axe-core's "wcag2aa" tag is
 * the auditor-defensible bar most procurement teams require.
 */
test.describe('ApprovalWorkflow a11y', () => {
  test('zero violations on empty pending list', async ({ page }) => {
    await page.goto('/approvals')

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa', 'wcag21aa', 'best-practice'])
      .analyze()

    // Snapshot violations into the report so reviewers see exactly what failed.
    expect(
      results.violations,
      formatAxeViolations(results.violations),
    ).toEqual([])
  })

  test('zero violations on populated approval list (mocked)', async ({ page, context }) => {
    // Stub the pending-approvals endpoint with a representative payload so
    // the test exercises every render branch (risk badges, factor lists,
    // action buttons) without depending on a live backend.
    await context.route('**/api/v1/approvals/pending', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: [
            {
              release_id: 'rel-001',
              version: '1.4.1',
              risk_score: 0.65,
              risk_level: 'medium',
              requires_review: true,
              review_reason: 'breaking change touches authentication',
              submitted_at: '2026-05-01T10:00:00Z',
              submitted_by: 'alice@example.com',
              commit_count: 5,
              changes: ['fix: webhook race', 'fix: token refresh nil-ptr'],
            },
            {
              release_id: 'rel-002',
              version: '2.0.0',
              risk_score: 0.92,
              risk_level: 'critical',
              requires_review: true,
              submitted_at: '2026-05-01T11:00:00Z',
              submitted_by: 'claude-code-1',
              commit_count: 12,
              changes: [],
            },
          ],
        }),
      }),
    )

    // loadData() fetches decisions in the same Promise.all as approvals —
    // without this stub the dead backend proxy rejects the whole load and
    // no cards render.
    await context.route('**/api/v1/governance/decisions**', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: [] }),
      }),
    )

    await page.goto('/approvals')
    await page.waitForSelector('[data-testid="approval-card"]')

    const results = await new AxeBuilder({ page })
      .include('[data-testid="approval-card"]')
      .withTags(['wcag2a', 'wcag2aa', 'wcag21aa'])
      .analyze()

    expect(
      results.violations,
      formatAxeViolations(results.violations),
    ).toEqual([])
  })

  test('high-contrast mode passes color-contrast checks', async ({ page }) => {
    await page.emulateMedia({ colorScheme: 'dark', forcedColors: 'active' })
    await page.goto('/approvals')

    const results = await new AxeBuilder({ page })
      .withTags(['cat.color'])
      .analyze()

    expect(
      results.violations,
      formatAxeViolations(results.violations),
    ).toEqual([])
  })
})

/**
 * formatAxeViolations renders an axe violation set into a CI-friendly
 * multi-line string so reviewers see what failed without opening the HTML
 * report. Empty-array case returns "" which keeps successful runs silent.
 */
function formatAxeViolations(violations: Array<{ id: string; description: string; nodes: unknown[] }>): string {
  if (violations.length === 0) return ''
  return violations
    .map(v => `  - [${v.id}] ${v.description} — ${v.nodes.length} node(s)`)
    .join('\n')
}
