package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandlerLogsEachAlert(t *testing.T) {
	var buf bytes.Buffer
	h := &handler{logger: slog.New(slog.NewJSONHandler(&buf, nil))}

	payload := Webhook{
		Receiver: "webhook",
		Status:   "firing",
		CommonLabels: map[string]string{
			"alertname": "TestAlert",
			"severity":  "warning",
		},
		CommonAnnotations: map[string]string{
			"summary": "test summary",
		},
		ExternalURL: "http://alertmanager.example.org",
		Alerts: []Alert{
			{
				Status:       "firing",
				Labels:       map[string]string{"instance": "node-1"},
				Annotations:  map[string]string{"description": "node down"},
				StartsAt:     time.Date(2026, 5, 26, 8, 0, 0, 0, time.UTC),
				Fingerprint:  "abc123",
				GeneratorURL: "http://prometheus.example.org/graph",
			},
			{
				Status:      "firing",
				Labels:      map[string]string{"instance": "node-2"},
				Fingerprint: "def456",
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status code: got %d, want %d", rec.Code, http.StatusNoContent)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != len(payload.Alerts) {
		t.Fatalf("log lines: got %d, want %d\n%s", len(lines), len(payload.Alerts), buf.String())
	}

	wantFirst := []string{
		`"alertname":"TestAlert"`,
		`"severity":"warning"`,
		`"summary":"test summary"`,
		`"instance":"node-1"`,
		`"description":"node down"`,
		`"fingerprint":"abc123"`,
		`"receiver":"webhook"`,
		`"externalURL":"http://alertmanager.example.org"`,
	}
	for _, want := range wantFirst {
		if !strings.Contains(lines[0], want) {
			t.Errorf("first log line missing %s\nline: %s", want, lines[0])
		}
	}

	if !strings.Contains(lines[1], `"instance":"node-2"`) || !strings.Contains(lines[1], `"fingerprint":"def456"`) {
		t.Errorf("second log line missing per-alert fields\nline: %s", lines[1])
	}
}

func TestHandlerRejectsInvalidJSON(t *testing.T) {
	h := &handler{logger: slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{not json"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status code: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
