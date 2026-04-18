# syntax=docker/dockerfile:1

ARG VERSION
FROM docker.io/blockstream/bitcoind:${VERSION}

# Base is Debian bookworm where `nogroup` resolves to GID 65534. On Alpine the
# group for UID 65534 is called `nobody` instead, so `nobody:nogroup` would not
# map to 65534:65534 there. If the upstream base ever switches, prefer the
# numeric form `65534:65534` for portability.
USER nobody:nogroup
