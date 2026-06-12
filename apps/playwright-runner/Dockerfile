# syntax=docker/dockerfile:1

ARG VERSION
FROM mcr.microsoft.com/playwright:${VERSION}-jammy

WORKDIR /app

ENV NODE_ENV=production \
    CI=true \
    PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
    PW_TEST_DIR=/tests

# Install @playwright/test pinned to the SAME version as the base image. The
# version is the single source of truth in docker-bake.hcl (`VERSION`); we strip
# the leading `v` to get the npm version. Keeping these aligned avoids the
# "Executable doesn't exist" drift between the package and the bundled browsers.
ARG VERSION
RUN npm install --no-audit --no-fund "@playwright/test@$(echo "${VERSION}" | sed -e 's/^v//')" && \
    rm -rf /root/.npm

# The runner config + reporters are baked in (a platform concern). Tests are
# NOT baked: they are provided at runtime, mounted into PW_TEST_DIR (e.g. from a
# Kubernetes ConfigMap). Playwright transpiles `.ts` specs on the fly, so no
# build step is needed for mounted tests.
COPY playwright.config.ts reporter-json-line.ts tsconfig.json ./
COPY selftest/ ./selftest/

CMD ["npx", "playwright", "test"]
