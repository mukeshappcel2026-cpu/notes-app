# Change Intelligence for AI Systems: Product Specification

## Core Problem

In AI agent systems, changes come from 5 directions — most of them invisible:

1. **Your changes** — prompt edits, tool additions, config changes, code changes
2. **Provider changes** — model updates (silent), API behavior changes, deprecations
3. **Data changes** — vector DB refreshed, knowledge base updated, schema changes
4. **Dependency changes** — tool APIs changed format, another agent modified, infra changes
5. **Usage changes** — different query patterns, volume spikes, adversarial inputs

When quality drops, teams spend hours/days investigating manually. Nobody tracks these systematically.

---

## The Workflow: Four Phases

### Phase 1: Before Change — Blast Radius Analysis

"What will this affect?"

Engineer changes a prompt. Product shows:
- All downstream agents that depend on this agent
- Data sources accessed (including PII sensitivity)
- Historical precedent (similar past changes and their impact)
- Risk score based on dependency count, data sensitivity, past incidents
- Recommendation (run evals, monitor downstream agents post-deploy)

**Technical approach:** Dependency graph built from static analysis (configs, code) + runtime observation (actual call patterns). Pure deterministic engineering.

### Phase 2: During Change — Pre-Deploy Eval Gate

"Is it safe to go live?"

CI/CD integration. When a change is detected:
1. Identify change type (prompt, config, tool, model)
2. Select eval suite based on change type + affected dependencies
3. Run evals in parallel against staging
4. Gate decision: pass/block/warn with specific regressions identified

**What's tested (all deterministic/feasible):**
- Did output format change? (schema validation)
- Do downstream agents still work with the new output? (integration test)
- Does PII leak? (regex + NER)
- Did behavior regress on known test cases? (diff from baseline)

### Phase 3: After Change — Change Attribution

"Did anything break?"

Post-deploy monitoring comparing against pre-change baseline:
- Latency, error rate, token usage changes (metrics — deterministic)
- Downstream agent impact (error rates, format parse rates — deterministic)
- User feedback signal changes (thumbs up/down — direct measurement)
- Auto-rollback if thresholds exceeded
- Surface data for human decision-making

### Phase 4: Ongoing — Drift Detection

"Is anything drifting without anyone changing anything?"

Continuous monitoring of behavioral fingerprints:
- Distribution of token counts, latency, tool call patterns
- Format consistency, error rate trends
- User feedback trends
- When distribution shifts, correlate with known change events (model updates, data refreshes)
- Suggest root cause

**All statistical measurements on deterministic data. Not AI judging AI.**

---

## Feasibility Assessment

| Capability | Deterministic? | Feasibility |
|---|---|---|
| Dependency graph construction | Yes — count call patterns | **High** |
| Blast radius calculation | Yes — graph traversal | **High** |
| Change detection (prompts, configs) | Yes — file diffs | **High** |
| Change detection (provider model updates) | Mostly — behavioral fingerprinting | **High** |
| Pre-deploy format/schema regression | Yes — structural comparison | **High** |
| Pre-deploy integration testing | Yes — downstream agent still works? | **High** |
| Post-deploy metric monitoring | Yes — latency, errors, tokens | **High** |
| Drift detection via statistical distribution shift | Yes — statistical tests | **High** |
| Change-to-impact correlation | Mostly — temporal correlation + known events | **Medium-High** |
| "Did quality actually drop?" | No — requires judgment | **Medium** (surface data, human decides) |
| "Is this output correct?" | No — domain-specific | **Not in scope** |

~80% of the product is deterministic engineering. The remaining 20% is "surface the right data so a human can decide."

---

## Technical Architecture

```
┌────────────────────────────────────────────────────────────┐
│                    CHANGE INTELLIGENCE                      │
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ Registry │  │ Change   │  │ Eval     │  │ Monitor  │  │
│  │ & Graph  │  │ Detector │  │ Engine   │  │ & Drift  │  │
│  └─────┬────┘  └─────┬────┘  └─────┬────┘  └─────┬────┘  │
│        └─────────────┴──────┬──────┴──────────────┘        │
│                             │                               │
│                    ┌────────▼────────┐                      │
│                    │  Change Event   │                      │
│                    │  Store          │                      │
│                    └─────────────────┘                      │
└────────────────────────────────────────────────────────────┘
```

### Data Ingestion
- SDK hooks (instrument agent frameworks — LangChain, CrewAI, custom)
- CI/CD webhooks (detect code/config changes)
- Git scanning (detect prompt/config file changes)
- Provider health APIs (detect model updates)
- Runtime metrics (latency, tokens, errors)
- User feedback signals (thumbs up/down, edits)

### Storage
- Graph DB (Neo4j/Memgraph) — topology and dependencies
- Time-series DB (ClickHouse) — metrics, behavioral fingerprints
- Change event store — every change, attributed, timestamped

---

## MVP Roadmap

### V0.1 — Discovery (Week 1-4)
```
$ changeint discover

Discovered 8 agents, 12 tools, 3 models, 4 data sources
47 dependencies mapped

⚠ Findings:
  - billing-agent has no fallback if claude-sonnet is unavailable
  - 3 agents share the same API key (security risk)
  - research-agent calls 2 agents that call each other (circular)
```

Show them what they have. The "terraform plan" moment.

### V0.2 — Change Tracking (Week 4-8)
Every prompt edit, config change, model update logged, attributed, correlated with quality metrics.

### V0.3 — Pre-Deploy Eval Gate (Week 8-12)
CI integration. Auto-run regression evals on affected agents and downstream dependencies. Block deploy if regressions found.

### V0.4 — Drift Detection (Week 12-16)
Continuous behavioral fingerprint monitoring. Alert on drift. Suggest root causes.

---

## Revenue Model

| Tier | Price | Includes |
|---|---|---|
| Open Source | Free | CLI discovery, local graph, basic SDK |
| Team | $500/month | Hosted graph, 20 agents, change tracking, 30-day history |
| Business | $2,000/month | 100 agents, CI/CD gates, drift detection, 90-day history |
| Enterprise | $50K-200K/year | Unlimited, SSO, RBAC, compliance add-on, self-hosted |

### Expansion Play: Compliance Add-On (Year 2)

Same data layer powers compliance evidence generation:
- EU AI Act evidence from change tracking + audit trails
- SOC2 AI controls from access patterns + governance data
- Sold to GRC/CISO team — different buyer, bigger budget ($100-200K/year)

---

## Go-to-Market

### Wedge Use Case
> "I changed a prompt and it broke a downstream agent. It took us 2 days to figure out what happened."

Every team with 5+ agents has this story.

### Buying Motion
1. Engineer has production incident caused by agent change
2. Postmortem reveals no visibility into dependencies or change impact
3. They find your tool
4. Install SDK, see dependency graph for first time
5. "Oh shit, I didn't know agent X depended on agent Y"
6. They buy

Pull motion, not push.

### The Pitch
> "You have 20 AI agents in production. Someone changes a prompt and it breaks a downstream agent. You spend 2 days debugging. We show you the blast radius before you deploy, gate your changes with automated regression testing, and tell you exactly what changed when something drifts. Change intelligence for AI systems."
