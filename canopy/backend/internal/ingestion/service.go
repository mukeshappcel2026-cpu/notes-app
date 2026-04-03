package ingestion

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/canopy-ai/canopy/internal/graph"
	"github.com/canopy-ai/canopy/internal/storage"
)

// IngestRequest is the payload from the LiteLLM callback / SDK.
type IngestRequest struct {
	Timestamp        string   `json:"timestamp"`
	AgentID          string   `json:"agent"`
	Team             string   `json:"team,omitempty"`
	Model            string   `json:"model"`
	Provider         string   `json:"provider,omitempty"`
	SystemPromptHash string   `json:"system_prompt_hash"`
	Temperature      *float64 `json:"temperature,omitempty"`
	MaxTokens        *int     `json:"max_tokens,omitempty"`
	InputTokens      int      `json:"input_tokens"`
	OutputTokens     int      `json:"output_tokens"`
	CostUSD          float64  `json:"cost"`
	LatencyMs        float64  `json:"latency_ms"`
	ToolCalls        []string `json:"tool_calls,omitempty"`
	Status           string   `json:"status"`
	ErrorMessage     string   `json:"error_message,omitempty"`
	Metadata         any      `json:"metadata,omitempty"`
}

// AgentCallEvent is emitted when one agent calls another.
type AgentCallEvent struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type OnLLMCallFunc func(ctx context.Context, call *storage.LLMCall) error

type Service struct {
	pg          *storage.Postgres
	mg          *graph.Memgraph
	onLLMCall   []OnLLMCallFunc
}

func NewService(pg *storage.Postgres, mg *graph.Memgraph) *Service {
	return &Service{pg: pg, mg: mg}
}

// OnLLMCall registers a callback for post-ingestion processing (e.g., change detection).
func (s *Service) OnLLMCall(fn OnLLMCallFunc) {
	s.onLLMCall = append(s.onLLMCall, fn)
}

// IngestLLMCall processes an incoming LLM call event.
func (s *Service) IngestLLMCall(ctx context.Context, req *IngestRequest) error {
	// Marshal tool calls and metadata to JSON
	toolCallsJSON, _ := json.Marshal(req.ToolCalls)
	metadataJSON, _ := json.Marshal(req.Metadata)

	call := &storage.LLMCall{
		AgentID:          req.AgentID,
		Team:             req.Team,
		Model:            req.Model,
		Provider:         req.Provider,
		SystemPromptHash: req.SystemPromptHash,
		Temperature:      req.Temperature,
		MaxTokens:        req.MaxTokens,
		InputTokens:      req.InputTokens,
		OutputTokens:     req.OutputTokens,
		CostUSD:          req.CostUSD,
		LatencyMs:        req.LatencyMs,
		ToolCalls:        toolCallsJSON,
		ToolCallCount:    len(req.ToolCalls),
		Status:           req.Status,
		ErrorMessage:     req.ErrorMessage,
		Metadata:         metadataJSON,
	}

	// 1. Store raw LLM call in Postgres
	if err := s.pg.InsertLLMCall(ctx, call); err != nil {
		return err
	}

	// 2. Upsert agent in registry
	if err := s.pg.UpsertAgent(ctx, req.AgentID, req.Team, req.Model, req.ToolCalls); err != nil {
		slog.Warn("failed to upsert agent", "agent", req.AgentID, "error", err)
	}

	// 3. Update graph: agent → model edges
	if err := s.mg.EnsureAgent(ctx, req.AgentID, req.Team); err != nil {
		slog.Warn("failed to ensure agent in graph", "agent", req.AgentID, "error", err)
	}
	if err := s.mg.EnsureModel(ctx, req.AgentID, req.Model); err != nil {
		slog.Warn("failed to ensure model edge", "agent", req.AgentID, "model", req.Model, "error", err)
	}

	// 4. Update graph: agent → tool edges
	for _, tool := range req.ToolCalls {
		if err := s.mg.EnsureTool(ctx, req.AgentID, tool); err != nil {
			slog.Warn("failed to ensure tool edge", "agent", req.AgentID, "tool", tool, "error", err)
		}
	}

	// 5. Run post-ingestion callbacks (change detection, etc.)
	for _, fn := range s.onLLMCall {
		if err := fn(ctx, call); err != nil {
			slog.Warn("post-ingestion callback failed", "error", err)
		}
	}

	slog.Debug("ingested llm call", "agent", req.AgentID, "model", req.Model, "tokens", req.InputTokens+req.OutputTokens)
	return nil
}

// IngestAgentCall records an agent-to-agent communication event.
func (s *Service) IngestAgentCall(ctx context.Context, evt *AgentCallEvent) error {
	if err := s.mg.RecordAgentCall(ctx, evt.From, evt.To); err != nil {
		return err
	}
	slog.Debug("ingested agent call", "from", evt.From, "to", evt.To)
	return nil
}
