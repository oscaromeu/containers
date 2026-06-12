import { expect, test } from '@playwright/test'

// Example probe targeting podinfo, in the Flanks probe format (per-step `log()`
// JSON lines + `step N:` naming). NOT baked into the image — copy it into a
// ConfigMap and mount it at PW_TEST_DIR. Target via env: PW_BASE_URL=https://podinfo.<domain>
//
// podinfo is public, so there's no login/beforeAll guard. For authenticated
// probes, mirror the Flanks tests: import { log, login } and start with a
// `step 1: log in` step.

const log = (message: string, extra: Record<string, unknown> = {}): void => {
  console.log(JSON.stringify({ level: 'info', message, ts: new Date().toISOString(), ...extra }))
}

const TIMEOUTS = {
  navigation: 30_000,
  heading: 15_000,
  action: 15_000,
} as const

test('podinfo: web UI is served', async ({ page }) => {
  await test.step('step 1: load the home page', async () => {
    log('step 1: load the home page', { url: process.env.PW_BASE_URL ?? '' })
    const resp = await page.goto('/', { waitUntil: 'load', timeout: TIMEOUTS.navigation })
    expect(resp?.ok(), 'home page should return a 2xx status').toBeTruthy()
  })

  await test.step('step 2: the page renders content', async () => {
    log('step 2: the page renders content')
    const body = await page.locator('body').innerText()
    expect(body.trim().length, 'body should not be blank').toBeGreaterThan(0)
  })
})

test('podinfo: API endpoints are healthy', async ({ request }) => {
  await test.step('step 1: GET /healthz returns 200', async () => {
    log('step 1: GET /healthz returns 200')
    const res = await request.get('/healthz', { timeout: TIMEOUTS.action })
    expect(res.status()).toBe(200)
  })

  await test.step('step 2: GET /api/info reports a version', async () => {
    log('step 2: GET /api/info reports a version')
    const res = await request.get('/api/info', { timeout: TIMEOUTS.action })
    expect(res.ok(), '/api/info should return a 2xx status').toBeTruthy()
    const body = await res.json()
    expect(body.version, 'response should carry a version field').toBeTruthy()
    log('version reported', { version: body.version })
  })
})
