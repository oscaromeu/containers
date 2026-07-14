# syntax=docker/dockerfile:1

ARG VERSION
FROM ghcr.io/kieraneglin/pinchflat:${VERSION}
ARG TARGETARCH

# renovate: datasource=github-releases depName=yt-dlp/yt-dlp versioning=loose
ARG YT_DLP_VERSION=2026.07.04
# renovate: datasource=github-releases depName=denoland/deno
ARG DENO_VERSION=2.9.2

# https://github.com/yt-dlp/yt-dlp/issues/14404
RUN export DENO_ARCH=$(case ${TARGETARCH} in \
        amd64) echo "x86_64"  ;; \
        arm64) echo "aarch64" ;; \
    esac) \
    && apt-get -qq update \
    && apt-get -qq -y upgrade \
    && apt-get -qq -y install --no-install-recommends unzip \
    && apt-get -qq -y autoremove \
    && rm -rf /var/lib/apt/lists/* \
    && cd /tmp \
    && curl -fsSLO "https://github.com/yt-dlp/yt-dlp/releases/download/${YT_DLP_VERSION}/yt-dlp" \
    && curl -fsSL -o SHA2-256SUMS "https://github.com/yt-dlp/yt-dlp/releases/download/${YT_DLP_VERSION}/SHA2-256SUMS" \
    && grep -E '^[0-9a-f]{64}  yt-dlp$' SHA2-256SUMS | sha256sum -c - \
    && install -m 0755 yt-dlp /usr/local/bin/yt-dlp \
    && curl -fsSLO "https://github.com/denoland/deno/releases/download/v${DENO_VERSION}/deno-${DENO_ARCH}-unknown-linux-gnu.zip" \
    && curl -fsSLO "https://github.com/denoland/deno/releases/download/v${DENO_VERSION}/deno-${DENO_ARCH}-unknown-linux-gnu.zip.sha256sum" \
    && sha256sum -c "deno-${DENO_ARCH}-unknown-linux-gnu.zip.sha256sum" \
    && unzip -q "deno-${DENO_ARCH}-unknown-linux-gnu.zip" \
    && install -m 0755 deno /usr/local/bin/deno \
    && chmod 0755 /usr/local/bin \
    && rm -rf /tmp/* \
    && yt-dlp --version \
    && deno --version
