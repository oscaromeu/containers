# syntax=docker/dockerfile:1

ARG VERSION
FROM mcr.microsoft.com/playwright:${VERSION}-jammy

WORKDIR /app

ENV NODE_ENV=production \
    CI=true \
    PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
    PW_TEST_DIR=/tests \
    # Tests are mounted at runtime in PW_TEST_DIR (outside /app), so Node's
    # node_modules walk-up never reaches /app/node_modules. NODE_PATH makes
    # @playwright/test and the @probe/playwright helper module resolvable from
    # specs located anywhere.
    NODE_PATH=/app/node_modules

# Install @playwright/test pinned to the SAME version as the base image. The
# version is the single source of truth in docker-bake.hcl (`VERSION`); we strip
# the leading `v` to get the npm version. Keeping these aligned avoids the
# "Executable doesn't exist" drift between the package and the bundled browsers.
ARG VERSION
RUN npm install --no-audit --no-fund \
      "@playwright/test@$(echo "${VERSION}" | sed -e 's/^v//')" \
      "@clickhouse/client@^1" && \
    rm -rf /root/.npm

# S3 client (MinIO mc, single static binary; S3-compatible with Garage). Used by
# the run-and-upload entrypoint to ship the Playwright HTML report — which embeds
# the video + trace on failure — to object storage after each run.
# TARGETARCH so each platform of the multi-arch build gets the matching binary:
# the homelab is arm64, and an amd64 mc fails at runtime with "Exec format error".
ARG TARGETARCH
RUN curl -fsSL "https://dl.min.io/client/mc/release/linux-${TARGETARCH}/mc" -o /usr/local/bin/mc \
    && chmod +x /usr/local/bin/mc

# The runner config + reporters are baked in (a platform concern). Tests are
# NOT baked: they are provided at runtime, mounted into PW_TEST_DIR (e.g. from a
# Kubernetes ConfigMap). Playwright transpiles `.ts` specs on the fly, so no
# build step is needed for mounted tests.
COPY playwright.config.ts reporter-json-line.ts reporter-clickhouse.ts reporter-prometheus.ts tsconfig.json ./
COPY selftest/ ./selftest/

# Baked instrumentation module. Specs do `import { test, expect } from
# '@probe/playwright'` and get HAR + Web Vitals + navigation timing for free.
# Plain CJS in node_modules → loaded directly by Node (not transpiled), resolved
# via NODE_PATH like @playwright/test. No per-probe copy of the fixture.
COPY lib/probe-playwright/ /app/node_modules/@probe/playwright/

# Opt-in artifact-upload entrypoint: runs the probe, then ships the HTML report
# to object storage. Used as the cronjob `command` when S3_* env is set; the
# default CMD (`npx playwright test`) still works for plain runs.
COPY --chmod=0755 lib/run-and-upload.sh /app/run-and-upload.sh

CMD ["npx", "playwright", "test"]
