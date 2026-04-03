package changes

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/canopy-ai/canopy/internal/graph"
	"github.com/canopy-ai/canopy/internal/storage"
)

// Detector watches incoming LLM calls for changes in agent behavior:
// - System prompt hash changes (prompt was edited)
// - Model changes (model was swapped)
// - Temperature/config changes
// - Tool list changes (tool added or removed)

type Detector struct {
	pg *storage.Postgres
	mg *graph.Memgraph

	mu             sync.RWMutex
	lastPromptHash map[string]string  // agentID → last seen system_prompt_hash
	lastModel      map[string]string  // agentID → last seen model
	lastTemp       map[string]float64 // agentID → last seen temperature
	lastTools      map[string]string  // agentID → last seen tool list hash
}

func NewDetector(pg *storage.Postgres, mg *graph.Memgraph) *Detector {
	return &Detector{
		pg:             pg,
		mg:             mg,
		lastPromptHash: make(map[string]string),
		lastModel:      make(map[string]string),
		lastTemp:       make(map[string]float64),
		lastTools:      make(map[string]string),
	}
}

// CheckForChanges is called after every LLM call ingestion.
// It compares current call parameters against the last known state for this agent.
func (d *Detector) CheckForChanges(ctx context.Context, call *storage.LLMCall) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	agentID := call.AgentID

	// Check system prompt hash change
	if call.SystemPromptHash != "" {
		if prev, exists := d.lastPromptHash[agentID]; exists && prev != call.SystemPromptHash {
			d.emitChange(ctx, agentID, "prompt_change", "system_prompt_hash", prev, call.SystemPromptHash)
		}
		d.lastPromptHash[agentID] = call.SystemPromptHash
	}

	// Check model change
	if call.Model != "" {
		if prev, exists := d.lastModel[agentID]; exists && prev != call.Model {
			d.emitChange(ctx, agentID, "model_change", "model", prev, call.Model)
		}
		d.lastModel[agentID] = call.Model
	}

	// Check temperature change
	if call.Temperature != nil {
		if prev, exists := d.lastTemp[agentID]; exists && prev != *call.Temperature {
			d.emitChangeFloat(ctx, agentID, "config_change", "temperature", prev, *call.Temperature)
		}
		d.lastTemp[agentID] = *call.Temperature
	}

	return nil
}

func (d *Detector) emitChange(ctx context.Context, agentID, changeType, field, oldVal, newVal string) {
	slog.Info("change detected",
		"agent", agentID,
		"type", changeType,
		"field", field,
		"old", oldVal,
		"new", newVal,
	)

	// Get downstream agents for blast radius
	downstream, err := d.mg.GetDownstream(ctx, agentID)
	if err != nil {
		slog.Warn("failed to get downstream for blast radius", "agent", agentID, "error", err)
	}

	evt := &storage.ChangeEvent{
		ChangeType:       changeType,
		EntityType:       "agent",
		EntityID:         agentID,
		Field:            field,
		OldValueHash:     oldVal,
		NewValueHash:     newVal,
		Author:           "proxy:runtime",
		Source:           "proxy:runtime",
		DownstreamAgents: downstream,
		BlastRadius:      len(downstream),
	}

	if err := d.pg.InsertChangeEvent(ctx, evt); err != nil {
		slog.Error("failed to insert change event", "error", err)
	}
}

func (d *Detector) emitChangeFloat(ctx context.Context, agentID, changeType, field string, oldVal, newVal float64) {
	slog.Info("change detected",
		"agent", agentID,
		"type", changeType,
		"field", field,
		"old", oldVal,
		"new", newVal,
	)

	downstream, _ := d.mg.GetDownstream(ctx, agentID)

	evt := &storage.ChangeEvent{
		ChangeType:       changeType,
		EntityType:       "agent",
		EntityID:         agentID,
		Field:            field,
		OldValueHash:     fmt.Sprintf("%f", oldVal),
		NewValueHash:     fmt.Sprintf("%f", newVal),
		Author:           "proxy:runtime",
		Source:           "proxy:runtime",
		DownstreamAgents: downstream,
		BlastRadius:      len(downstream),
	}

	if err := d.pg.InsertChangeEvent(ctx, evt); err != nil {
		slog.Error("failed to insert change event", "error", err)
	}
}
