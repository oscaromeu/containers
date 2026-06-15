import type {
  TestType,
  PlaywrightTestArgs,
  PlaywrightTestOptions,
  PlaywrightWorkerArgs,
  PlaywrightWorkerOptions,
} from '@playwright/test'

// Same surface as @playwright/test — the instrumentation (HAR + Web Vitals +
// navigation timing) is added via fixtures and is transparent to the spec.
export declare const test: TestType<
  PlaywrightTestArgs & PlaywrightTestOptions,
  PlaywrightWorkerArgs & PlaywrightWorkerOptions
>
export { expect } from '@playwright/test'
