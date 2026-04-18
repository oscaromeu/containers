# syntax=docker/dockerfile:1

ARG VERSION
FROM docker.io/blockstream/bitcoind:${VERSION}

USER root

# Apply latest security patches from Debian bookworm and drop wget
# (only used during upstream build, not needed at runtime).
RUN apt-get -qq update \
    && apt-get -qq -y upgrade \
    && apt-get -qq -y purge wget \
    && apt-get -qq -y autoremove \
    && rm -rf /var/lib/apt/lists/*

# Base is Debian bookworm where `nogroup` resolves to GID 65534. On Alpine the
# group for UID 65534 is called `nobody` instead, so `nobody:nogroup` would not
# map to 65534:65534 there. If the upstream base ever switches, prefer the
# numeric form `65534:65534` for portability.

USER nobody:nogroup
