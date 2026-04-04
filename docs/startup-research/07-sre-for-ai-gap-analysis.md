# SRE for AI: Complete Problem Map & Gap Analysis

*Research compiled March 2026*

---

## The 37 Problems an AI SRE Faces

### Category A: "Is It Working?" (Availability & Health)

| # | Problem | Best Current Solution | Status |
|---|---|---|---|
| A1 | Agent health monitoring | Datadog (latency, errors, throughput) | **Mostly solved** |
| A2 | LLM provider health | Datadog, Solo.io gateway | **Mostly solved** |
| A3 | Tool/dependency health | Datadog (infra monitoring) | **Mostly solved** |
| A4 | Multi-agent workflow health | Partial — everyone traces runs, nobody monitors ongoing workflow health | **NOT solved** |
| A5 | Failover/fallback management | Temporal (execution), Solo.io (routing) — separately | **Partially solved** |
| A6 | Output quality monitoring | Arize, Fiddler, LangSmith evals | **Solved for basic quality** |
| A7 | AI-specific incident detection | Datadog alerts on metrics, but no AI-specific incident detection | **Partially solved** |
| A8 | Alerting & on-call | Datadog + PagerDuty | **Solved** |

### Category B: "What Changed?" (Change Management)

| # | Problem | Best Current Solution | Status |
|---|---|---|---|
| B1 | Prompt change tracking | LangSmith prompt versioning | **Partially solved** |
| B2 | Model change detection | Arize/Fiddler drift detection | **NOT really solved** |
| B3 | Tool/data source changes | Nobody | **NOT solved** |
| B4 | Pre-deploy validation | LangSmith, Arize, Braintrust evals | **Partially solved** |
| B5 | Dependency mapping | Nobody | **NOT solved** |
| B6 | Blast radius analysis | Nobody | **NOT solved** |
| B7 | Rollback capability | Nobody | **NOT solved** |
| B8 | Change attribution | Nobody (manual investigation) | **NOT solved** |

### Category C: "Is It Safe?" (Security & Access)

| # | Problem | Best Current Solution | Status |
|---|---|---|---|
| C1 | Agent identity/IAM | Nobody | **NOT solved** |
| C2 | Data access control | Solo.io gateway authz (partial) | **NOT solved** |
| C3 | PII/sensitive data handling | Fiddler guardrails, Datadog SDS | **Partially solved** |
| C4 | Prompt injection defense | Fiddler Trust Service (<100ms) | **Partially solved** |
| C5 | Tool permission scoping | Nobody | **NOT solved** |
| C6 | Secret management | Nobody (use Vault manually) | **NOT solved** |
| C7 | Access audit trail | Fiddler (SOC2 audit trails) | **Partially solved** |

### Category D: "Can We Prove It?" (Compliance & Governance)

| # | Problem | Best Current Solution | Status |
|---|---|---|---|
| D1 | Regulatory evidence collection | Nobody auto-generates evidence | **NOT solved** |
| D2 | Human oversight documentation | Nobody | **NOT solved** |
| D3 | Model/agent inventory | Datadog AI Agents Console (preview) | **Partially solved** |
| D4 | Bias/fairness monitoring | Fiddler (traditional ML only) | **Partially solved** |
| D5 | Incident documentation | Nobody for AI-specific | **NOT solved** |
| D6 | Risk assessment per agent | Nobody | **NOT solved** |
| D7 | Audit-ready reporting | Nobody | **NOT solved** |

### Category E: "Is It Worth It?" (Cost & Performance)

| # | Problem | Best Current Solution | Status |
|---|---|---|---|
| E1 | Cost tracking per agent | Datadog, LangSmith, Fiddler | **Mostly solved** |
| E2 | Cost optimization / model routing | Nobody | **NOT solved** |
| E3 | Token waste detection | Nobody | **NOT solved** |
| E4 | Budget controls / circuit breakers | Temporal timeouts, Solo.io rate limiting | **Partially solved** |
| E5 | Chargeback to business units | Nobody | **NOT solved** |
| E6 | ROI measurement | Datadog AI Agents Console (preview) | **NOT solved** |

### Category F: "Will It Keep Working?" (Reliability Engineering)

| # | Problem | Best Current Solution | Status |
|---|---|---|---|
| F1 | SLO definition for agents | Nobody | **NOT solved** |
| F2 | Error budgets for agents | Nobody | **NOT solved** |
| F3 | Chaos engineering for agents | Solo.io agentevals (just launched) | **NOT solved** |
| F4 | Capacity planning | Temporal worker scaling (partial) | **NOT solved** |
| F5 | Graceful degradation | Temporal + Solo.io (separate pieces) | **Partially solved** |
| F6 | Runbook automation for AI | Datadog Bits AI (traditional infra) | **NOT solved** for AI |

---

## Summary

| Status | Count |
|---|---|
| NOT solved by anyone | **18 of 37** (49%) |
| Partially solved | **13 of 37** (35%) |
| Mostly/fully solved | **6 of 37** (16%) |

---

## Competitive Landscape Detail

### Datadog AI Monitoring
- LLM Observability (GA), AI Agent Monitoring (GA), AI Agents Console (Preview)
- Strengths: breadth, existing enterprise relationships, cross-correlation with infra
- Gaps: framework-specific auto-instrumentation, no change management, opaque pricing (~$120/day auto-activation reported), 15-day trace retention
- Good at: "is it up?" Bad at: "what changed and why?"

### Arize AI ($131M, Series C)
- Best-in-class post-hoc agent debugging and evaluation
- Phoenix open source (8K GitHub stars, 2M+ monthly downloads)
- Gaps: observability only (doesn't prevent failures), engineering-centric UI, no change management, no dependency mapping

### Fiddler AI ($100M, Series C)
- "AI Control Plane" — observability + guardrails + governance
- Trust Service guardrails (<100ms, in-environment) are a genuine differentiator
- Gaps: steep learning curve, no free tier, Python-only agent SDKs, not LLM-native (extended from ML monitoring), former employees questioning PMF

### Temporal ($300M, $5B valuation)
- Durable execution for stateful workflows — ensures agents survive crashes
- OpenAI Codex and Replit Agent 3 built on Temporal
- Gaps: infrastructure not SRE tooling, no monitoring/evals/guardrails/networking, significant learning curve

### Solo.io ($130M+)
- Agent Gateway (MCP + A2A protocol support, Linux Foundation)
- "AIRE" concept — aspirational, not fully shipping
- Gaps: enterprise gateway in beta, no production case studies, LLM gateway support incomplete

### LangSmith ($260M, $1.25B valuation)
- Agent engineering platform with tracing, prompt management, evals
- Gaps: LangChain-coupled, dev tool not ops tool, no change management
