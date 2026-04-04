package storage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.MaxConns = 20
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() {
	p.pool.Close()
}

// LLM Call Storage

type LLMCall struct {
	ID               uuid.UUID
	Timestamp        time.Time
	AgentID          string
	Team             string
	Model            string
	Provider         string
	SystemPromptHash string
	Temperature      *float64
	MaxTokens        *int
	InputTokens      int
	OutputTokens     int
	CostUSD          float64
	LatencyMs        float64
	ToolCalls        []byte // JSON
	ToolCallCount    int
	Status           string
	ErrorMessage     string
	Metadata         []byte // JSON
}

func (p *Postgres) InsertLLMCall(ctx context.Context, call *LLMCall) error {
	call.ID = uuid.New()
	if call.Timestamp.IsZero() {
		call.Timestamp = time.Now()
	}

	_, err := p.pool.Exec(ctx, `
		INSERT INTO llm_calls (
			id, timestamp, agent_id, team, model, provider,
			system_prompt_hash, temperature, max_tokens,
			input_tokens, output_tokens, cost_usd, latency_ms,
			tool_calls, tool_call_count, status, error_message, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17, $18
		)`,
		call.ID, call.Timestamp, call.AgentID, call.Team, call.Model, call.Provider,
		call.SystemPromptHash, call.Temperature, call.MaxTokens,
		call.InputTokens, call.OutputTokens, call.CostUSD, call.LatencyMs,
		call.ToolCalls, call.ToolCallCount, call.Status, call.ErrorMessage, call.Metadata,
	)
	return err
}

// Agent Registry

type Agent struct {
	ID        string
	FirstSeen time.Time
	LastSeen  time.Time
	Team      string
	CallCount int64
	ModelsUsed []string
	ToolsUsed  []string
}

func (p *Postgres) UpsertAgent(ctx context.Context, agentID, team string, model string, tools []string) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO agents (id, team, call_count, models_used, tools_used)
		VALUES ($1, $2, 1, ARRAY[$3], $4)
		ON CONFLICT (id) DO UPDATE SET
			last_seen = NOW(),
			call_count = agents.call_count + 1,
			team = COALESCE(NULLIF($2, ''), agents.team),
			models_used = ARRAY(SELECT DISTINCT unnest(agents.models_used || ARRAY[$3])),
			tools_used = ARRAY(SELECT DISTINCT unnest(agents.tools_used || $4))`,
		agentID, team, model, tools,
	)
	return err
}

func (p *Postgres) ListAgents(ctx context.Context) ([]Agent, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT id, first_seen, last_seen, COALESCE(team, ''), call_count,
		       COALESCE(models_used, '{}'), COALESCE(tools_used, '{}')
		FROM agents ORDER BY call_count DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.FirstSeen, &a.LastSeen, &a.Team, &a.CallCount, &a.ModelsUsed, &a.ToolsUsed); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (p *Postgres) GetAgent(ctx context.Context, id string) (*Agent, error) {
	var a Agent
	err := p.pool.QueryRow(ctx, `
		SELECT id, first_seen, last_seen, COALESCE(team, ''), call_count,
		       COALESCE(models_used, '{}'), COALESCE(tools_used, '{}')
		FROM agents WHERE id = $1`, id).
		Scan(&a.ID, &a.FirstSeen, &a.LastSeen, &a.Team, &a.CallCount, &a.ModelsUsed, &a.ToolsUsed)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &a, err
}

// Change Events

type ChangeEvent struct {
	ID              uuid.UUID
	Timestamp       time.Time
	ChangeType      string
	EntityType      string
	EntityID        string
	Field           string
	OldValueHash    string
	NewValueHash    string
	Diff            []byte // JSON
	Author          string
	Source          string
	DownstreamAgents []string
	BlastRadius     int
}

func (p *Postgres) InsertChangeEvent(ctx context.Context, evt *ChangeEvent) error {
	evt.ID = uuid.New()
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}

	_, err := p.pool.Exec(ctx, `
		INSERT INTO change_events (
			id, timestamp, change_type, entity_type, entity_id,
			field, old_value_hash, new_value_hash, diff,
			author, source, downstream_agents, blast_radius
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		evt.ID, evt.Timestamp, evt.ChangeType, evt.EntityType, evt.EntityID,
		evt.Field, evt.OldValueHash, evt.NewValueHash, evt.Diff,
		evt.Author, evt.Source, evt.DownstreamAgents, evt.BlastRadius,
	)
	return err
}

func (p *Postgres) ListChangeEvents(ctx context.Context, limit int) ([]ChangeEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := p.pool.Query(ctx, `
		SELECT id, timestamp, change_type, entity_type, entity_id,
		       COALESCE(field, ''), COALESCE(old_value_hash, ''), COALESCE(new_value_hash, ''),
		       diff, COALESCE(author, ''), COALESCE(source, ''),
		       COALESCE(downstream_agents, '{}'), COALESCE(blast_radius, 0)
		FROM change_events ORDER BY timestamp DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ChangeEvent
	for rows.Next() {
		var e ChangeEvent
		if err := rows.Scan(
			&e.ID, &e.Timestamp, &e.ChangeType, &e.EntityType, &e.EntityID,
			&e.Field, &e.OldValueHash, &e.NewValueHash, &e.Diff,
			&e.Author, &e.Source, &e.DownstreamAgents, &e.BlastRadius,
		); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Agent stats for discovery
type AgentStats struct {
	AgentID      string
	CallCount    int64
	ModelsUsed   []string
	ToolsUsed    []string
	AvgLatencyMs float64
	AvgTokensIn  float64
	AvgTokensOut float64
	ErrorRate    float64
}

func (p *Postgres) GetAgentStats(ctx context.Context, agentID string, since time.Time) (*AgentStats, error) {
	var s AgentStats
	s.AgentID = agentID

	err := p.pool.QueryRow(ctx, `
		SELECT
			COUNT(*),
			AVG(latency_ms),
			AVG(input_tokens),
			AVG(output_tokens),
			AVG(CASE WHEN status != 'success' THEN 1.0 ELSE 0.0 END)
		FROM llm_calls
		WHERE agent_id = $1 AND timestamp >= $2`, agentID, since).
		Scan(&s.CallCount, &s.AvgLatencyMs, &s.AvgTokensIn, &s.AvgTokensOut, &s.ErrorRate)
	if err != nil {
		return nil, err
	}

	agent, err := p.GetAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	if agent != nil {
		s.ModelsUsed = agent.ModelsUsed
		s.ToolsUsed = agent.ToolsUsed
	}
	return &s, nil
}
