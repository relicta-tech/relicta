import { test, expect } from '@playwright/test'
import AxeBuilder from '@axe-core/playwright'

/**
 * a11y gate for the main dashboard + governance analytics.
 *
 * Less critical than ApprovalWorkflow but covers the most-visited views.
 * Catches header / nav / chart-region drift early.
 */
test.describe('Dashboard a11y', () => {
  test('dashboard root view passes WCAG 2 AA', async ({ page }) => {
    await page.goto('/')

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa'])
      .analyze()

    expect(results.violations, formatViolations(results.violations)).toEqual([])
  })

  test('governance analytics page passes WCAG 2 AA', async ({ page }) => {
    await page.goto('/governance/analytics')

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa'])
      // Charts render via canvas — exclude from rule scope, document
      // separately. Tabular fallback should be tested via separate snapshot
      // when added.
      .exclude('canvas')
      .analyze()

    expect(results.violations, formatViolations(results.violations)).toEqual([])
  })

  test('release detail view passes WCAG 2 AA', async ({ page, context }) => {
    await context.route('**/api/v1/releases/*', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'rel-stub',
          version_current: '1.4.0',
          version_next: '1.4.1',
          state: 'approved',
          bump_kind: 'patch',
          risk_score: 0.3,
          risk_level: 'low',
          commit_count: 3,
          created_at: '2026-05-01T10:00:00Z',
          updated_at: '2026-05-01T10:30:00Z',
          actor_id: 'alice@example.com',
          actor_kind: 'human',
          is_active: true,
        }),
      }),
    )

    await page.goto('/releases/rel-stub')

    const results = await new AxeBuilder({ page })
      .withTags(['wcag2a', 'wcag2aa'])
      .analyze()

    expect(results.violations, formatViolations(results.violations)).toEqual([])
  })
})

function formatViolations(violations: Array<{ id: string; description: string; nodes: unknown[] }>): string {
  if (violations.length === 0) return ''
  return violations
    .map(v => `  - [${v.id}] ${v.description} — ${v.nodes.length} node(s)`)
    .join('\n')
}
