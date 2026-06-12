import { test, expect } from '@playwright/test'

// Example probe targeting podinfo. This file is a reference — it is NOT baked
// into the image. Copy it into a ConfigMap and mount it at PW_TEST_DIR so the
// cronjob picks it up. Set the target via env:
//
//   PW_BASE_URL=https://podinfo.<domain>
//
// The steps are split with test.step() so per-step timings show up in the
// detail view once the ClickHouse reporter is wired.

test('podinfo: web UI is served', async ({ page }) => {
  await test.step('GET / returns 2xx', async () => {
    const resp = await page.goto('/')
    expect(resp?.ok(), 'home page should return a 2xx status').toBeTruthy()
  })

  await test.step('page is not blank', async () => {
    const body = await page.locator('body').innerText()
    expect(body.trim().length).toBeGreaterThan(0)
  })
})

test('podinfo: API endpoints are healthy', async ({ request }) => {
  await test.step('GET /healthz is 200', async () => {
    const res = await request.get('/healthz')
    expect(res.status()).toBe(200)
  })

  await test.step('GET /api/info reports a version', async () => {
    const res = await request.get('/api/info')
    expect(res.ok(), '/api/info should return a 2xx status').toBeTruthy()
    const body = await res.json()
    expect(body.version, 'response should carry a version field').toBeTruthy()
  })
})
