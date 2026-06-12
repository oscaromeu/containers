import type { FullConfig, FullResult, Reporter, TestCase, TestResult, TestStep } from '@playwright/test/reporter'
import { createClient, type ClickHouseClient } from '@clickhouse/client'
import { readFileSync } from 'node:fs'

// Inserts per probe execution into ClickHouse:
//   e2e.runs       — one row (status, duration, counts, artifact URLs)
//   e2e.steps      — one row per test.step (duration + pass/fail)
//   e2e.net_timing — one row per request, parsed from the `network.har` attachment
//                    (httpstat phases + status/method/size/protocol + failed),
//                    correlated to the active step by wall-clock time window.
// Buffers during the run and flushes one batched insert per table in onEnd.
// Activates only when CLICKHOUSE_URL is set; a telemetry failure never fails the probe.
//
// net_timing requires the test to record a HAR — provided by the co-mounted
// net-timing.ts fixture (import { test } from './net-timing').

type Status = 'pass' | 'flaky' | 'fail'

// eslint-disable-next-line no-control-regex
const stripAnsi = (s: string): string => s.replace(/\x1b\[[0-9;]*m/g, '')
const clip = (s: string | undefined, n = 2000): string => (s ? stripAnsi(s).slice(0, n) : '')
const iso = (ms: number): string => new Date(ms).toISOString()
const nonNeg = (v: unknown): number => (typeof v === 'number' && v >= 0 ? v : 0)

const emit = (level: string, message: string, extra: Record<string, unknown> = {}): void => {
  console.log(JSON.stringify({ level, message, ts: new Date().toISOString(), ...extra }))
}

interface StepRow {
  run_id: string
  probe: string
  test: string
  step: string
  started_at: string
  duration_ms: number
  status: Status
  error: string
}

interface StepWindow { test: string; step: string; startMs: number; endMs: number }

// One captured request, derived from a HAR entry.
interface NetRecord {
  test: string
  ts: number
  url: string
  type: string
  method: string
  status_code: number
  protocol: string
  dns_ms: number
  tcp_ms: number
  tls_ms: number
  server_ms: number
  transfer_ms: number
  total_ms: number
  response_bytes: number
  failed: number
  error_text: string
}

const CAPTURED = new Set(['document', 'xhr', 'fetch'])

// Map a HAR 1.2 entry → NetRecord. HAR `connect` includes `ssl`, so tcp = connect - ssl.
const fromHarEntry = (test: string, entry: Record<string, any>): NetRecord | null => {
  const req = entry.request ?? {}
  const resp = entry.response ?? {}
  const t = entry.timings ?? {}
  const type = String(entry._resourceType ?? '').toLowerCase()
  if (type && !CAPTURED.has(type)) return null

  const tls = nonNeg(t.ssl)
  const connect = nonNeg(t.connect)
  const status = nonNeg(resp.status)
  const failureText = resp._failureText ?? ''
  return {
    test,
    ts: Date.parse(entry.startedDateTime) || Date.now(),
    url: String(req.url ?? ''),
    type,
    method: String(req.method ?? ''),
    status_code: status,
    protocol: String(resp.httpVersion ?? req.httpVersion ?? '').toLowerCase(),
    dns_ms: Math.round(nonNeg(t.dns)),
    tcp_ms: Math.round(Math.max(0, connect - tls)),
    tls_ms: Math.round(tls),
    server_ms: Math.round(nonNeg(t.wait)),
    transfer_ms: Math.round(nonNeg(t.receive)),
    total_ms: Math.round(nonNeg(entry.time)),
    response_bytes: Math.round(nonNeg(resp._transferSize) || nonNeg(resp.bodySize) || nonNeg(resp.content?.size)),
    failed: status === 0 || failureText ? 1 : 0,
    error_text: clip(String(failureText), 500),
  }
}

class ClickHouseReporter implements Reporter {
  private readonly enabled = !!process.env.CLICKHOUSE_URL
  private readonly runId = process.env.HOSTNAME || `run-${Date.now()}`
  private readonly probe = process.env.PROBE_NAME || 'unknown'
  private readonly env = process.env.PROBE_ENV || 'dev'

  private startedAtMs = Date.now()
  private testsTotal = 0
  private firstError = ''
  private readonly outcomes = new Map<string, string>()
  private readonly steps: StepRow[] = []
  private readonly stepWindows: StepWindow[] = []
  private readonly net: NetRecord[] = []
  private readonly vitals: Array<Record<string, number> & { test: string; started_at: number }> = []

  onBegin(_config: FullConfig, suite: { allTests(): TestCase[] }): void {
    if (!this.enabled) return
    this.startedAtMs = Date.now()
    this.testsTotal = suite.allTests().length
  }

  onStepEnd(test: TestCase, _result: TestResult, step: TestStep): void {
    if (!this.enabled || step.category !== 'test.step') return
    const startMs = step.startTime.getTime()
    const durationMs = Math.round(step.duration)
    this.steps.push({
      run_id: this.runId,
      probe: this.probe,
      test: test.title,
      step: step.title,
      started_at: iso(startMs),
      duration_ms: durationMs,
      status: step.error ? 'fail' : 'pass',
      error: clip(step.error?.message),
    })
    this.stepWindows.push({ test: test.title, step: step.title, startMs, endMs: startMs + durationMs })
  }

  onTestEnd(test: TestCase, result: TestResult): void {
    if (!this.enabled) return
    const outcome = test.outcome()
    this.outcomes.set(test.id, outcome)
    if (!this.firstError && outcome === 'unexpected') {
      this.firstError = clip(result.error?.message)
    }
    // Parse the HAR recorded by the net-timing.ts fixture.
    const har = result.attachments.find((a) => a.name === 'network.har')
    const harPath = har?.path
    if (harPath) {
      try {
        const log = JSON.parse(readFileSync(harPath, 'utf-8')).log
        for (const entry of log.entries ?? []) {
          const rec = fromHarEntry(test.title, entry)
          if (rec) this.net.push(rec)
        }
      } catch {
        // missing/malformed HAR — skip net timing for this test
      }
    }
    // Parse the Web Vitals recorded by the fixture.
    const wv = result.attachments.find((a) => a.name === 'web-vitals.json')
    if (wv?.body) {
      try {
        const v = JSON.parse(wv.body.toString()) as Record<string, number>
        this.vitals.push({ test: test.title, started_at: result.startTime.getTime(), ...v })
      } catch {
        // missing/malformed web-vitals — skip
      }
    }
  }

  // Innermost step (shortest window) of `test` active at wall-clock `ts`.
  private stepFor(test: string, ts: number): string {
    let best = ''
    let bestSpan = Infinity
    for (const w of this.stepWindows) {
      if (w.test !== test || ts < w.startMs || ts > w.endMs) continue
      const span = w.endMs - w.startMs
      if (span < bestSpan) { bestSpan = span; best = w.step }
    }
    return best
  }

  async onEnd(result: FullResult): Promise<void> {
    if (!this.enabled) return

    const outcomes = [...this.outcomes.values()]
    const testsFailed = outcomes.filter((o) => o === 'unexpected').length
    const anyFlaky = outcomes.some((o) => o === 'flaky')
    const status: Status =
      testsFailed > 0 || result.status !== 'passed' ? 'fail' : anyFlaky ? 'flaky' : 'pass'

    const run = {
      run_id: this.runId,
      probe: this.probe,
      env: this.env,
      started_at: iso(this.startedAtMs),
      duration_ms: Math.round(result.duration),
      status,
      tests_total: this.testsTotal,
      tests_failed: testsFailed,
      report_url: process.env.PROBE_REPORT_URL || '',
      video_url: process.env.PROBE_VIDEO_URL || '',
      trace_url: process.env.PROBE_TRACE_URL || '',
      error: this.firstError,
      git_sha: process.env.GIT_SHA || '',
    }

    const netRows = this.net.map((r) => ({
      run_id: this.runId,
      probe: this.probe,
      test: r.test,
      step: this.stepFor(r.test, r.ts),
      url: r.url,
      type: r.type,
      method: r.method,
      status_code: r.status_code,
      protocol: r.protocol,
      dns_ms: r.dns_ms,
      tcp_ms: r.tcp_ms,
      tls_ms: r.tls_ms,
      server_ms: r.server_ms,
      transfer_ms: r.transfer_ms,
      total_ms: r.total_ms,
      response_bytes: r.response_bytes,
      failed: r.failed,
      error_text: r.error_text,
      ts: iso(r.ts),
    }))

    const vitalsRows = this.vitals.map((v) => ({
      run_id: this.runId,
      probe: this.probe,
      test: v.test,
      started_at: iso(v.started_at),
      lcp_ms: v.lcp_ms ?? 0,
      fcp_ms: v.fcp_ms ?? 0,
      ttfb_ms: v.ttfb_ms ?? 0,
      inp_ms: v.inp_ms ?? 0,
      cls: v.cls ?? 0,
      dom_content_loaded_ms: v.dom_content_loaded_ms ?? 0,
      load_ms: v.load_ms ?? 0,
    }))

    let client: ClickHouseClient | undefined
    try {
      client = createClient({
        url: process.env.CLICKHOUSE_URL,
        username: process.env.CLICKHOUSE_USER || 'default',
        password: process.env.CLICKHOUSE_PASSWORD || '',
        database: process.env.CLICKHOUSE_DATABASE || 'e2e',
      })
      const settings = { date_time_input_format: 'best_effort' as const }
      await client.insert({ table: 'runs', values: [run], format: 'JSONEachRow', clickhouse_settings: settings })
      if (this.steps.length > 0) {
        await client.insert({ table: 'steps', values: this.steps, format: 'JSONEachRow', clickhouse_settings: settings })
      }
      if (netRows.length > 0) {
        await client.insert({ table: 'net_timing', values: netRows, format: 'JSONEachRow', clickhouse_settings: settings })
      }
      if (vitalsRows.length > 0) {
        await client.insert({ table: 'web_vitals', values: vitalsRows, format: 'JSONEachRow', clickhouse_settings: settings })
      }
      emit('info', 'clickhouse insert ok', {
        run_id: this.runId, status, steps: this.steps.length, net: netRows.length, vitals: vitalsRows.length,
      })
    } catch (err) {
      emit('error', 'clickhouse insert failed', { run_id: this.runId, error: String(err) })
    } finally {
      await client?.close().catch(() => undefined)
    }
  }
}

export default ClickHouseReporter
