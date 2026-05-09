#!/usr/bin/env bash
set -euo pipefail

: "${CHANNEL:?CHANNEL is required (Twitch channel login)}"
: "${ICECAST_URL:?ICECAST_URL is required (e.g. http://icecast:8000)}"
: "${ICECAST_MOUNT:?ICECAST_MOUNT is required (e.g. /channel.mp3)}"
: "${ICECAST_PASS:?ICECAST_PASS is required}"
: "${ICECAST_USER:=source}"
: "${QUALITY:=audio_only}"
: "${BITRATE:=192k}"
: "${ENCODE_MODE:=mp3}"      # mp3 = libmp3lame at $BITRATE, copy = AAC passthrough (no re-encode)
: "${ICECAST_WAIT_TIMEOUT:=30}"

# Wait for Icecast to accept HTTP requests before starting the pipeline.
# A failed curl PUT in the middle of the streamlink|ffmpeg pipeline tears
# down the whole stack via SIGPIPE.
echo "Waiting for ${ICECAST_URL} to become reachable (timeout ${ICECAST_WAIT_TIMEOUT}s)..."
deadline=$(( $(date +%s) + ICECAST_WAIT_TIMEOUT ))
until curl --silent --fail --max-time 2 "${ICECAST_URL}/" >/dev/null 2>&1; do
    if [ "$(date +%s)" -ge "${deadline}" ]; then
        echo "Timed out waiting for Icecast at ${ICECAST_URL}" >&2
        exit 1
    fi
    sleep 1
done
echo "Icecast is reachable, starting pipeline for channel '${CHANNEL}'."

# Build icecast://user:pass@host:port/mount URL from ICECAST_URL + ICECAST_MOUNT.
# ICECAST_URL is expected to be http://host:port (no trailing slash).
ICECAST_HOST_PORT="${ICECAST_URL#http://}"
ICECAST_HOST_PORT="${ICECAST_HOST_PORT#https://}"
ICECAST_TARGET="icecast://${ICECAST_USER}:${ICECAST_PASS}@${ICECAST_HOST_PORT}${ICECAST_MOUNT}"

case "${ENCODE_MODE}" in
    mp3)
        FFMPEG_ENCODE_OPTS=(-acodec libmp3lame -b:a "${BITRATE}" -write_xing 0)
        FFMPEG_OUTPUT_FORMAT="mp3"
        ICECAST_CONTENT_TYPE="audio/mpeg"
        ;;
    copy)
        # AAC passthrough — re-mux the source audio to ADTS without re-encoding.
        # Maximum quality, but assumes the Twitch stream is AAC (true today).
        FFMPEG_ENCODE_OPTS=(-acodec copy)
        FFMPEG_OUTPUT_FORMAT="adts"
        ICECAST_CONTENT_TYPE="audio/aac"
        ;;
    *)
        echo "Unknown ENCODE_MODE='${ENCODE_MODE}'. Use 'mp3' or 'copy'." >&2
        exit 1
        ;;
esac

streamlink \
    --twitch-disable-ads \
    --hls-live-edge 1 \
    --hls-segment-stream-data \
    --retry-streams 5 \
    --retry-max 3 \
    --stdout "twitch.tv/${CHANNEL}" "${QUALITY}" \
| ffmpeg -hide_banner -loglevel warning \
    -fflags +nobuffer -flags low_delay \
    -i pipe:0 -vn \
    "${FFMPEG_ENCODE_OPTS[@]}" \
    -flush_packets 1 \
    -content_type "${ICECAST_CONTENT_TYPE}" \
    -ice_name "${CHANNEL}" \
    -ice_public 1 \
    -legacy_icecast 1 \
    -f "${FFMPEG_OUTPUT_FORMAT}" \
    "${ICECAST_TARGET}"
