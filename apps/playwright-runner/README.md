# playwright-runner

Generic Playwright test runner image. The image bakes the **runner**
(`@playwright/test`, the config, and the reporters) but **not the tests** — specs
are provided at runtime, typically mounted from a Kubernetes ConfigMap into
`PW_TEST_DIR`. The same image can run any probe by changing the env and the
mounted spec.

Playwright transpiles `.ts` specs on the fly, so mounted tests need no build
step. Constraint: mounted tests may only import from `@playwright/test` (plus
anything else mounted alongside them) — extra npm dependencies are not present
in the image.

## Configuration (env vars)

| Var | Default | Description |
|-----|---------|-------------|
| `PW_BASE_URL` | `http://localhost` | Base URL the test navigates against |
| `PW_TEST_DIR` | `/tests` | Directory the runner discovers specs in (mount point) |
| `PW_RETRIES` | `0` | Retries per test |
| `PW_TIMEOUT_MS` | `90000` | Per-test timeout |
| `PW_GLOBAL_TIMEOUT_MS` | `600000` | Whole-run timeout |
| `PW_TRACE` | `retain-on-failure` | `on` \| `off` \| `retain-on-failure` |
| `PW_SCREENSHOT` | `only-on-failure` | `on` \| `off` \| `only-on-failure` |
| `PW_VIDEO` | `off` | `on` \| `off` \| `retain-on-failure` |
| `PW_ACTION_TIMEOUT_MS` | `15000` | Per-action timeout |
| `PW_NAV_TIMEOUT_MS` | `30000` | Navigation timeout |
| `PW_REPORT_DIR` | `playwright-report` | HTML report output folder |

## Reporters

- `reporter-json-line.ts` — one structured JSON line per lifecycle event to
  stdout (run started/finished, test passed/failed with `duration_ms`). Meant to
  be scraped into logs.
- `html` — Playwright HTML report under `PW_REPORT_DIR`.

- `reporter-clickhouse.ts` — inserts one `runs` row + N `steps` rows per
  execution into ClickHouse (batched in `onEnd`). Activates **only** when
  `CLICKHOUSE_URL` is set; a telemetry failure never fails the probe.

### ClickHouse reporter env

| Var | Default | Description |
|-----|---------|-------------|
| `CLICKHOUSE_URL` | — | HTTP endpoint, e.g. `http://clickhouse-<chi>.<ns>.svc:8123`. **Set to activate.** |
| `CLICKHOUSE_USER` | `default` | ClickHouse user |
| `CLICKHOUSE_PASSWORD` | _(empty)_ | ClickHouse password (from a Secret) |
| `CLICKHOUSE_DATABASE` | `e2e` | Target database |
| `PROBE_NAME` | `unknown` | Logical probe id (becomes the `probe` column / row in Grafana) |
| `PROBE_ENV` | `dev` | Environment label |
| `GIT_SHA`, `PROBE_REPORT_URL`, `PROBE_VIDEO_URL`, `PROBE_TRACE_URL` | _(empty)_ | Optional metadata stored on the `runs` row |

`run_id` is the pod name (`$HOSTNAME`). Schema: see the `e2e.runs` / `e2e.steps`
DDL in the project doc.

## Run a test from a ConfigMap (sketch)

```yaml
# The spec comes from a ConfigMap mounted at /tests; the image stays generic.
containers:
  - name: playwright
    image: ghcr.io/oscaromeu/playwright-runner:rolling
    env:
      - { name: PW_BASE_URL, value: https://podinfo.example.com }
    volumeMounts:
      - { name: tests, mountPath: /tests }
volumes:
  - name: tests
    configMap:
      name: podinfo-test
```

See [`examples/podinfo.spec.ts`](examples/podinfo.spec.ts) for a starting spec.

## Local usage

```bash
# Build
docker buildx bake image-local

# Run the baked self-test (network-free)
docker run --rm -e PW_TEST_DIR=/app/selftest playwright-runner:v1.57.0

# Run a spec from the host against a target
docker run --rm \
  -e PW_BASE_URL=https://example.com \
  -v "$PWD/examples:/tests:ro" \
  playwright-runner:v1.57.0
```

## Image tests

`container_test.go` checks that Playwright is installed and that the runner can
discover, transpile and execute the baked self-test spec end to end.
