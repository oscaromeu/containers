#!/bin/sh
# Run the Playwright probe, then (only if S3_ENDPOINT is set) upload the HTML
# report — which embeds the video + trace on failure — to object storage under
#   <bucket>/<probe>/<timestamp>/report
# The timestamp is computed ONCE here and reused for both the stored
# PROBE_REPORT_URL (read by reporter-clickhouse) and the upload path, so the
# link in Grafana always matches the uploaded files. Opt-in: with no S3_ENDPOINT
# the image behaves exactly like `npx playwright test`.
set -u

TS=$(date -u +%Y%m%dT%H%M%SZ)
DEST="${PROBE_NAME:-unknown}/${TS}"

# Tell the ClickHouse reporter where the artifacts will live (deterministic).
if [ -n "${PROBE_ARTIFACT_BASE:-}" ]; then
  export PROBE_REPORT_URL="${PROBE_ARTIFACT_BASE}/${DEST}/report/index.html"
fi

npx playwright test
ec=$?

REPORT_DIR="${PW_REPORT_DIR:-playwright-report}"
if [ -n "${S3_ENDPOINT:-}" ] && [ -d "$REPORT_DIR" ]; then
  export MC_HOST_artifacts="http://${S3_ACCESS_KEY}:${S3_SECRET_KEY}@${S3_ENDPOINT}"
  if mc cp --quiet --recursive "${REPORT_DIR}/" "artifacts/${S3_BUCKET}/${DEST}/report/"; then
    echo "{\"level\":\"info\",\"message\":\"artifacts uploaded\",\"dest\":\"${S3_BUCKET}/${DEST}/report\"}"
  else
    echo "{\"level\":\"error\",\"message\":\"artifact upload failed\",\"dest\":\"${S3_BUCKET}/${DEST}/report\"}"
  fi
fi

exit $ec
