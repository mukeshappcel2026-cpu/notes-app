package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestServer() *Server {
	g := NewGraph(time.Hour)
	// Pin wall clock to the fixture timestamp used by sampleRecord so
	// TTL pruning doesn't drop test data.
	g.now = func() time.Time { return time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC) }
	return &Server{Graph: g}
}

func TestServer_IngestAndGraph(t *testing.T) {
	srv := newTestServer()
	h := srv.Handler()

	// POST a single topology record.
	body := mustMarshal(t, sampleRecord("sw-a", "Eth1", "sw-b", "Eth2", "lldp"))
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ingest: want 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	var resp map[string]int
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if resp["accepted"] != 1 {
		t.Errorf("want accepted=1, got %v", resp)
	}

	// GET the snapshot.
	req = httptest.NewRequest(http.MethodGet, "/graph.json", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("graph: want 200, got %d", rec.Code)
	}
	var snap Snapshot
	if err := json.NewDecoder(rec.Body).Decode(&snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snap.Counts["devices"] != 2 {
		t.Errorf("want 2 devices, got %d", snap.Counts["devices"])
	}
	if snap.Counts["links"] != 1 {
		t.Errorf("want 1 link, got %d", snap.Counts["links"])
	}
}

func TestServer_IngestRejectsGET(t *testing.T) {
	srv := newTestServer()
	h := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/ingest", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", rec.Code)
	}
}

func TestServer_IndexServesHTML(t *testing.T) {
	srv := newTestServer()
	h := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("want html content-type, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "<svg") {
		t.Errorf("expected the HTML to contain an <svg> root")
	}
}

func TestServer_Healthz(t *testing.T) {
	srv := newTestServer()
	h := srv.Handler()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("want 'ok', got %q", rec.Body.String())
	}
}
