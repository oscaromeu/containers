import type { FullConfig, FullResult, Reporter, TestCase, TestResult } from '@playwright/test/reporter'

// Prometheus Pushgateway reporter — pushes per-run gauges on run end. Activates
// only when PUSHGATEWAY_URL is set, so the runner still works without it, and is
// fully independent of reporter-clickhouse.ts. A telemetry failure never fails
// the probe (same contract as the ClickHouse reporter).
//
// This is the "naive metric" baseline that sits NEXT TO the ClickHouse event
// store, on the SAME run, for a like-for-like comparison. It is deliberately low
// cardinality: the grouping key is job=<probe>/env=<env> and there is NO run_id
// label — adding one would create a Prometheus series per run (the cardinality
// trap). The consequences are visible by design:
//   * probe_success is last-value → a dead cron stays "green" forever.
//   * probe_runs_total pushed as 1 → a stateless job can't accumulate a counter.
//   * the *why* (error/step/trace/video) cannot be represented here at all.

type Status = 'pass' | 'flaky' | 'fail'

const emit = (level: string, message: string, extra: Record<string, unknown> = {}): void => {
  console.log(JSON.stringify({ level, message, ts: new Date().toISOString(), ...extra }))
}

class PrometheusReporter implements Reporter {
  private readonly base = process.env.PUSHGATEWAY_URL
  private readonly enabled = !!this.base
  private readonly probe = process.env.PROBE_NAME || 'unknown'
  private readonly env = process.env.PROBE_ENV || 'dev'

  private testsTotal = 0
  private readonly outcomes = new Map<string, string>()

  onBegin(_config: FullConfig, suite: { allTests(): TestCase[] }): void {
    if (!this.enabled) return
    this.testsTotal = suite.allTests().length
  }

  onTestEnd(test: TestCase, _result: TestResult): void {
    if (!this.enabled) return
    this.outcomes.set(test.id, test.outcome())
  }

  async onEnd(result: FullResult): Promise<void> {
    if (!this.enabled || !this.base) return

    const outcomes = [...this.outcomes.values()]
    const testsFailed = outcomes.filter((o) => o === 'unexpected').length
    const anyFlaky = outcomes.some((o) => o === 'flaky')
    const status: Status =
      testsFailed > 0 || result.status !== 'passed' ? 'fail' : anyFlaky ? 'flaky' : 'pass'
    const statusCode = status === 'pass' ? 2 : status === 'flaky' ? 1 : 0

    // job + env come from the Pushgateway grouping key (URL path), so the body
    // carries bare metrics. A trailing newline is required by the text format.
    const body = [
      '# TYPE probe_success gauge',
      `probe_success ${status === 'fail' ? 0 : 1}`,
      '# TYPE probe_status gauge',
      `probe_status ${statusCode}`,
      '# TYPE probe_duration_ms gauge',
      `probe_duration_ms ${Math.round(result.duration)}`,
      '# TYPE probe_tests_total gauge',
      `probe_tests_total ${this.testsTotal}`,
      '# TYPE probe_tests_failed gauge',
      `probe_tests_failed ${testsFailed}`,
      // A counter pushed as 1 every run: it CANNOT accumulate from a stateless
      // job (Pushgateway keeps the last value). Left in on purpose as a live demo.
      '# TYPE probe_runs_total counter',
      'probe_runs_total 1',
      '',
    ].join('\n')

    const url =
      `${this.base.replace(/\/$/, '')}` +
      `/metrics/job/${encodeURIComponent(this.probe)}/env/${encodeURIComponent(this.env)}`

    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), 5000)
    try {
      // PUT replaces the whole group → last-value semantics per (probe, env).
      const res = await fetch(url, {
        method: 'PUT',
        headers: { 'Content-Type': 'text/plain' },
        body,
        signal: controller.signal,
      })
      if (res.ok) emit('info', 'pushgateway push ok', { probe: this.probe, status })
      else emit('error', 'pushgateway push non-2xx', { probe: this.probe, code: res.status })
    } catch (err) {
      emit('error', 'pushgateway push failed', { probe: this.probe, error: String(err) })
    } finally {
      clearTimeout(timer)
    }
  }
}

export default PrometheusReporter
