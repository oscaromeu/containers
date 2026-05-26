# syntax=docker/dockerfile:1

FROM docker.io/library/golang:1.25-alpine AS builder
ARG TARGETARCH
ARG VERSION

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
        go build -trimpath -ldflags="-s -w" \
        -installsuffix 'static' -o /out/alertmanager-webhook-logger .

FROM scratch
ARG VERSION

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/alertmanager-webhook-logger /app/alertmanager-webhook-logger

USER 65534:65534

EXPOSE 6725

ENTRYPOINT ["/app/alertmanager-webhook-logger"]
