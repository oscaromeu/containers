#!/usr/bin/env bash
set -euo pipefail

# Required: anything that must be supplied by the operator (no safe default).
: "${ICECAST_SOURCE_PASSWORD:?ICECAST_SOURCE_PASSWORD is required}"
: "${ICECAST_RELAY_PASSWORD:?ICECAST_RELAY_PASSWORD is required}"
: "${ICECAST_ADMIN_PASSWORD:?ICECAST_ADMIN_PASSWORD is required}"

# Optional with sensible defaults.
: "${ICECAST_HOSTNAME:=localhost}"
: "${ICECAST_LOCATION:=Earth}"
: "${ICECAST_ADMIN_EMAIL:=admin@localhost}"
: "${ICECAST_ADMIN_USERNAME:=admin}"
: "${ICECAST_MAX_CLIENTS:=100}"
: "${ICECAST_MAX_SOURCES:=20}"
: "${ICECAST_LOG_LEVEL:=3}"     # 1=err, 2=warn, 3=info, 4=debug
: "${ICECAST_BURST_SIZE:=65535}"

export ICECAST_HOSTNAME ICECAST_LOCATION ICECAST_ADMIN_EMAIL \
       ICECAST_SOURCE_PASSWORD ICECAST_RELAY_PASSWORD \
       ICECAST_ADMIN_USERNAME ICECAST_ADMIN_PASSWORD \
       ICECAST_MAX_CLIENTS ICECAST_MAX_SOURCES ICECAST_LOG_LEVEL \
       ICECAST_BURST_SIZE

# Render the config from the template.
envsubst < /etc/icecast.xml.template > /tmp/icecast.xml

# Pipe Icecast logs to the container's stdout/stderr so `docker logs` and
# Kubernetes log collectors pick them up.
ln -sf /dev/stdout /var/log/icecast/access.log
ln -sf /dev/stderr /var/log/icecast/error.log

exec icecast -c /tmp/icecast.xml
