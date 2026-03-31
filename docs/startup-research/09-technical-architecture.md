# Change Intelligence: Technical Architecture & Build Phases (Detailed)

## Architecture Decision: Proxy-First, Not SDK-First

### Why Proxy-First

Instead of building framework-specific SDKs (LangChain, CrewAI, etc.), integrate with LiteLLM proxy to capture ~70% of signals with zero instrumentation code.

```
Agent Code → LiteLLM Proxy → LLM Provider
                  ↓
            ChangeInt Backend (captures 70% of signals)
                  ↑
            Lightweight event emitter (captures remaining 30%)
```

### What Proxy Gives You Free

| Signal | Via Proxy? |
|---|---|
| Every LLM call (prompt + response) | Yes |
| Model used per call | Yes |
| Token counts (input/output) | Yes |
| Latency per LLM call | Yes |
| Cost per call | Yes (LiteLLM computes this) |
| Error rates | Yes |
| Tool calls | Yes (visible in tool_use messages) |
| Agent-to-agent calls | No — needs lightweight event emitter |
| Data source access | No — needs lightweight event emitter |
| User feedback | No — needs lightweight event emitter |

### Three Integration Layers

**Layer 1: LiteLLM Callback (zero-code, immediate)**

```python
import litellm

class ChangeIntCallback(litellm.Callback):
    def log_success_event(self, kwargs, response_obj, start_time, end_time):
        event = {
            "timestamp": start_time.isoformat(),
            "agent": kwargs.get("metadata", {}).get("agent", "unknown"),
            "model": kwargs.get("model"),
            "system_prompt_hash": hash(extract_system_prompt(kwargs["messages"])),
            "temperature": kwargs.get("temperature"),
            "tool_calls": extract_tool_calls(response_obj),
            "input_tokens": response_obj.usage.prompt_tokens,
            "output_tokens": response_obj.usage.completion_tokens,
            "latency_ms": (end_time - start_time).total_seconds() * 1000,
            "cost": kwargs.get("response_cost", 0),
            "status": "success"
        }
        self.send_event(event)

litellm.callbacks = [ChangeIntCallback()]
```

**Layer 2: Metadata Convention (one line per agent)**

```python
response = litellm.completion(
    model="claude-sonnet-4-6",
    messages=[...],
    metadata={"agent": "customer-support-agent", "team": "support"}
)
```

**Layer 3: Event Emitter (optional, for the 30% proxy can't see)**

```python
from changeint import events

events.emit("agent_call", {"from": "support-agent", "to": "billing-agent"})
events.emit("user_feedback", {"agent": "support-agent", "type": "thumbs_up"})
events.emit("data_access", {"agent": "support-agent", "source": "customers_db"})
```

---

## Tech Stack

| Component | Choice | Why |
|---|---|---|
| Language | Go (services), Python (callback/emitter) | Go for perf; Python for LiteLLM ecosystem |
| Graph DB | Memgraph (start), Neo4j (scale) | Memgraph: in-memory, Cypher-compatible |
| Time-Series | TimescaleDB (start), ClickHouse (scale) | TimescaleDB = Postgres extension |
| Event Store | Postgres + JSONB + event sourcing | Simple, reliable |
| Message Queue | NATS (start), Kafka (scale) | NATS: lightweight |
| API | GraphQL (queries), gRPC (ingestion), REST (webhooks) | GraphQL for variable-depth graph queries |
| Web UI | React + D3.js (graph) + Tremor (dashboards) | D3 for dependency graph viz |
| CI/CD | GitHub Actions first | Largest market |

---

## Data Models

### Change Events (Postgres)

```sql
CREATE TABLE change_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    change_type     TEXT NOT NULL,  -- prompt_change, config_change, provider_change,
                                   -- data_source_change, tool_change
    entity_type     TEXT NOT NULL,  -- agent, tool, model, data_source
    entity_id       TEXT NOT NULL,
    field           TEXT,
    old_value_hash  TEXT,
    new_value_hash  TEXT,
    diff            JSONB,
    author          TEXT,           -- "sarah@company.com" or "system"
    source          TEXT,           -- "git:main:abc123", "proxy:runtime", "detector:fingerprint"
    downstream_agents TEXT[],
    blast_radius    INT,
    correlated_metric_changes JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_change_events_entity ON change_events(entity_type, entity_id);
CREATE INDEX idx_change_events_timestamp ON change_events(timestamp DESC);
```

### Behavioral Fingerprints (TimescaleDB)

```sql
CREATE TABLE fingerprints (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        TEXT NOT NULL,
    computed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    window_start    TIMESTAMPTZ NOT NULL,
    window_end      TIMESTAMPTZ NOT NULL,
    sample_size     INT NOT NULL,
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
```

### Drift Events

```sql
CREATE TABLE drift_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        TEXT NOT NULL,
    detected_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    drift_started   TIMESTAMPTZ,
    shifted_signals JSONB,       -- what changed and by how much
    probable_causes JSONB,       -- correlated change events
    status          TEXT DEFAULT 'open',  -- open, acknowledged, resolved, expected
    resolved_at     TIMESTAMPTZ,
    resolution_note TEXT
);
```

---

## Change Tracking: How Each Source Is Detected

### Prompt Changes
- **Detection:** Git webhook on PR/commit + proxy sees system_prompt_hash change
- **Event:** diff, old/new hash, author, affected agents

### Config Changes (model, temperature, tools)
- **Detection:** Proxy sees parameter changes in LLM API calls
- **Event:** field, old value, new value, agent

### Model Provider Updates
- **Detection:** Behavioral fingerprinting — run fixed canonical prompts daily, detect output/embedding similarity shifts
- **Event:** model, confidence score, evidence (similarity drop, distribution shift)

### Data Source Changes
- **Detection:** Content hash on vector DB collections + event emitter hook on indexing pipeline
- **Event:** documents affected, semantic drift score, affected agents

---

## Drift Detection: Statistical Methods

All deterministic — no AI checking AI.

### For continuous metrics (latency, tokens)
**Method:** Welch's t-test
**Compare:** Last 6 hours vs 7-day baseline
**Threshold:** p-value < 0.01 AND effect size > 15%

### For distributions (tool call counts, response lengths)
**Method:** Jensen-Shannon divergence
**Threshold:** JS divergence > 0.10

### For rates (error rate, thumbs-up rate)
**Method:** Proportions z-test
**Threshold:** p < 0.01

### Root Cause Correlation
When drift detected:
1. Query change events in 48-hour window before drift
2. Check: direct changes to this agent (confidence: 0.9)
3. Check: upstream dependency changes (confidence: 0.7)
4. Check: model provider fingerprint shifts (confidence: 0.6)
5. Check: data source changes (confidence: 0.65)
6. If nothing found: flag as "unexplained drift"

---

## Build Phases (16-Week Plan)

### Phase 0: Foundation (Week 1-2)

**Goal:** Ingest all LLM traffic via proxy, store structured events.

**Deliverables:**
- LiteLLM callback that captures all LLM calls
- Event ingestion API (gRPC endpoint)
- Postgres schema for change events + raw LLM call logs
- Basic agent registry (auto-populated from metadata in LLM calls)
- Docker Compose for local development

**What works after Phase 0:**
```
$ docker compose up
$ # Configure LiteLLM callback
$ # Make some agent calls
$ curl localhost:8080/api/agents
  → Shows all agents that have made LLM calls, with call counts
```

### Phase 1: Discovery & Graph (Week 3-4)

**Goal:** Build dependency graph from observed LLM traffic patterns.

**Deliverables:**
- Graph construction from runtime data:
  - Agent → Model edges (from LLM calls)
  - Agent → Tool edges (from tool_use in responses)
  - Agent → Agent edges (from event emitter or trace context)
- Memgraph integration for graph storage
- CLI tool: `changeint discover`
- Basic graph query API (GraphQL)

**What works after Phase 1:**
```
$ changeint discover
  Discovered: 8 agents, 12 tools, 3 models
  47 dependency edges mapped

  ⚠ Findings:
    - billing-agent has single model dependency (no fallback)
    - 3 agents share same API key
    - research-agent → analyst-agent → research-agent (circular)
```

### Phase 2: Change Detection & Tracking (Week 5-8)

**Goal:** Detect and record every change to the agent system.

**Week 5-6 deliverables:**
- GitHub webhook integration (detect prompt/config file changes)
- Proxy-based change detection:
  - System prompt hash changes between calls
  - Model parameter changes (temperature, model name, etc.)
  - Tool list changes
- Change event creation with attribution (who, what, when)
- Link changes to affected agents via graph

**Week 7-8 deliverables:**
- Change timeline API
- Web UI: chronological change timeline with filtering
- Blast radius calculation (graph traversal from changed entity)
- Change correlation: when metrics shift, show recent changes

**What works after Phase 2:**
```
Change Timeline UI shows:
  Mar 31 14:23  PROMPT  @sarah  customer-support-agent
                Changed system prompt (+2 lines, -1 line)
                Blast radius: 3 downstream agents

  Mar 31 10:15  CONFIG  @mike   research-agent
                Model: sonnet → haiku
                Cost impact: -62% estimated
```

### Phase 3: Drift Detection (Week 9-12)

**Week 9-10 deliverables:**
- Fingerprint computation pipeline (hourly cron job)
- Statistical comparison engine:
  - Welch's t-test for continuous metrics
  - Jensen-Shannon divergence for distributions
  - Proportions z-test for rates
- Drift event creation when thresholds exceeded

**Week 11-12 deliverables:**
- Root cause correlation engine
- Drift alert UI + Slack integration
- Provider model change detection (daily canonical prompt runner)
- Alert format with shifted signals, probable causes, suggested actions

**What works after Phase 3:**
```
DRIFT ALERT: customer-support-agent
  Escalation rate: 12% → 19% (p<0.001)
  Probable cause: claude-sonnet-4-6 behavioral shift
  detected at 05:45 UTC
  [View Timeline] [Run Evals] [Compare Outputs]
```

### Phase 4: Pre-Deploy Eval Gate (Week 13-14)

**Deliverables:**
- GitHub Actions integration
- Pre-deploy checks (triggered on PR/commit):
  - Prompt diff analysis
  - Blast radius report as PR comment
  - Format regression tests (does output schema still match?)
  - Downstream integration tests (do dependent agents still work?)
- Deploy gate: pass/warn/block with justification override

**What works after Phase 4:**
```
GitHub PR Comment:
  ChangeInt Analysis: customer-support-agent prompt change

  Blast radius: 3 downstream agents
  Regression tests: 48/50 passed, 2 warnings
  Format stability: ✓ passed

  ⚠ Review recommended before merge
  [View Details]
```

### Phase 5: Polish & Launch (Week 15-16)

**Deliverables:**
- Open-source CLI + LiteLLM callback (Apache 2.0)
- Documentation site
- Landing page
- Self-hosted Docker Compose distribution
- Cloud hosted beta (for early customers)
- Launch: Hacker News, Product Hunt, AI/ML communities

---

## What You DON'T Build (Proxy-First Savings)

| Originally Planned | Status | Why |
|---|---|---|
| Python SDK with decorators | Don't build | Proxy captures LLM calls automatically |
| LangChain instrumentor | Don't build | Framework-agnostic via proxy |
| CrewAI instrumentor | Don't build | Framework-agnostic via proxy |
| Token counting logic | Don't build | LiteLLM computes this |
| Cost calculation | Don't build | LiteLLM computes this |
| Latency measurement | Don't build | Measured at proxy layer |

## What You DO Build

| Component | Complexity | Week |
|---|---|---|
| LiteLLM callback + event parser | Low | 1 |
| Lightweight event emitter | Low | 2 |
| Graph construction from runtime data | Medium | 3-4 |
| Git webhook + change detection | Medium | 5-6 |
| Change timeline UI | Medium | 7-8 |
| Fingerprint computation pipeline | Medium | 9-10 |
| Statistical drift detection | Medium | 11-12 |
| CI/CD eval gate | Medium | 13-14 |
| Polish + launch | Medium | 15-16 |

---

## Post-Launch Roadmap

**Month 5-6:** Compliance expansion (EU AI Act + SOC2 evidence generation from change/audit data)
**Month 7-8:** Additional proxy support (Portkey, Helicone, direct OpenAI/Anthropic SDK hooks)
**Month 9-10:** Advanced graph features (risk scoring, SPOF detection, capacity planning)
**Month 11-12:** Enterprise features (SSO, RBAC, self-hosted, SLA)
