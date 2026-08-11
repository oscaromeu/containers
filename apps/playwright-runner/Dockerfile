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
    NODE_PATH=/app/node_modules \
    # mc rewrites its config on every invocation; the runner is non-root and
    # $HOME is not guaranteed writable, /tmp always is.
    MC_CONFIG_DIR=/tmp/.mc

# Dependencies come from the lockfile (`npm ci`), so rebuilding an old commit
# resolves the exact same tree.
#
# @playwright/test must match the browsers bundled in the base image or runs die
# with "Executable doesn't exist". The image tag (`VERSION` in docker-bake.hcl)
# and the pin in package.json are two places to say the same thing, so the build
# compares them and fails on drift instead of shipping a broken image.
ARG VERSION
COPY package.json package-lock.json ./
# @probe/playwright is a `file:` dep — it must be present before npm ci links it.
COPY lib/probe-playwright/ ./lib/probe-playwright/
RUN pinned="$(node -p "require('./package.json').dependencies['@playwright/test']")" \
    && if [ "${pinned}" != "${VERSION#v}" ]; then \
         echo "VERSION=${VERSION} does not match package.json @playwright/test=${pinned}" >&2; \
         exit 1; \
       fi \
    && npm ci --omit=dev --no-audit --no-fund \
    && npm cache clean --force

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
#
# Specs do `import { test, expect } from '@probe/playwright'` and get HAR + Web
# Vitals + navigation timing for free: npm ci links it into /app/node_modules,
# resolved via NODE_PATH like @playwright/test.
COPY playwright.config.ts reporter-json-line.ts reporter-clickhouse.ts reporter-prometheus.ts tsconfig.json ./
COPY selftest/ ./selftest/

# Opt-in artifact-upload entrypoint: runs the probe, then ships the HTML report
# to object storage. Used as the cronjob `command` when S3_* env is set; the
# default CMD (`npx playwright test`) still works for plain runs.
COPY --chmod=0755 lib/run-and-upload.sh /app/run-and-upload.sh

# Non-root pwuser (uid 1000, from the base image) writes the report and the test
# output dirs under /app at runtime.
RUN chown -R pwuser:pwuser /app
USER pwuser

CMD ["npx", "playwright", "test"]
