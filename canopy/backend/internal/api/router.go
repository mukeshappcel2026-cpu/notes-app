package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/canopy-ai/canopy/internal/graph"
	"github.com/canopy-ai/canopy/internal/ingestion"
	"github.com/canopy-ai/canopy/internal/storage"
)

func NewRouter(pg *storage.Postgres, mg *graph.Memgraph, ingestor *ingestion.Service) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Ingestion
	mux.HandleFunc("POST /api/v1/ingest/llm", handleIngestLLM(ingestor))
	mux.HandleFunc("POST /api/v1/ingest/agent-call", handleIngestAgentCall(ingestor))

	// Agents
	mux.HandleFunc("GET /api/v1/agents", handleListAgents(pg))
	mux.HandleFunc("GET /api/v1/agents/{id}", handleGetAgent(pg))

	// Changes
	mux.HandleFunc("GET /api/v1/changes", handleListChanges(pg))

	// Graph
	mux.HandleFunc("GET /api/v1/graph", handleGetGraph(mg))
	mux.HandleFunc("GET /api/v1/graph/downstream/{id}", handleGetDownstream(mg))
	mux.HandleFunc("GET /api/v1/graph/upstream/{id}", handleGetUpstream(mg))

	// Wrap with logging middleware
	return logMiddleware(mux)
}

func handleIngestLLM(ingestor *ingestion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ingestion.IngestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		if req.AgentID == "" {
			http.Error(w, `{"error":"agent field is required"}`, http.StatusBadRequest)
			return
		}
		if req.Model == "" {
			http.Error(w, `{"error":"model field is required"}`, http.StatusBadRequest)
			return
		}

		if err := ingestor.IngestLLMCall(r.Context(), &req); err != nil {
			slog.Error("ingestion failed", "error", err)
			http.Error(w, `{"error":"ingestion failed"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	}
}

func handleIngestAgentCall(ingestor *ingestion.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var evt ingestion.AgentCallEvent
		if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		if evt.From == "" || evt.To == "" {
			http.Error(w, `{"error":"from and to fields are required"}`, http.StatusBadRequest)
			return
		}

		if err := ingestor.IngestAgentCall(r.Context(), &evt); err != nil {
			slog.Error("agent call ingestion failed", "error", err)
			http.Error(w, `{"error":"ingestion failed"}`, http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	}
}

func handleListAgents(pg *storage.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agents, err := pg.ListAgents(r.Context())
		if err != nil {
			slog.Error("list agents failed", "error", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		if agents == nil {
			agents = []storage.Agent{}
		}
		json.NewEncoder(w).Encode(map[string]any{"agents": agents})
	}
}

func handleGetAgent(pg *storage.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		agent, err := pg.GetAgent(r.Context(), id)
		if err != nil {
			slog.Error("get agent failed", "error", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		if agent == nil {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(agent)
	}
}

func handleListChanges(pg *storage.Postgres) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		events, err := pg.ListChangeEvents(r.Context(), limit)
		if err != nil {
			slog.Error("list changes failed", "error", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		if events == nil {
			events = []storage.ChangeEvent{}
		}
		json.NewEncoder(w).Encode(map[string]any{"changes": events})
	}
}

func handleGetGraph(mg *graph.Memgraph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g, err := mg.GetDependencyGraph(r.Context())
		if err != nil {
			slog.Error("get graph failed", "error", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(g)
	}
}

func handleGetDownstream(mg *graph.Memgraph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		agents, err := mg.GetDownstream(r.Context(), id)
		if err != nil {
			slog.Error("get downstream failed", "error", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		if agents == nil {
			agents = []string{}
		}
		json.NewEncoder(w).Encode(map[string]any{"agent": id, "downstream": agents, "blast_radius": len(agents)})
	}
}

func handleGetUpstream(mg *graph.Memgraph) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		agents, err := mg.GetUpstream(r.Context(), id)
		if err != nil {
			slog.Error("get upstream failed", "error", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		if agents == nil {
			agents = []string{}
		}
		json.NewEncoder(w).Encode(map[string]any{"agent": id, "upstream": agents})
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("request", "method", r.Method, "path", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
