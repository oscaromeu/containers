# syntax=docker/dockerfile:1

FROM docker.io/library/alpine:3.23
ARG TARGETARCH
ARG VERSION

USER root

RUN apk add --no-cache \
        bash \
        ca-certificates \
        catatonit \
        coreutils \
        curl \
        ffmpeg \
        streamlink \
        tzdata \
    && rm -rf /var/cache/apk/* /tmp/*

COPY . /

USER nobody:nogroup
WORKDIR /tmp

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]
