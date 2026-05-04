import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright config for Relicta dashboard accessibility tests.
 *
 * Scope: a11y-only. Functional E2E tests live elsewhere (component tests
 * via vitest, contract tests against API client). Keep the surface narrow
 * so the CI gate is fast and deterministic.
 *
 * Browser: chromium-only by default — axe-core results are the same across
 * engines and adding firefox/webkit triples CI runtime for no signal gain.
 */
export default defineConfig({
  testDir: './tests/a11y',
  timeout: 30_000,
  expect: { timeout: 5_000 },
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['github'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:5173',
    trace: 'on-first-retry',
  },
  webServer: {
    command: 'npm run dev',
    url: 'http://localhost:5173',
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
