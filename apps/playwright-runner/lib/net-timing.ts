// Test instrumentation fixture: per-test HAR (network timing) + Core Web Vitals.
//
// Usage: import { test, expect } from './net-timing'  (instead of '@playwright/test').
// Co-mount this file next to the spec in the ConfigMap. Captures:
//   - HAR (`network.har`): per-request timings + status/method/size/protocol +
//     failed requests. The ClickHouse reporter parses it → e2e.net_timing.
//   - Web Vitals (`web-vitals.json`): LCP / CLS / FCP / TTFB + DCL/load, read via
//     PerformanceObserver. The reporter → e2e.web_vitals.
// (INP needs user interactions + the web-vitals lib; left out for load probes.)

import { test as base, expect } from '@playwright/test'
import { existsSync } from 'node:fs'

// Runs in the page BEFORE any script: set up Web Vitals observers (buffered so
// they catch entries from the very start). Self-contained — no outer scope.
const CWV_INIT = (): void => {
  const w = window as unknown as { __cwv?: { lcp: number; cls: number; fcp: number } }
  w.__cwv = { lcp: 0, cls: 0, fcp: 0 }
  try {
    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) {
        const lcp = e as unknown as { renderTime?: number; loadTime?: number; startTime: number }
        w.__cwv!.lcp = lcp.renderTime || lcp.loadTime || lcp.startTime
      }
    }).observe({ type: 'largest-contentful-paint', buffered: true })

    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) {
        const ls = e as unknown as { value: number; hadRecentInput: boolean }
        if (!ls.hadRecentInput) w.__cwv!.cls += ls.value
      }
    }).observe({ type: 'layout-shift', buffered: true })

    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) if (e.name === 'first-contentful-paint') w.__cwv!.fcp = e.startTime
    }).observe({ type: 'paint', buffered: true })
  } catch {
    // PerformanceObserver type unsupported on this browser — leave defaults.
  }
}

// Read the collected vitals + navigation timing at the end of the test.
const CWV_READ = () => {
  const cwv = (window as unknown as { __cwv?: { lcp: number; cls: number; fcp: number } }).__cwv || { lcp: 0, cls: 0, fcp: 0 }
  const nav = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined
  return {
    lcp_ms: Math.round(cwv.lcp || 0),
    cls: Number((cwv.cls || 0).toFixed(4)),
    fcp_ms: Math.round(cwv.fcp || 0),
    ttfb_ms: Math.round(nav ? nav.responseStart : 0),
    dom_content_loaded_ms: Math.round(nav ? nav.domContentLoadedEventEnd : 0),
    load_ms: Math.round(nav ? nav.loadEventEnd : 0),
  }
}

export const test = base.extend({
  // HAR recording on for this test's context, to a per-test path.
  contextOptions: async ({ contextOptions }, use, testInfo) => {
    await use({
      ...contextOptions,
      recordHar: { path: testInfo.outputPath('network.har'), mode: 'full', content: 'omit' },
    })
  },
  // Inject the Web Vitals collector into every page; read + attach at the end.
  page: async ({ page }, use, testInfo) => {
    await page.addInitScript(CWV_INIT)
    await use(page)
    try {
      const vitals = await page.evaluate(CWV_READ)
      await testInfo.attach('web-vitals.json', { body: JSON.stringify(vitals), contentType: 'application/json' })
    } catch {
      // page already closed, or a non-page (request-only) test — skip vitals.
    }
  },
  // Close the context to flush the HAR, then attach it (reporter parses it).
  context: async ({ context }, use, testInfo) => {
    await use(context)
    await context.close().catch(() => undefined)
    const har = testInfo.outputPath('network.har')
    if (existsSync(har)) {
      await testInfo.attach('network.har', { path: har, contentType: 'application/json' })
    }
  },
})

export { expect }
