# syntax=docker/dockerfile:1

FROM docker.io/library/alpine:3.23
ARG TARGETARCH
ARG VERSION

USER root

RUN apk add --no-cache \
        bash \
        ca-certificates \
        catatonit \
        curl \
        gettext \
        icecast \
        tzdata \
    && rm -rf /var/cache/apk/* /tmp/* \
    && mkdir -p /var/log/icecast \
    && chown -R nobody:nogroup /var/log/icecast

COPY icecast.xml /etc/icecast.xml.template
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER nobody:nogroup
WORKDIR /tmp

EXPOSE 8000

ENTRYPOINT ["/usr/bin/catatonit", "--", "/entrypoint.sh"]
