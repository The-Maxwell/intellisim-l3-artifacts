import { defineConfig } from '@playwright/test'

// Two tiers:
//  - core: deterministic, no LLM calls (default; excludes @llm-titled tests)
//  - llm:  tests titled with @llm — real provider calls, long timeouts
export default defineConfig({
  testDir: './e2e',
  timeout: 45_000,
  retries: 0,
  // Suites share one backend, one Mongo, one engine (4-run capacity) and
  // assert on shared list views — parallel spec files interfere (verified
  // 2026-07-12: 7 contention flakes at --workers=4, all green serially).
  // Override with --workers=N for a faster, flake-tolerant run.
  workers: 1,
  globalSetup: './e2e/_helpers/global-setup',
  use: {
    baseURL: 'http://localhost:3000',
    headless: true,
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'core', grepInvert: /@llm/ },
    { name: 'llm', grep: /@llm/, timeout: 300_000 },
  ],
  webServer: {
    command: 'npm run dev',
    port: 3000,
    reuseExistingServer: true,
  },
})
