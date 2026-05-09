# syntax=docker/dockerfile:1

FROM docker.io/library/python:3.13-alpine3.23
ARG TARGETARCH
ARG VERSION

ENV \
    PIP_BREAK_SYSTEM_PACKAGES=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_ROOT_USER_ACTION=ignore \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

USER root

# Tooling philosophy: enough to (a) write small Python scripts that hit
# cluster APIs (Subsonic etc) and (b) replace `kubectl run --rm -it` with
# a useful debug pod for any kind of network/DNS/system inspection —
# netshoot vibes, Alpine size. I know this Container is chunky.
RUN apk add --no-cache \
        # --- Shell + core ---
        bash \
        bash-completion \
        catatonit \
        coreutils \
        findutils \
        gawk \
        grep \
        less \
        sed \
        tmux \
        tzdata \
        util-linux \
        # --- Editors ---
        nano \
        vim \
        # --- Process inspection ---
        htop \
        iotop \
        lsof \
        procps \
        strace \
        # --- Cert / time / crypto ---
        ca-certificates \
        gnupg \
        openssl \
        # --- DNS ---
        bind-tools \
        drill \
        whois \
        # --- Routing / interfaces ---
        arp-scan \
        bridge-utils \
        ethtool \
        iproute2 \
        iputils \
        iptables \
        net-tools \
        nftables \
        traceroute \
        # --- Connectivity probes ---
        arping \
        busybox-extras \
        fping \
        iftop \
        mtr \
        netcat-openbsd \
        socat \
        tcpdump \
        # --- Application-level HTTP / data ---
        curl \
        jo \
        jq \
        openssh-client \
        wget \
        yq \
        # --- Network scanning ---
        nmap \
        # --- Throughput / bench ---
        apache2-utils \
        iperf3 \
        # --- Database / cache CLIs ---
        mariadb-client \
        postgresql-client \
        redis \
        # --- Compression / archive ---
        unzip \
        zip \
        zstd \
        # --- Kubernetes / cloud ---
        helm \
        kubectl \
    && pip install --no-cache-dir \
        click \
        cryptography \
        httpie \
        httpx \
        kubernetes \
        paramiko \
        pyyaml \
        requests \
        rich \
        tabulate \
    && rm -rf /var/cache/apk/* /tmp/*

USER nobody:nogroup
WORKDIR /tmp

# Default entrypoint is bash so this image works both as an interactive
# debug pod (`kubectl run --rm -it ...`) and as a base for CronJobs that
# override command/args.
ENTRYPOINT ["/usr/bin/catatonit", "--", "/bin/bash"]
