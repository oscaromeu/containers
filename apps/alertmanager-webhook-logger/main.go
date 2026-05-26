// Package main implements a tiny Alertmanager webhook receiver that writes
// each alert as one structured JSON log line to stdout.
//
// Intended use: ship Alertmanager's notification history to a log store
// (VictoriaLogs, Loki, Elasticsearch, ...) via the cluster's existing
// pod-log collection path. Alertmanager itself keeps no persistent state.
//
// Derived from https://github.com/tomtom-international/alertmanager-webhook-logger
// (Apache 2.0). See LICENSE for the original copyright notice.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Alert mirrors one entry of the Alertmanager webhook payload's "alerts" array.
// See: https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
type Alert struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// Webhook mirrors the top-level Alertmanager webhook payload.
type Webhook struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []Alert           `json:"alerts"`
}

func main() {
	var (
		address     = flag.String("address", ":6725", "TCP address to listen on")
		tlsEnabled  = flag.Bool("tls", false, "serve HTTPS instead of HTTP")
		tlsKeyPath  = flag.String("tls-key", "key.pem", "path to PEM-encoded TLS private key")
		tlsCertPath = flag.String("tls-cert", "cert.pem", "path to PEM-encoded TLS certificate")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	mux := http.NewServeMux()
	mux.Handle("POST /", &handler{logger: logger})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              *address,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		var err error
		if *tlsEnabled {
			err = srv.ListenAndServeTLS(*tlsCertPath, *tlsKeyPath)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	logger.Info("server starting", "address", *address, "tls", *tlsEnabled)

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "err", err)
			os.Exit(1)
		}
		logger.Info("shutdown complete")
	}
}

type handler struct {
	logger *slog.Logger
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var payload Webhook
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, alert := range payload.Alerts {
		h.logger.LogAttrs(r.Context(), slog.LevelInfo, "alert", alertAttrs(payload, alert)...)
	}

	w.WriteHeader(http.StatusNoContent)
}

// alertAttrs flattens common + per-alert labels/annotations into slog attributes.
// Per-alert keys are appended last so they win when slog's JSON handler resolves
// duplicates at decode time (matches the upstream behaviour).
func alertAttrs(p Webhook, a Alert) []slog.Attr {
	attrs := make([]slog.Attr, 0, 7+len(p.CommonLabels)+len(p.CommonAnnotations)+len(p.GroupLabels)+len(a.Labels)+len(a.Annotations))

	attrs = append(attrs,
		slog.String("status", a.Status),
		slog.Time("startsAt", a.StartsAt),
		slog.Time("endsAt", a.EndsAt),
		slog.String("generatorURL", a.GeneratorURL),
		slog.String("externalURL", p.ExternalURL),
		slog.String("receiver", p.Receiver),
		slog.String("fingerprint", a.Fingerprint),
	)

	for k, v := range p.CommonAnnotations {
		attrs = append(attrs, slog.String(k, v))
	}
	for k, v := range p.CommonLabels {
		attrs = append(attrs, slog.String(k, v))
	}
	for k, v := range p.GroupLabels {
		attrs = append(attrs, slog.String(k, v))
	}
	for k, v := range a.Labels {
		attrs = append(attrs, slog.String(k, v))
	}
	for k, v := range a.Annotations {
		attrs = append(attrs, slog.String(k, v))
	}

	return attrs
}
