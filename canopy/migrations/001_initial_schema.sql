-- Canopy: Change Intelligence for AI Agent Systems
-- Initial schema: change events, LLM call logs, behavioral fingerprints, drift events

-- Raw LLM call log (every call through the proxy)
CREATE TABLE llm_calls (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    agent_id        TEXT NOT NULL,
    team            TEXT,
    model           TEXT NOT NULL,
    provider        TEXT,
    system_prompt_hash TEXT,
    temperature     FLOAT,
    max_tokens      INT,
    input_tokens    INT NOT NULL,
    output_tokens   INT NOT NULL,
    cost_usd        FLOAT,
    latency_ms      FLOAT NOT NULL,
    tool_calls      JSONB,
    tool_call_count INT DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'success',
    error_message   TEXT,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_llm_calls_agent ON llm_calls(agent_id, timestamp DESC);
CREATE INDEX idx_llm_calls_timestamp ON llm_calls(timestamp DESC);
CREATE INDEX idx_llm_calls_model ON llm_calls(model);

-- Change events (detected changes to agents, models, tools, configs)
CREATE TABLE change_events (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    change_type         TEXT NOT NULL,  -- prompt_change, model_change, config_change,
                                        -- tool_change, provider_change, data_source_change
    entity_type         TEXT NOT NULL,  -- agent, tool, model, data_source
    entity_id           TEXT NOT NULL,
    field               TEXT,
    old_value_hash      TEXT,
    new_value_hash      TEXT,
    diff                JSONB,
    author              TEXT,           -- "sarah@company.com" or "system" or "proxy:runtime"
    source              TEXT,           -- "git:main:abc123", "proxy:runtime", "detector:fingerprint"
    downstream_agents   TEXT[],
    blast_radius        INT,
    correlated_metric_changes JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_change_events_entity ON change_events(entity_type, entity_id);
CREATE INDEX idx_change_events_timestamp ON change_events(timestamp DESC);
CREATE INDEX idx_change_events_type ON change_events(change_type);

-- Behavioral fingerprints (computed periodically per agent)
CREATE TABLE fingerprints (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id                TEXT NOT NULL,
    computed_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    window_start            TIMESTAMPTZ NOT NULL,
    window_end              TIMESTAMPTZ NOT NULL,
    sample_size             INT NOT NULL,
    -- Structural signals
    avg_tool_calls          FLOAT,
    tool_call_distribution  JSONB,
    avg_llm_calls           FLOAT,
    escalation_rate         FLOAT,
    error_rate              FLOAT,
    -- Volume signals
    avg_input_tokens        FLOAT,
    avg_output_tokens       FLOAT,
    avg_latency_ms          FLOAT,
    -- Distribution signals
    output_tokens_p50       FLOAT,
    output_tokens_p90       FLOAT,
    output_tokens_p99       FLOAT,
    latency_p50_ms          FLOAT,
    latency_p90_ms          FLOAT,
    latency_p99_ms          FLOAT,
    -- Quality proxies
    thumbs_up_rate          FLOAT,
    user_edit_rate          FLOAT,
    retry_rate              FLOAT
);

CREATE INDEX idx_fingerprints_agent ON fingerprints(agent_id, computed_at DESC);

-- Drift events (detected behavioral shifts)
CREATE TABLE drift_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        TEXT NOT NULL,
    detected_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    drift_started   TIMESTAMPTZ,
    shifted_signals JSONB,          -- what changed and by how much
    probable_causes JSONB,          -- correlated change events
    status          TEXT DEFAULT 'open',  -- open, acknowledged, resolved, expected
    resolved_at     TIMESTAMPTZ,
    resolution_note TEXT
);

CREATE INDEX idx_drift_events_agent ON drift_events(agent_id, detected_at DESC);
CREATE INDEX idx_drift_events_status ON drift_events(status);

-- Agent registry (auto-populated from LLM calls)
CREATE TABLE agents (
    id              TEXT PRIMARY KEY,  -- agent name from metadata
    first_seen      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    team            TEXT,
    call_count      BIGINT DEFAULT 0,
    models_used     TEXT[],
    tools_used      TEXT[],
    metadata        JSONB
);
