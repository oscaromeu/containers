import type { FullConfig, FullResult, Reporter, TestCase, TestResult, TestStep } from '@playwright/test/reporter'
import { createClient, type ClickHouseClient } from '@clickhouse/client'

// Inserts per probe execution into ClickHouse:
//   e2e.runs  — one row (status, duration, counts, artifact URLs)
//   e2e.steps — one row per test.step (duration + pass/fail)
// Buffers during the run and flushes one batched insert per table in onEnd.
// Activates only when CLICKHOUSE_URL is set; a telemetry failure never fails the probe.
//
// Per-request network timing and Web Vitals are deliberately NOT stored: they
// are evidence for one execution, not a series worth querying across runs. They
// live in the HTML report (HAR included) that run-and-upload.sh ships to object
// storage.

type Status = 'pass' | 'flaky' | 'fail'

// eslint-disable-next-line no-control-regex
const stripAnsi = (s: string): string => s.replace(/\x1b\[[0-9;]*m/g, '')
const clip = (s: string | undefined, n = 2000): string => (s ? stripAnsi(s).slice(0, n) : '')
const iso = (ms: number): string => new Date(ms).toISOString()

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
      started_at: iso(step.startTime.getTime()),
      duration_ms: Math.round(step.duration),
      status: step.error ? 'fail' : 'pass',
      error: clip(step.error?.message),
    })
  }

  onTestEnd(test: TestCase, result: TestResult): void {
    if (!this.enabled) return
    const outcome = test.outcome()
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

    // Per-probe thresholds from env (sane defaults so a new probe works without
    // any config). Upserted every run → e2e.probe_config (ReplacingMergeTree).
    const numEnv = (v: string | undefined, d: number): number => {
      const n = Number(v)
      return Number.isFinite(n) && n > 0 ? n : d
    }
    const probeConfig = {
      probe: this.probe,
      slo_target: numEnv(process.env.PROBE_SLO_TARGET, 99),
      high_ms: numEnv(process.env.PROBE_HIGH_MS, 2000),
      critical_ms: numEnv(process.env.PROBE_CRITICAL_MS, 4000),
      fatal_ms: numEnv(process.env.PROBE_FATAL_MS, 8000),
      updated_at: iso(this.startedAtMs),
    }

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
      // Best-effort: a missing/old probe_config table must never drop the run's telemetry.
      try {
        await client.insert({ table: 'probe_config', values: [probeConfig], format: 'JSONEachRow', clickhouse_settings: settings })
      } catch (e) {
        console.error('[reporter-clickhouse] probe_config upsert failed (continuing):', e)
      }
      if (this.steps.length > 0) {
        await client.insert({ table: 'steps', values: this.steps, format: 'JSONEachRow', clickhouse_settings: settings })
      }
      emit('info', 'clickhouse insert ok', { run_id: this.runId, status, steps: this.steps.length })
    } catch (err) {
      emit('error', 'clickhouse insert failed', { run_id: this.runId, error: String(err) })
    } finally {
      await client?.close().catch(() => undefined)
    }
  }
}

export default ClickHouseReporter
