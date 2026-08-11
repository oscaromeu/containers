// @probe/playwright — baked test instrumentation fixture for the runner image.
//
// Specs import { test, expect } from '@probe/playwright' (instead of
// '@playwright/test') and transparently get, per test, a HAR (network.har)
// attached to the run. Playwright copies attachments into the HTML report, so
// the HAR travels with the report that run-and-upload.sh ships to object
// storage: per-request evidence for a specific failed run, not a time series.
//
// Resolved from /app/node_modules via NODE_PATH, exactly like @playwright/test.
// Shipped as plain CommonJS so Node loads it directly (Playwright does not
// transpile node_modules). This is the single source of truth — probes no
// longer co-mount a copy.

const { test: base, expect } = require('@playwright/test')
const { existsSync } = require('node:fs')

const test = base.extend({
  // HAR recording on for this test's context, to a per-test path.
  contextOptions: async ({ contextOptions }, use, testInfo) => {
    await use({
      ...contextOptions,
      recordHar: { path: testInfo.outputPath('network.har'), mode: 'full', content: 'omit' },
    })
  },
  // Close the context to flush the HAR, then attach it.
  context: async ({ context }, use, testInfo) => {
    await use(context)
    await context.close().catch(() => undefined)
    const har = testInfo.outputPath('network.har')
    if (existsSync(har)) {
      await testInfo.attach('network.har', { path: har, contentType: 'application/json' })
    }
  },
})

module.exports = { test, expect }
