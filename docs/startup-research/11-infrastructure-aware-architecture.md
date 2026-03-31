# Infrastructure-Aware Architecture: Leveraging Existing Observability

## Key Insight

Agents are just apps deployed on K8s pods or ECS tasks. Infrastructure observability (Canopy, Datadog, service mesh) already maps inter-service communication. We don't need to build agent-to-agent discovery — we need to enrich existing infrastructure data with AI semantics.

---

## Two-Layer Architecture

### Layer 1: Infrastructure Graph (Already Exists — Don't Build)

Source: K8s API, network traffic, OTel traces, service mesh

```
  ┌────────────┐     ┌────────────┐     ┌────────────┐
  │ support-   │────▶│ billing-   │────▶│ payments   │
  │ agent      │     │ agent      │     │ service    │
  │ (pod)      │     │ (pod)      │     │ (pod)      │
  └─────┬──────┘     └────────────┘     └────────────┘
        │
        ├────▶ api.anthropic.com (Claude API)
        ├────▶ pinecone.io (Vector DB)
        └────▶ postgres:5432 (Database)
```

Gives you: WHO talks to WHO, HOW OFTEN, HOW FAST.

### Layer 2: AI Semantic Layer (What We Build)

Source: OTel collector enrichment + LiteLLM proxy callback

Enriches infrastructure spans with AI-specific context:
- Prompt hash, model name, token counts, tool calls, cost, quality signals

---

## What We Get For Free From Infrastructure

| Signal | Source | Already Exists? |
|---|---|---|
| Agent A calls Agent B | Network traffic / OTel traces | Yes |
| Call frequency between agents | Request count between services | Yes |
| Latency between agents | Span duration in distributed trace | Yes |
| Error rate between agents | HTTP status codes | Yes |
| Which agents exist | K8s API / ECS API / deployment manifests | Yes |
| Agent-to-tool calls (if tool is a service) | HTTP call to another service | Yes |
| Agent-to-LLM-provider calls | Outbound HTTP to api.anthropic.com | Yes |

## What We DON'T Get (What We Build)

| Signal | Why Infra Can't See It |
|---|---|
| What prompt was sent | Application-level payload |
| What model was used | Inside the HTTP body |
| Token counts | Inside the API response |
| Tool call decisions | Agent-internal logic |
| Output quality | Semantic judgment |
| System prompt hash | Application-level content |

---

## OTel Enrichment: Our Core Integration

### Standard OTel Trace (infra gives you this)

```json
{
  "trace_id": "abc123",
  "span_name": "POST /api/chat",
  "service": "support-agent",
  "duration_ms": 2340,
  "status": 200,
  "downstream_spans": [
    {"service": "billing-agent", "duration_ms": 890},
    {"service": "api.anthropic.com", "duration_ms": 1200}
  ]
}
```

### AI-Enriched OTel Trace (our collector adds this)

```json
{
  "trace_id": "abc123",
  "span_name": "POST /api/chat",
  "service": "support-agent",
  "duration_ms": 2340,
  "status": 200,

  "ai.agent.name": "support-agent",
  "ai.prompt.hash": "7b2e91",
  "ai.model": "claude-sonnet-4-6",
  "ai.tokens.input": 1200,
  "ai.tokens.output": 680,
  "ai.tokens.cost_usd": 0.0034,
  "ai.tools.called": ["search", "database"],
  "ai.tools.count": 2,

  "downstream_spans": [
    {
      "service": "billing-agent",
      "ai.agent.name": "billing-agent",
      "ai.model": "claude-haiku-4-5",
      "ai.tokens.input": 800
    }
  ]
}
```

### AI Span Attribute Convention

```
ai.agent.name          # agent identifier
ai.agent.version       # agent version/hash
ai.prompt.hash         # hash of system prompt
ai.prompt.version      # if versioned in git
ai.model.name          # claude-sonnet-4-6
ai.model.provider      # anthropic
ai.model.temperature   # 0.7
ai.tokens.input        # 1200
ai.tokens.output       # 680
ai.tokens.cost_usd     # 0.0034
ai.tools.names         # ["search", "database"]
ai.tools.count         # 2
ai.quality.score       # 91 (if available)
ai.feedback.thumbs_up  # true/false (if available)
```

---

## End-to-End Example: How a Change Becomes Intelligence

### Step 1: Change Happens
Sarah edits prompts/support-agent.txt and deploys.

### Step 2: Two Detection Paths Fire

**Path A — Git Webhook:** Knows WHO changed it, the diff, the commit.

**Path B — Proxy + OTel:** Sees system_prompt_hash changed from a3f8 → 7b2e in enriched spans.

### Step 3: Change Event Processor Correlates
Merges git attribution + proxy detection into one enriched change event.

### Step 4: Graph Enrichment (from existing infra)
Queries K8s/infra topology: "What depends on support-agent?"
Returns: billing-agent (430 calls/day), escalation-agent (180 calls/day)
Calculates blast radius.

### Step 5: Metrics Flow (from enriched OTel spans)
Post-change spans show: tokens +25%, tool calls +22%

### Step 6: Drift Detection
Hourly fingerprint comparison detects statistical shift.

### Step 7: Root Cause Correlation
Drift correlates with change event (37 min prior, same agent, high confidence).
Also detects downstream impact: escalation-agent escalation rate +15%.

### Step 8: Intelligence Output
```
CHANGE: support-agent prompt modified by @sarah at 14:23

IMPACT:
  support-agent: tokens +25%, tool calls +22%
  escalation-agent (downstream): escalation rate +15%
  billing-agent (downstream): no impact ✓

COST: +$12/day estimated
ACTION: Review escalation rate increase — likely unintended
```

---

## Revised Architecture

```
┌──────────────────────────────────────────────────────────┐
│ EXISTING INFRASTRUCTURE (don't build)                     │
│                                                           │
│  K8s API ──▶ Service topology                            │
│  OTel traces ──▶ Request paths, latency, errors          │
│  Service mesh ──▶ Inter-service communication            │
│                                                           │
│  = THE GRAPH (free)                                      │
└─────────────────────────┬────────────────────────────────┘
                          ▼
┌──────────────────────────────────────────────────────────┐
│ OUR LAYER (build this)                                    │
│                                                           │
│  OTel Processor ──▶ Enriches spans with AI attributes    │
│  LiteLLM Callback ──▶ Prompt/model/token details         │
│  Git Webhooks ──▶ Change attribution                      │
│  Canonical Prompts ──▶ Provider update detection          │
│                                                           │
│  = THE AI CONTEXT                                        │
└─────────────────────────┬────────────────────────────────┘
                          ▼
┌──────────────────────────────────────────────────────────┐
│ INTELLIGENCE ENGINE (our core IP)                         │
│                                                           │
│  Change Detection ──▶ What changed in the AI layer?      │
│  Drift Detection ──▶ Is behavior shifting?               │
│  Correlation ──▶ Infra graph + AI context + changes      │
│                  = root cause intelligence                │
│                                                           │
│  = THE INTELLIGENCE                                      │
└──────────────────────────────────────────────────────────┘
```

## What We Build vs Reuse

| Component | Build or Reuse? |
|---|---|
| Agent discovery | **Reuse** — K8s/ECS API |
| Agent-to-agent mapping | **Reuse** — OTel traces, service mesh |
| Call frequency/latency | **Reuse** — OTel spans |
| Agent-to-provider mapping | **Reuse** — outbound traffic |
| OTel AI enrichment processor | **Build** — our core integration |
| LiteLLM callback | **Build** — captures AI semantics |
| Change detection logic | **Build** — prompt/model/config comparison |
| Provider update fingerprinting | **Build** — canonical prompts |
| Drift detection | **Build** — statistical methods |
| Correlation engine | **Build** — connects everything |

## Our IP Is Narrow But Deep

1. OTel processor adding AI semantic attributes to existing spans
2. Change detection (prompt hash, model, config comparison over time)
3. Provider update fingerprinting (canonical prompts)
4. Statistical drift detection
5. Correlation engine (infra graph + AI context + change history → root cause)

Everything else — graph, topology, tracing, metrics — already exists.

## Revised Build Timeline: ~10-12 Weeks

| Week | What |
|---|---|
| 1-2 | OTel processor + LiteLLM callback (AI enrichment layer) |
| 3-4 | Change event store + git webhook integration |
| 5-6 | Infra graph integration (consume K8s/OTel topology) + blast radius |
| 7-8 | Drift detection (statistical fingerprinting + root cause correlation) |
| 9-10 | Web UI (change timeline, enriched graph view, drift alerts) |
| 11-12 | CI/CD gate + provider fingerprinting + polish + launch |
