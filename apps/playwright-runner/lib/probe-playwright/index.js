// @probe/playwright — baked test instrumentation fixture for the runner image.
//
// Specs import { test, expect } from '@probe/playwright' (instead of
// '@playwright/test') and transparently get, per test:
//   - HAR (network.har): per-request timings → e2e.net_timing
//   - Web Vitals + PerformanceNavigationTiming (web-vitals.json) → e2e.web_vitals
//
// Resolved from /app/node_modules via NODE_PATH, exactly like @playwright/test.
// Shipped as plain CommonJS so Node loads it directly (Playwright does not
// transpile node_modules). This is the single source of truth — probes no
// longer co-mount a copy.

const { test: base, expect } = require('@playwright/test')
const { existsSync } = require('node:fs')

// Runs in the page BEFORE any script: buffered Web Vitals observers (LCP/CLS/FCP).
// Self-contained — serialized and executed in the browser, no outer scope.
const CWV_INIT = () => {
  window.__cwv = { lcp: 0, cls: 0, fcp: 0 }
  try {
    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) window.__cwv.lcp = e.renderTime || e.loadTime || e.startTime
    }).observe({ type: 'largest-contentful-paint', buffered: true })

    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) if (!e.hadRecentInput) window.__cwv.cls += e.value
    }).observe({ type: 'layout-shift', buffered: true })

    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) if (e.name === 'first-contentful-paint') window.__cwv.fcp = e.startTime
    }).observe({ type: 'paint', buffered: true })
  } catch {
    // PerformanceObserver type unsupported on this browser — leave defaults.
  }
}

// Read vitals + the full PerformanceNavigationTiming breakdown at test end.
// Phase durations follow the MDN formulas. DNS/TCP/TLS are only non-zero on a
// COLD navigation (fresh context); they read 0 when the browser reuses a warm
// connection. https://developer.mozilla.org/en-US/docs/Web/API/PerformanceNavigationTiming
const CWV_READ = () => {
  const cwv = window.__cwv || { lcp: 0, cls: 0, fcp: 0 }
  const nav = performance.getEntriesByType('navigation')[0]
  const round = (n) => Math.max(0, Math.round(n || 0))
  const diff = (a, b) => round((nav[b] || 0) - (nav[a] || 0))

  const out = {
    lcp_ms: round(cwv.lcp),
    cls: Number((cwv.cls || 0).toFixed(4)),
    fcp_ms: round(cwv.fcp),
    ttfb_ms: nav ? round(nav.responseStart) : 0,
    dom_content_loaded_ms: nav ? round(nav.domContentLoadedEventEnd) : 0,
    load_ms: nav ? round(nav.loadEventEnd) : 0,
  }
  if (nav) {
    // secureConnectionStart === 0 means no TLS handshake (http or reused conn).
    const tcpEnd = nav.secureConnectionStart > 0 ? nav.secureConnectionStart : nav.connectEnd
    Object.assign(out, {
      redirect_ms: diff('redirectStart', 'redirectEnd'),
      dns_ms: diff('domainLookupStart', 'domainLookupEnd'),
      tcp_ms: round(tcpEnd - nav.connectStart),
      tls_ms: nav.secureConnectionStart > 0 ? round(nav.connectEnd - nav.secureConnectionStart) : 0,
      request_ms: diff('requestStart', 'responseStart'),
      response_ms: diff('responseStart', 'responseEnd'),
      dom_processing_ms: diff('responseEnd', 'domComplete'),
      dom_interactive_ms: round(nav.domInteractive),
      transfer_bytes: round(nav.transferSize),
      encoded_body_bytes: round(nav.encodedBodySize),
      decoded_body_bytes: round(nav.decodedBodySize),
      redirect_count: round(nav.redirectCount),
      response_status: round(nav.responseStatus),
      nav_type: String(nav.type || ''),
    })
  }
  return out
}

const test = base.extend({
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

module.exports = { test, expect }
