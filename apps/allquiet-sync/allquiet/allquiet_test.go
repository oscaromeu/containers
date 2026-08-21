package allquiet

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchIncidentsPagesUntilDone(t *testing.T) {
	var gotAuth string
	var offsets []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("X-Authorization")
		if r.URL.Path != "/api/public/v1/incident/search/list" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		offsets = append(offsets, q.Get("Offset"))
		if q.Get("TeamIds") != "team-1" {
			t.Errorf("TeamIds = %s", q.Get("TeamIds"))
		}
		hasMore := q.Get("Offset") == "0"
		fmt.Fprintf(w, `{"incidents":[{"id":"inc-%s","title":"t"}],"hasMore":%v}`, q.Get("Offset"), hasMore)
	}))
	defer srv.Close()

	c, err := New("secret", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	incidents, err := c.SearchIncidents(context.Background(), SearchParams{
		TeamIDs:         []string{"team-1"},
		LastUpdatedFrom: time.Now().Add(-time.Hour),
		Limit:           2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "secret" {
		t.Fatalf("X-Authorization = %q", gotAuth)
	}
	if len(incidents) != 2 || incidents[0].ID != "inc-0" || incidents[1].ID != "inc-2" {
		t.Fatalf("incidents = %+v", incidents)
	}
	if fmt.Sprint(offsets) != "[0 2]" {
		t.Fatalf("offsets = %v", offsets)
	}
}

func TestGetErrorsIncludeStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c, _ := New("bad-key", WithBaseURL(srv.URL))
	_, err := c.GetIncident(context.Background(), "x")
	if err == nil {
		t.Fatal("want error on 401")
	}
}

func TestNewRejectsEmptyKey(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("want error on empty api key")
	}
}
