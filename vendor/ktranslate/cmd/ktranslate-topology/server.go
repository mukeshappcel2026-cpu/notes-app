package main

import (
	"embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
)

//go:embed assets/*
var assetsFS embed.FS

// Server wires the graph store to the HTTP endpoints the viewer needs:
//
//	POST /ingest     — receive JCHF JSON from ktranslate's http sink
//	GET  /graph.json — current graph snapshot (what the browser polls)
//	GET  /           — the static HTML viewer
//	GET  /healthz    — liveness probe
type Server struct {
	Graph *Graph
	Log   *log.Logger
}

// Handler returns a net/http.Handler that routes all four endpoints.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/ingest", s.handleIngest)
	mux.HandleFunc("/graph.json", s.handleGraph)
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/", s.handleIndex)

	return mux
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Cap the body at 16 MiB so a buggy upstream can't OOM us.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	accepted, total, err := ParseAndApply(body, s.Graph)
	if err != nil {
		s.logf("ingest parse error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.logf("ingest: accepted=%d total=%d", accepted, total)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{
		"accepted": accepted,
		"total":    total,
	})
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	snap := s.Graph.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(snap); err != nil {
		s.logf("graph encode error: %v", err)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := assetsFS.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) logf(format string, args ...interface{}) {
	if s.Log != nil {
		s.Log.Printf(format, args...)
	}
}
