# Canopy: Change Intelligence for AI Agent Systems — Full Context Dump

## What This File Is

This is a complete context dump from a multi-session research and development effort
to build **Canopy** — a change intelligence platform for multi-AI agent systems.
Use this file to onboard a new session (e.g., integrating into CloudScope) with full
context of what was decided, why, and what code exists.

---

## 1. PRODUCT VISION

### One-Sentence Value Prop
"You changed something in your AI agent system and it broke something else — Canopy tells you what broke, why, and what's downstream."

### What Canopy Does
Canopy is a **change intelligence platform** for AI agent systems. It:
1. **Ingests** every LLM call via a lightweight proxy callback (LiteLLM)
2. **Builds a dependency graph** of agents → models → tools → other agents
3. **Computes behavioral fingerprints** (token distributions, latency, error rates) per agent
4. **Detects changes** (model swaps, prompt edits, config drift, tool removals, provider silent updates)
5. **Detects statistical drift** using Welch's t-test, z-test for proportions, Jensen-Shannon divergence
6. **Correlates root causes** — when drift happens, it finds which change caused it using graph distance + temporal proximity

### Why It Matters
In multi-agent AI systems, a single change (e.g., OpenAI silently updating gpt-4o-mini) can cascade
through the dependency graph and break downstream agents. Today, teams spend hours debugging these
failures. Canopy catches them automatically with zero AI — 100% deterministic statistical methods.

### Key Design Principles
- **Proxy-first**: Use LiteLLM callback, not custom SDKs. One line of code to integrate.
- **Infrastructure-aware**: Agents are K8s pods / ECS tasks. Reuse existing OTel distributed tracing
  for topology. Don't rebuild what infra already provides.
- **No AI judging AI**: ~80% of features are deterministic (statistical drift detection, format
  checking, entity tracking). No LLM used for diagnosis.
- **Topology-agnostic**: Core features work for both pipeline (A→B→C) and collaborative
  (A↔B↔C) agent patterns.
- **Graph-native**: The product IS a graph. Use graph DB from day 1 (Memgraph, with Neo4j migration path).

---

## 2. ARCHITECTURE

### Tech Stack
- **Backend**: Go
- **SDK/CLI**: Python
- **Storage**: Postgres (events, fingerprints) + Memgraph (dependency graph)
- **Protocol**: REST (MVP), gRPC ingestion planned for later
- **Infrastructure**: Docker Compose for local dev
- **Graph DB**: Memgraph (Cypher-compatible, in-memory, lightweight). Uses Neo4j Go driver
  which works with both Memgraph and Neo4j — migration path built in.

### Data Flow
```
LLM Call → LiteLLM Callback → POST /api/v1/ingest/llm → Canopy Server
    ├── Store raw call in Postgres (llm_calls table)
    ├── Upsert agent registry (agents table)
    ├── Update Memgraph graph (Agent→Model, Agent→Tool edges)
    └── Change detection pipeline
         ├── Compare against last known state (prompt hash, model, temperature)
         ├── If changed → Insert change_event
         ├── Compute blast radius via graph traversal
         └── Trigger fingerprint recomputation + drift detection
```

### Database Schema (Postgres)

**llm_calls**: Every LLM call through the proxy
- id, timestamp, agent_id, team, model, provider, system_prompt_hash
- temperature, max_tokens, input_tokens, output_tokens, cost_usd, latency_ms
- tool_calls (JSONB), tool_call_count, status, error_message, metadata (JSONB)

**agents**: Auto-populated agent registry
- id, first_seen, last_seen, team, call_count, models_used[], tools_used[]

**change_events**: Detected changes
- id, timestamp, change_type, entity_type, entity_id, field
- old_value_hash, new_value_hash, diff (JSONB), author, source
- downstream_agents[], blast_radius

**fingerprints**: Behavioral snapshots per agent per time window
- id, agent_id, computed_at, window_start, window_end, sample_size
- avg_tool_calls, tool_call_distribution, avg_llm_calls, escalation_rate, error_rate
- avg_input_tokens, avg_output_tokens, avg_latency_ms
- output_tokens_p50/p90/p99, latency_p50/p90/p99_ms
- thumbs_up_rate, user_edit_rate, retry_rate

**drift_events**: Detected behavioral shifts
- id, agent_id, detected_at, drift_started, shifted_signals (JSONB)
- probable_causes (JSONB), status, resolved_at, resolution_note

### Graph Schema (Memgraph/Neo4j)

Nodes:
- `:Agent {id, team, last_seen}`
- `:Model {name}`
- `:Tool {name}`

Edges:
- `(Agent)-[:USES_MODEL {last_seen, call_count}]->(Model)`
- `(Agent)-[:USES_TOOL {last_seen, call_count}]->(Tool)`
- `(Agent)-[:CALLS {last_seen, call_count}]->(Agent)`

### API Endpoints

**Ingestion:**
- `POST /api/v1/ingest/llm` — Ingest LLM call event
- `POST /api/v1/ingest/agent-call` — Record agent-to-agent communication

**Query:**
- `GET /api/v1/agents` — List all agents
- `GET /api/v1/agents/{id}` — Get agent details
- `GET /api/v1/changes?limit=N` — List recent changes
- `GET /api/v1/graph` — Full dependency graph (nodes + edges)
- `GET /api/v1/graph/downstream/{id}` — Downstream agents + blast radius
- `GET /api/v1/graph/upstream/{id}` — Upstream agents

**Health:**
- `GET /health`

---

## 3. CHANGE DETECTION

### What Changes We Track

**Type 1: Explicit Changes (Git-trackable)**
- System prompt edits
- Model version changes
- Temperature / max_tokens / top_p changes
- Tool additions / removals
- Agent code changes

**Type 2: Runtime Changes (Proxy-detectable)**
- Model string changes at runtime
- Config drift between environments
- New tools appearing in responses

**Type 3: Silent Provider Changes**
- OpenAI/Anthropic silently updating model weights
- Detected via "canonical prompt fingerprinting": run 30 fixed prompts
  through each model daily, compare output distributions
- Scoring: token_distribution_shift (JSD, weight 0.30) + response_time_change
  (weight 0.20) + format_consistency (weight 0.25) + semantic_similarity (weight 0.25)

**Type 4: Data Source Changes**
- RAG index updates, database schema changes, API response format changes

### Change Detection Logic
The detector maintains last-known state per agent (prompt hash, model, temperature).
On every incoming LLM call, it compares against last state. If different → emit change_event.
Blast radius is computed by traversing the graph downstream.

---

## 4. INTELLIGENCE ENGINE

### Behavioral Fingerprinting (`demo/fingerprint.py`)
Computes per-agent statistical profiles over time windows:
- Token distributions (mean, stdev, p50/p90/p99)
- Latency profiles (mean, stdev, p50/p90/p99)
- Error rates
- Tool usage distribution (proportion of calls using each tool)
- Cost aggregates

Minimum 5 calls per window. Stored as rolling windows (1h, 4h, 24h, 7d).

### Drift Detection (`demo/drift.py`)
Compares current fingerprint against baseline using:

1. **Welch's t-test** — For continuous signals (tokens, latency, cost)
   - "Is the current mean significantly different from baseline?"
   - Uses Welch-Satterthwaite approximation for degrees of freedom
   
2. **Z-test for proportions** — For rates (error_rate)
   - "Is the current proportion significantly different from baseline?"

3. **Jensen-Shannon divergence** — For distributions (tool usage)
   - "Has the distribution shape changed?"
   - JSD > 0.1 = notable, > 0.3 = significant

Significance thresholds:
- p < 0.05 → WARNING
- p < 0.01 → CRITICAL

Minimum change percentages to avoid noise:
- output_tokens: 10%, latency: 15%, error_rate: 50% relative

### Root Cause Correlation (`demo/correlate.py`)
When drift is detected, finds which change caused it:

**Scoring formula**: `score = temporal^0.30 * graph^0.35 * relevance^0.35`

1. **Temporal proximity** (30% weight)
   - Exponential decay: `score = exp(-hours_ago * ln(2) / 4)`
   - Half-life = 4 hours (change 4 hours ago scores 0.5)
   - Lookback window = 24 hours

2. **Graph distance** (35% weight)
   - Direct (same agent): 1.0
   - 1 hop: 0.7
   - 2 hops: 0.5
   - 3+ hops: 0.3
   - Unrelated: 0.1

3. **Change-drift relevance** (35% weight)
   - Relevance matrix maps (change_type, drift_metric) → score
   - E.g., model_change → output_tokens_mean = 0.95
   - E.g., tool_change → error_rate = 0.9
   - E.g., config_change → latency = 0.4

Output: Ranked list of probable causes with confidence scores and human-readable explanations.

---

## 5. DEMO & VALIDATION RESULTS

### Simulated System: 5-Agent E-Commerce Support
```
Router Agent (gpt-4o-mini) ──→ Support Agent (Claude Sonnet) ──→ Billing Agent (Claude Sonnet)
       │                              │
       │                              ↓
       │                       Escalation Agent (Claude Opus)
       ↓
Product Agent (gpt-4o-mini)
```

### 5 Failure Scenarios Injected

| # | Failure | Agent | Behavior Change |
|---|---------|-------|-----------------|
| 1 | Silent model upgrade | router-agent | gpt-4o-mini → gpt-4o-mini-2025-04-01, +20% tokens, +30% latency |
| 2 | Prompt guardrail removed | support-agent | "Never recommend competitors" deleted, +10% tokens |
| 3 | Model downgrade for cost | billing-agent | Sonnet → Haiku, -45% tokens, -65% latency, +3% errors |
| 4 | Tool removed (DB migration) | product-agent | check_inventory gone, 18% error rate |
| 5 | Temperature drift | escalation-agent | 0.2 → 0.9, +45% tokens, 2x variance |

### Detection Results: 9 drift signals across all 5 agents

| Agent | Drift Detected | Root Cause #1 | Score |
|-------|---------------|---------------|-------|
| router-agent | tokens +21%, latency +25% | Own model changed | 0.89 |
| billing-agent | tokens -45%, latency -67% | Own model changed | 0.90 |
| product-agent | Tool distribution shifted (JSD=0.19) | Own tool removed | 0.91 |
| escalation-agent | tokens +48% | Temperature drift | 0.82 |
| support-agent | tokens +12% | Prompt changed | 0.84 |

### Validation Suite: 8/8 tests passed
Server health, LLM call ingestion, agent registration, graph population,
model change detection, prompt change detection, agent call graph, blast radius.

---

## 6. CODE LOCATION

All code is in `mukeshappcel2026-cpu/notes-app` on branch `claude/ai-agent-startup-ideas-vZ1f2`.

```
canopy/
├── docker-compose.yml              # Postgres + Memgraph + server
├── Makefile                        # up, down, build, test, reset
├── migrations/
│   └── 001_initial_schema.sql      # Full Postgres schema
├── backend/                        # Go backend
│   ├── Dockerfile
│   ├── go.mod
│   ├── cmd/server/
│   │   ├── main.go                 # Server entrypoint
│   │   └── config.go               # Env-based config
│   └── internal/
│       ├── api/router.go           # REST API handlers
│       ├── storage/postgres.go     # Postgres operations
│       ├── graph/memgraph.go       # Memgraph/Neo4j graph operations
│       ├── ingestion/service.go    # Ingestion pipeline
│       └── changes/detector.go     # Change detection
├── sdk/python/
│   ├── pyproject.toml
│   └── canopy/
│       ├── __init__.py
│       ├── callback.py             # LiteLLM callback (ONE LINE integration)
│       ├── events.py               # Event emitter for agent-to-agent calls
│       └── cli.py                  # CLI: discover, status, changes, graph
└── demo/
    ├── fingerprint.py              # Behavioral fingerprinting engine
    ├── drift.py                    # Statistical drift detection
    ├── correlate.py                # Root cause correlation engine
    ├── inmemory_server.py          # In-memory server for testing
    ├── simulate.py                 # Multi-agent simulation
    ├── run_full_pipeline.py        # Full pipeline demo (self-contained)
    ├── validate_detection.py       # 8-test validation suite
    └── README.md
```

---

## 7. INTEGRATION INTO CLOUDSCOPE

### What CloudScope Already Has (Assumed)
- Infrastructure graph (K8s, cloud resources, services)
- Change tracking for infrastructure (deploys, config changes, scaling events)
- Graph database (likely Neo4j or similar)
- Platform engineer users who already think in graphs and dependencies

### What Canopy Adds to CloudScope
**A new "AI Agent" layer on top of the existing infrastructure graph.**

CloudScope today: `K8s Pod → Service → Database → Cloud Resource`
CloudScope + Canopy: `AI Agent → Model → Tool → K8s Pod → Service → Database`

The AI agent layer connects to the infra layer because agents ARE pods/services.
When a K8s deploy changes an agent's pod, CloudScope already tracks the infra change.
Canopy adds the semantic layer: "that deploy changed the agent's model from Sonnet to Haiku."

### Integration Points

1. **Graph extension**: Add Agent, Model, Tool node types and USES_MODEL, USES_TOOL,
   CALLS edge types to CloudScope's existing graph schema.

2. **Ingestion endpoint**: Add `/api/v1/ingest/llm` to CloudScope's API. The LiteLLM
   callback points here.

3. **Change correlation**: Connect Canopy's change_events to CloudScope's existing
   infrastructure change events. A K8s deploy that changes an agent's config should
   appear in the same change timeline.

4. **Fingerprinting job**: Background worker that computes behavioral fingerprints
   from the llm_calls table every 15 minutes.

5. **Drift detection**: Runs after fingerprinting. Compares current vs baseline.
   Emits drift_events.

6. **Correlation engine**: When drift detected, searches both AI change events AND
   infra change events for root causes. This is the killer feature — "your agent
   broke because of a K8s config change 2 hours ago."

### Migration Strategy
- The Go backend code (`backend/internal/`) can be imported directly into CloudScope's Go codebase
- The Postgres schema (`migrations/001_initial_schema.sql`) adds tables alongside CloudScope's existing schema
- The Memgraph graph operations use the Neo4j Go driver — if CloudScope uses Neo4j, it works as-is
- The Python SDK is a separate pip package, independent of CloudScope's backend
- The intelligence engine (`demo/fingerprint.py`, `demo/drift.py`, `demo/correlate.py`) needs
  to be ported to Go to live in CloudScope's backend, OR kept as a Python microservice

---

## 8. DECISIONS MADE AND WHY

| Decision | Choice | Why |
|----------|--------|-----|
| Proxy vs SDK | LiteLLM callback | One line integration, captures ~70% of signals, no custom SDK adoption barrier |
| Graph DB | Memgraph (Neo4j migration path) | Product IS a graph. Cypher-compatible, in-memory, lightweight. Neo4j driver works with both. |
| AI for detection | No — deterministic stats | Welch's t-test, z-test, JSD. Auditable. No "AI checking AI" paradox. |
| Topology approach | Reuse K8s/OTel | Agents are pods. Don't rebuild infra observability — enrich it with AI semantics. |
| API style | REST for MVP | Simple. GraphQL adds complexity for MVP. Add later for variable-depth graph queries. |
| Message queue | Skip for MVP | Direct Postgres writes. Add NATS when throughput demands it. |
| Customization | Not in MVP | Built-in tests only. YAML config layer in Phase 2. Python custom tests in Phase 3. |

---

## 9. COMPETITIVE LANDSCAPE

### Why Not Datadog?
Datadog WILL build AI observability, but their 18-24 month window gives us time.
Canopy's moat: deep multi-agent topology understanding and change correlation.
Datadog will add AI metrics to existing APM. We build agent-native intelligence.

### Unsolved Problems We Address (from SRE-for-AI gap analysis)
- 18 of 37 mapped SRE-for-AI problems are unsolved
- Key gaps: agent-to-agent contract testing, behavioral drift detection,
  change attribution across agent dependencies, provider silent update detection

### Why This Is Not Transient
- AI agents are infrastructure now, not experiments
- Multi-agent systems are the default architecture for complex AI tasks
- Every agent system will need change intelligence the way every service needs APM
- The pipeline → collaborative agent transition makes this MORE needed, not less

---

## 10. WHAT'S NOT BUILT YET

1. Go backend doesn't compile (needs go mod tidy with actual dependencies)
2. No dashboard/UI
3. Fingerprinting + drift + correlation only in Python demo, not in Go server
4. No canonical prompt fingerprinting (silent provider update detection)
5. No OTel integration (AI semantic attributes enriching existing traces)
6. No CI/CD gate (pre-deploy eval)
7. No YAML/Python custom test framework
8. No alerting/notification system
