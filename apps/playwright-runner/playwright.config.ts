import { defineConfig, devices } from '@playwright/test'

// Generic, env-driven Playwright runner config. Tests are NOT baked into the
// image — they are mounted at runtime into PW_TEST_DIR (e.g. from a ConfigMap).
// Everything tunable is read from PW_* env vars so the same image can run any
// probe by just changing the cronjob env + the mounted spec.

const num = (v: string | undefined, fallback: number): number =>
  v && !Number.isNaN(Number(v)) ? Number(v) : fallback

// `||` (not `??`) so an empty PW_BASE_URL="" also falls back, instead of leaving
// baseURL empty and failing later with a cryptic "invalid URL".
const BASE_URL = process.env.PW_BASE_URL || 'http://localhost'

export default defineConfig({
  testDir: process.env.PW_TEST_DIR || '/tests',
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: num(process.env.PW_RETRIES, 0),
  timeout: num(process.env.PW_TIMEOUT_MS, 90_000),
  globalTimeout: num(process.env.PW_GLOBAL_TIMEOUT_MS, 10 * 60_000),
  reporter: [
    // Step 1: structured stdout only (→ scraped into logs). The ClickHouse
    // reporter will be added here in step 5.
    ['./reporter-json-line.ts'],
    ['html', { outputFolder: process.env.PW_REPORT_DIR || 'playwright-report', open: 'never' }],
  ],
  use: {
    baseURL: BASE_URL,
    trace: (process.env.PW_TRACE as 'on' | 'off' | 'retain-on-failure') || 'retain-on-failure',
    screenshot: (process.env.PW_SCREENSHOT as 'on' | 'off' | 'only-on-failure') || 'only-on-failure',
    video: (process.env.PW_VIDEO as 'on' | 'off' | 'retain-on-failure') || 'off',
    actionTimeout: num(process.env.PW_ACTION_TIMEOUT_MS, 15_000),
    navigationTimeout: num(process.env.PW_NAV_TIMEOUT_MS, 30_000),
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
})
