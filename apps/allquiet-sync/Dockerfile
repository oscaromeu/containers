# syntax=docker/dockerfile:1

FROM docker.io/library/golang:1.25-alpine AS builder
ARG TARGETARCH
ARG VERSION

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY allquiet ./allquiet
COPY store ./store
COPY sync ./sync
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
        go build -trimpath -ldflags="-s -w" \
        -installsuffix 'static' -o /out/allquiet-sync ./cmd/allquiet-sync

FROM scratch
ARG VERSION

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/allquiet-sync /app/allquiet-sync

USER 65534:65534

ENTRYPOINT ["/app/allquiet-sync"]
