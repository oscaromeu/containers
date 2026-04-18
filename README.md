<div align="center">

## Containers

_An opinionated collection of container images for my homelab and side projects._

</div>

<div align="center">

![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/oscaromeu/containers/release.yaml?style=for-the-badge&label=Release)
![GitHub last commit](https://img.shields.io/github/last-commit/oscaromeu/containers?style=for-the-badge)

</div>

> Most of this great work comes from https://github.com/home-operations/containers — go give them a star.

A small set of container images I use across my [home-ops](https://github.com/oscaromeu/home-ops) cluster and personal projects. Browse published images on the [GitHub Packages page](https://github.com/oscaromeu?tab=packages&repo_name=containers).

> **Heritage**
>
> This project started as a fork of [home-operations/containers](https://github.com/home-operations/containers) and keeps their opinionated, KISS-first philosophy. The CI pipeline, Renovate configuration, and build conventions are theirs — I trimmed apps I don't use, added my own, and adapted a few workflows (Trivy scan with PR comments, App-token auth, attribution to my own GitHub App). If this repo is useful to you, consider starring the upstream.

## Apps

| App | Base image | Notes |
|-----|------------|-------|
| [`actions-runner`](./apps/actions-runner) | `ghcr.io/actions/actions-runner` | GitHub Actions self-hosted runner with `yq`, `gh`, Homebrew |
| [`webhook`](./apps/webhook) | `python:3.13-alpine` | [`adnanh/webhook`](https://github.com/adnanh/webhook) + apprise + `gcloud` |

## Principles

- **Semantic versioning** — image tags follow [semver](https://semver.org/) when upstream provides it.
- **Rootless** — containers run as a non-root user (`65534:65534`, a.k.a. `nobody:nogroup`) whenever practical.
- **Multi-architecture** — images are built for `linux/amd64` and `linux/arm64`.
- **One process per container** — no `s6-overlay`, no `gosu`, logs to stdout.
- **Immutable via digest** — tags like `rolling` and `27.0` move; pin to `@sha256:...` in production.

## Features

### Tag immutability — pin by digest

Only the `sha256` digest is truly immutable.

| Reference | Immutable |
|-----------|-----------|
| `ghcr.io/oscaromeu/webhook:rolling` | ❌ |
| `ghcr.io/oscaromeu/webhook:2.8.2` | ❌ |
| `ghcr.io/oscaromeu/webhook:2.8.2@sha256:abc1…` | ✅ |

Pair this with [Renovate](https://github.com/renovatebot/renovate): it can update tag **and** pinned digest automatically.

### Rootless

Most images run as `UID/GID 65534:65534`. On Kubernetes, use `fsGroup` so the Kubelet sets volume ownership to the same GID at mount time:

```yaml
spec:
  containers:
    - name: webhook
      image: ghcr.io/oscaromeu/webhook:2.8.2
      securityContext:
        allowPrivilegeEscalation: false
        capabilities:
          drop: [ALL]
  securityContext:
    fsGroup: 65534
    fsGroupChangePolicy: OnRootMismatch
```

`OnRootMismatch` skips the recursive chown once the volume root already matches the fsGroup — a noticeable win on stateful workloads with large working sets.

### Passing arguments

Apps that expect flags instead of env vars:

```yaml
args:
  - --port
  - "8080"
```

### Verify image signature

Images are signed with [`attest-build-provenance`](https://github.com/actions/attest-build-provenance):

```sh
gh attestation verify --repo oscaromeu/containers \
  oci://ghcr.io/oscaromeu/${APP}:${TAG}
```

Or with [cosign](https://github.com/sigstore/cosign):

```sh
cosign verify-attestation --new-bundle-format --type slsaprovenance1 \
    --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
    --certificate-identity-regexp "^https://github.com/oscaromeu/containers/.github/workflows/app-builder.yaml@refs/heads/main" \
    ghcr.io/oscaromeu/${APP}:${TAG}
```

### Vulnerability scanning

- **Trivy** runs on every build and posts a sticky comment on the PR with CRITICAL/HIGH CVEs and outdated OS packages. On main, the scan result is committed back to `apps/<app>/trivy.json` so the history is visible via `git log`.
- **Grype** runs as a nightly cron scan and uploads SARIF to the repo's Security tab for an aggregated view.

## Working locally

- [`mise`](https://mise.jdx.dev) manages the tool chain (Go, `just`, `jq`, `yq`, `lefthook`). Run `mise install` once.
- Build and test a single app locally: `just local-build <app>`.
- Trigger a remote build: `just remote-build <app> [release]`.

## Credits

This repository started as a fork of [home-operations/containers](https://github.com/home-operations/containers) — most of the CI plumbing, Renovate configuration, and build philosophy is directly theirs. I also drew inspiration from [hotio.dev](https://hotio.dev/) and [linuxserver.io](https://www.linuxserver.io/).

All mistakes in this repo are mine.
