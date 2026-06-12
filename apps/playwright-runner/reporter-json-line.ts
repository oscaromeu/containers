import type { FullResult, Reporter, TestCase, TestResult } from '@playwright/test/reporter'

// eslint-disable-next-line no-control-regex
const stripAnsi = (s: string): string => s.replace(/\x1b\[[0-9;]*m/g, '')

const emit = (level: string, message: string, extra: Record<string, unknown> = {}): void => {
  const cleaned = Object.fromEntries(
    Object.entries(extra).filter(([, v]) => v !== undefined && v !== null),
  )
  console.log(JSON.stringify({ level, message, ts: new Date().toISOString(), ...cleaned }))
}

class JsonLineReporter implements Reporter {
  onBegin(_config: unknown, suite: { allTests(): TestCase[] }): void {
    emit('info', 'run started', { tests: suite.allTests().length })
  }

  onTestBegin(test: TestCase): void {
    emit('info', 'test started', { test: test.title })
  }

  // Forward test stdout/stderr (e.g. the `log()` helper inside steps) to the
  // terminal. Without these, Playwright captures test output and attaches it
  // to the result instead of printing it live.
  onStdOut(chunk: string | Buffer): void {
    process.stdout.write(typeof chunk === 'string' ? chunk : chunk.toString('utf-8'))
  }

  onStdErr(chunk: string | Buffer): void {
    process.stderr.write(typeof chunk === 'string' ? chunk : chunk.toString('utf-8'))
  }

  onTestEnd(test: TestCase, result: TestResult): void {
    const passed = result.status === 'passed'
    emit(
      passed ? 'info' : 'error',
      passed ? 'test passed' : 'test failed',
      {
        test: test.title,
        status: passed ? 'ok' : 'failed',
        duration_ms: result.duration,
        error: result.error ? stripAnsi(result.error.message ?? '') : undefined,
      },
    )
  }

  onEnd(result: FullResult): void {
    emit('info', 'run finished', {
      status: result.status === 'passed' ? 'ok' : 'failed',
      duration_ms: result.duration,
    })
  }
}

export default JsonLineReporter
