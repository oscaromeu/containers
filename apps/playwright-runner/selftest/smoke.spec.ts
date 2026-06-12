import { test, expect } from '@playwright/test'

// Network-free smoke test, baked into the image at /app/selftest. It does NOT
// touch the network or launch a browser (no `page` fixture), so it validates
// that the runner can discover, transpile and execute a spec — and that the
// reporters fire — without any external dependency. Used by container_test.go
// via PW_TEST_DIR=/app/selftest.
test('runner self-test: executes a spec', async () => {
  expect(1 + 1).toBe(2)
})
