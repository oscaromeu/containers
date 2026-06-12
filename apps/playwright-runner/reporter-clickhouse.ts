import type { FullConfig, FullResult, Reporter, TestCase, TestResult, TestStep } from '@playwright/test/reporter'
import { createClient, type ClickHouseClient } from '@clickhouse/client'

// Inserts one `runs` row + N `steps` rows per probe execution into ClickHouse.
// Buffers during the run and flushes a single batched insert per table in
// onEnd (ClickHouse dislikes row-by-row). Activates only when CLICKHOUSE_URL is
// set, so the runner still works without ClickHouse. A telemetry failure must
// never fail the probe — all ClickHouse calls are wrapped and swallowed.
//
// Config via env:
//   CLICKHOUSE_URL       e.g. http://clickhouse-<chi>.<ns>.svc:8123  (required to activate)
//   CLICKHOUSE_USER      default: default
//   CLICKHOUSE_PASSWORD  default: empty
//   CLICKHOUSE_DATABASE  default: e2e
//   PROBE_NAME           logical probe id, e.g. podinfo            (default: unknown)
//   PROBE_ENV            default: dev
//   HOSTNAME             pod name, used as run_id (set by k8s)
//   GIT_SHA / PROBE_{REPORT,VIDEO,TRACE}_URL  optional metadata

type Status = 'pass' | 'flaky' | 'fail'

// eslint-disable-next-line no-control-regex
const stripAnsi = (s: string): string => s.replace(/\x1b\[[0-9;]*m/g, '')
const clip = (s: string | undefined): string => (s ? stripAnsi(s).slice(0, 2000) : '')

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

class ClickHouseReporter implements Reporter {
  private readonly enabled = !!process.env.CLICKHOUSE_URL
  private readonly runId = process.env.HOSTNAME || `run-${Date.now()}`
  private readonly probe = process.env.PROBE_NAME || 'unknown'
  private readonly env = process.env.PROBE_ENV || 'dev'

  private startedAtMs = Date.now()
  private testsTotal = 0
  private firstError = ''
  // Keyed by test id so retries don't double-count.
  private readonly outcomes = new Map<string, string>()
  private readonly steps: StepRow[] = []

  onBegin(_config: FullConfig, suite: { allTests(): TestCase[] }): void {
    if (!this.enabled) return
    this.startedAtMs = Date.now()
    this.testsTotal = suite.allTests().length
  }

  onStepEnd(test: TestCase, _result: TestResult, step: TestStep): void {
    if (!this.enabled || step.category !== 'test.step') return
    this.steps.push({
      run_id: this.runId,
      probe: this.probe,
      test: test.title,
      step: step.title,
      started_at: step.startTime.toISOString(),
      duration_ms: Math.round(step.duration),
      status: step.error ? 'fail' : 'pass',
      error: clip(step.error?.message),
    })
  }

  onTestEnd(test: TestCase, result: TestResult): void {
    if (!this.enabled) return
    const outcome = test.outcome() // expected | unexpected | flaky | skipped
    this.outcomes.set(test.id, outcome)
    if (!this.firstError && outcome === 'unexpected') {
      this.firstError = clip(result.error?.message)
    }
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
      started_at: new Date(this.startedAtMs).toISOString(),
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

    let client: ClickHouseClient | undefined
    try {
      client = createClient({
        url: process.env.CLICKHOUSE_URL,
        username: process.env.CLICKHOUSE_USER || 'default',
        password: process.env.CLICKHOUSE_PASSWORD || '',
        database: process.env.CLICKHOUSE_DATABASE || 'e2e',
      })
      // best_effort parses the ISO-8601 strings (with `T`/`Z`) into DateTime64.
      const settings = { date_time_input_format: 'best_effort' as const }
      await client.insert({ table: 'runs', values: [run], format: 'JSONEachRow', clickhouse_settings: settings })
      if (this.steps.length > 0) {
        await client.insert({ table: 'steps', values: this.steps, format: 'JSONEachRow', clickhouse_settings: settings })
      }
      emit('info', 'clickhouse insert ok', { run_id: this.runId, status, steps: this.steps.length })
    } catch (err) {
      // Telemetry must never fail the probe.
      emit('error', 'clickhouse insert failed', { run_id: this.runId, error: String(err) })
    } finally {
      await client?.close().catch(() => undefined)
    }
  }
}

export default ClickHouseReporter
