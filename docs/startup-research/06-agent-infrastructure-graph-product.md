# Agent Infrastructure Graph: Product Deep Dive (Canopy Model)

## Core Concept

Auto-discover AI agent infrastructure and build a living dependency graph — the same way Canopy does for cloud infrastructure.

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    AGENT INFRASTRUCTURE GRAPH                    │
│                                                                  │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐  │
│  │ Discovery│───▶│  Graph   │───▶│  Change  │───▶│  Alert   │  │
│  │  Engine  │    │  Engine  │    │  Tracker │    │  Engine  │  │
│  └──────────┘    └──────────┘    └──────────┘    └──────────┘  │
│       │               │               │               │         │
│  Scans code,     Builds          Diffs graph      Notifies on   │
│  configs,        topology        state over       drift, breaks,│
│  runtime traces  + dependencies  time             blast radius  │
└─────────────────────────────────────────────────────────────────┘
```

### Layer 1: Discovery Engine

```
STATIC DISCOVERY (code/config scanning)
├── Agent definitions (LangChain, CrewAI, AutoGen, custom)
├── Tool registrations (MCP servers, function tools, API integrations)
├── Prompt templates + versions
├── Model configurations (which model, which params)
├── Data source connections (vector DBs, SQL, APIs)
└── Permission configs (API keys, OAuth scopes, access policies)

RUNTIME DISCOVERY (trace ingestion)
├── Actual agent-to-agent calls observed in production
├── Tool usage patterns (which tools actually get called, how often)
├── Data access patterns (what data sources are hit, what's read/written)
├── Model call patterns (token volumes, latency distributions)
└── Error patterns (which connections fail, how often)
```

### Layer 2: Graph Engine

```
NODE TYPES:
├── Agent          (name, version, prompt hash, model, owner)
├── Tool           (name, type, endpoint, permissions required)
├── Model          (provider, model ID, version, parameters)
├── Data Source    (type, location, schema, sensitivity level)
├── Human Gate     (approval workflow, approver, SLA)
└── External API   (endpoint, auth method, rate limits)

EDGE TYPES:
├── CALLS          (Agent → Agent, frequency, avg latency)
├── USES_TOOL      (Agent → Tool, frequency, error rate)
├── INVOKES_MODEL  (Agent → Model, tokens/day, cost/day)
├── READS_FROM     (Agent → Data Source, volume, sensitivity)
├── WRITES_TO      (Agent → Data Source, volume, sensitivity)
├── REQUIRES_APPROVAL (Agent → Human Gate, approval rate)
└── DEPENDS_ON     (any → any, criticality score)
```

### Layer 3: Change Tracker

Every mutation to the graph is versioned with attribution, blast radius, and impact assessment.

### Layer 4: Alert Engine

Declarative policies for quality regression, cost spikes, unauthorized data access, new dependencies, and single points of failure.

---

## Competitive Landscape

### Direct Competitors (LLM/Agent Observability)

| Company | Funding | Agent Graph? | Change Tracking? | Focus |
|---|---|---|---|---|
| **LangSmith** (LangChain) | ~$50M | No | Prompt versioning only | LangChain ecosystem tracing |
| **Langfuse** | ~$4M seed | No | Basic prompt mgmt | Open-source, framework-agnostic tracing |
| **Arize AI** | ~$62M Series B | No | Drift detection on models | Most mature, ML + LLM coverage |
| **Braintrust** | ~$36M (a16z) | No | Prompt versioning | Eval-first, CI/CD integration |
| **Helicone** | ~$5M seed | No | No | Dead-simple proxy, single LLM call level |
| **AgentOps** | ~$2-3M seed | No | No | Purpose-built for agents, session replay |
| **Portkey** | ~$3M seed | No | No | AI gateway + routing |
| **W&B Weave** | $250M+ Series C | No | Experiment tracking | LLM obs is secondary to MLOps |
| **Galileo** | ~$23M Series A | No | No | Hallucination detection, eval |
| **Patronus AI** | ~$17M Series A | No | No | Safety/eval/red-teaming |

### The Critical Gap

**Nobody is building the infrastructure graph.** Everyone observes individual LLM calls or agent runs. Nobody maps the system of agents as interconnected infrastructure.

This is like monitoring individual HTTP requests without understanding the service topology.

---

## Product Differentiation

### What Everyone Else Does (Trace-Level)
```
"Show me what happened in this one agent run"
```

### What This Startup Does (System-Level)
```
"Show me my entire agent infrastructure, how it's connected,
 what changed, and what's at risk"
   └── 47 agents → 23 tools → 8 data sources → 4 models
       12 agent-to-agent dependencies
       3 changes in last 24 hours
       1 single point of failure
       2 agents drifting from baseline
```

---

## Technical Integration

### Data Ingestion
1. **SDK instrumentation** — Python, TypeScript SDKs wrapping LangChain, CrewAI, custom
2. **Config scanning** — Git repo scanning, CI/CD integration, MCP server registry
3. **Provider integrations** — OpenAI/Anthropic usage APIs, vector DB metrics, cloud APIs

### Storage
- Graph DB (Neo4j/Memgraph) for topology and dependencies
- Time-series DB (ClickHouse) for traces, metrics, costs, quality scores

---

## Go-to-Market

### Phase 1: Open-Source Graph SDK (Months 0-6)
- CLI tool: `agentmap scan` → outputs agent infrastructure as a graph
- Developers try it, see their topology for the first time
- Goal: community adoption

### Phase 2: Hosted Platform (Months 3-12)
- Cloud UI, change tracking, alerts
- Free tier (< 10 agents), paid ($500-2,000/month)
- Goal: 50 paying teams

### Phase 3: Enterprise (Months 9-18)
- RBAC, SSO, compliance, self-hosted
- Blast radius analysis, approval workflows
- Integration with Datadog, Grafana, PagerDuty
- Goal: $50K-200K ACV enterprise contracts

---

## Revenue Model

| Tier | Price | Includes |
|---|---|---|
| Open Source | Free | Local graph, CLI, basic SDK |
| Team | $500/month | Hosted graph, 20 agents, change tracking, 30-day history |
| Business | $2,000/month | 100 agents, alerts, blast radius, 90-day history |
| Enterprise | $50K-200K/year | Unlimited, SSO, RBAC, compliance, self-hosted |

### Revenue Trajectory

| Year | ARR Target |
|---|---|
| Year 1 | $500K |
| Year 2 | $3M |
| Year 3 | $12M |

---

## The Pitch

> "You have 50 AI agents in production. Can you draw the dependency graph? Can you tell me what changed yesterday? Can you tell me the blast radius if Anthropic has an outage? No? That's what we do. We're Canopy for AI agent infrastructure."
