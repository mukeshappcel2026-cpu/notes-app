# YC Startup Ideas — AI Agent Outcome Verification

## The Core Insight

AI agents are taking real-world actions (restarting services, blocking IPs, modifying databases, sending emails). Every agent vendor reports "action completed successfully." Nobody independently verifies whether the action **actually achieved the intended outcome**.

This is the fundamental trust gap in AI agent adoption. We close it.

---

## The Product: Agent Outcome Verification Engine

**One-liner:** "We tell you if your AI agent actually worked — not just that it ran."

### The Atomic Problem

Traditional software: function returns a value, you assert on it. Done.

AI agents: agent takes 8 actions across 3 systems over 45 seconds, and "success" is a subjective, delayed, context-dependent judgment that nobody is measuring.

Every team with production agents is flying blind on outcomes. They know the agent ran. They don't know if it worked. They find out days or weeks later when a customer complains, a bug resurfaces, or an audit fails.

### Why Existing Tools Don't Solve This

- **LangSmith/Langfuse:** Trace what happened. Don't verify outcomes.
- **Guardrails AI:** Validates LLM outputs (format, toxicity). Doesn't verify real-world consequences.
- **Braintrust:** Runs evals on test datasets. Doesn't evaluate production outcomes.
- **Arize:** Monitors model metrics. Doesn't understand agent-level success/failure.

Everyone is building **input-side** tools (prompt management, guardrails, tracing). Nobody is solving the **output-side**: did it actually work?

---

## MVP: SRE Agent Outcome Verification (4 weeks)

### Why SRE First

- Most objectively measurable outcomes (infrastructure metrics)
- Highest urgency (downtime = money)
- SRE teams are early agent adopters (PagerDuty, Shoreline, Rootly all shipping agents)
- Fast feedback loops (outcomes visible in minutes)

### How It Works

Every SRE agent action maps to verifiable outcome probes:

| Agent Action | Outcome Probes |
|---|---|
| Restart service | 1. Did trigger metric recover (sustained 15min)? 2. Did error rates return to baseline? 3. Did alert re-fire within 1hr? 4. Collateral: did other services degrade? 5. Causality: was metric already recovering? |
| Scale up replicas | 1. Did throughput increase? 2. Did latency decrease? 3. Was scaling even needed (CPU/mem check)? 4. Cost impact: $/hr added. 5. Did it scale back down in expected window? |
| Rollback deploy | 1. Did new-deploy error pattern disappear? 2. Did we regress metrics the deploy improved? 3. DB migration state consistency? |
| Modify config | 1. Clean service restart? 2. Target behavior change confirmed? 3. Downstream breakage? |

### The Hard Part: Causal Verification

Agent restarts a service at 14:23. Latency recovers at 14:25. Agent claims credit. But actually:
- An autoscaler had already added 3 replicas at 14:21
- An upstream traffic spike ended at 14:22
- The restart had nothing to do with the recovery

**Agents taking credit for coincidental recoveries is the #1 source of false confidence in SRE automation.**

Causal verification approach:

1. Capture metric trajectory BEFORE agent action (was it already recovering?)
2. Compare recovery slope with vs without action (statistical counterfactual)
3. Check if the action target was the actual source of the problem
4. Look for confounding events in the same window (autoscaler, other deploys, traffic changes)

Verdict: CAUSAL / UNCLEAR / SPURIOUS

### Architecture (Minimal)

```
┌─────────────────────────────────────┐
│      Verification Dashboard         │
│  Score per action, trends, alerts   │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│         Verification Engine          │
│                                      │
│  1. Receive: agent action event      │
│  2. Wait: configurable window        │
│     (5min, 15min, 1hr, 24hr)         │
│  3. Probe: query metrics sources     │
│  4. Analyze: run causal checks       │
│  5. Verdict: score the outcome       │
│  6. Alert: if failed/spurious        │
└──────┬───────────────┬──────────────┘
       │               │
┌──────▼──────┐ ┌──────▼──────┐
│ Metrics     │ │  Event      │
│ Sources     │ │  Sources    │
│             │ │             │
│ Prometheus  │ │ K8s events  │
│ Datadog     │ │ Deploy logs │
│ CloudWatch  │ │ PagerDuty   │
│ Grafana     │ │ Autoscaler  │
└─────────────┘ └─────────────┘
```

No ClickHouse. No Kafka. Postgres for storing verdicts. Pull metrics from sources the customer already has. Dead simple.

### MVP Build Plan

- **Week 1:** Webhook receiver + Prometheus integration. Agent POSTs action event, engine queries trigger metric before/after, basic verdict (did metric recover? yes/no).
- **Week 2:** Durability + collateral checks. Alert re-fire detection. Related service degradation check.
- **Week 3:** Causal scoring. Slope analysis, confounding event detection, causal/unclear/spurious verdicts.
- **Week 4:** Dashboard + integration with one SRE agent platform (PagerDuty or Shoreline webhook). Alert on dropping causal scores.

### The Demo Stat

> "Your SRE agent resolved 142 incidents last month. We verified outcomes: 89 were actually fixed by the agent, 31 were coincidental recoveries the agent took credit for, and 22 re-fired within an hour. Your agent's true effectiveness is 63%, not the 100% it reports."

---

## The Moat

### Why Agent Vendors Won't Build This

**Structural incentive misalignment.** PagerDuty's value prop is "our agent resolved 142 incidents." An honest verification layer that reveals 37% were false attributions cuts their own success metric. No product team ships that. No sales team wants that number in a QBR.

The entity performing the action cannot be the entity judging the outcome. Same reason car dealerships don't build CarFax.

### Why the Moat Compounds

1. **Cross-vendor benchmarking:** We see outcomes across PagerDuty, Shoreline, Rootly, and homegrown agents. No individual vendor can benchmark against competitors. "For restart actions, PagerDuty has 71% true-fix rate, Shoreline has 68%."
2. **Causal inference improves with data:** With 100K verified actions across hundreds of companies, we build the best counterfactual baselines. We know payment services self-recover from memory pressure in ~8 minutes. If the agent resolved it in 2 minutes, that's causal. In 9 minutes, probably coincidental.
3. **Standard/certification:** "This agent is AgentVerify-certified with 89% true-effectiveness." Agent vendors need this stamp to close enterprise deals.

---

## Domain Expansion Roadmap

### Phase 1: SRE Agents (Months 0-4)
- Metrics-based verification (Prometheus, Datadog, CloudWatch)
- Causal inference engine
- Proves the model works

### Phase 2: Security Agents (Months 4-8)
- Same engine, new probes: Did blocking an IP stop the attack? Did isolating a host break production? False positive rate of automated remediations?
- Higher ACV (security buyers have budget, compliance pressure)
- Incentive misalignment even stronger (security vendors report "threats blocked" — never report false positive rate)

### Phase 3: Data Pipeline Agents (Months 8-12)
- New probe type: data quality assertions (row counts, schema validation, distribution checks)
- Catches silently wrong data that doesn't trigger alerts but causes bad dashboards and decisions
- Large buyer ecosystem (modern data stack teams spend aggressively on tooling)

### Phase 4: CI/CD & Coding Agents (Months 12-16)
- Deeper semantic verification: did the auto-fix change behavior or just suppress the error? Performance regression? New vulnerability introduced?
- Massive market (every company using coding agents)
- Builds on all prior probe infrastructure

### What We're NOT Doing

Customer support, sales, legal, content agents — outcomes are subjective and delayed. Low moat, mushy verification. We stay in domains where outcomes are objectively measurable.

---

## Who Buys This and Why

### Buyer 1: VP of Engineering / Head of Platform
**Problem:** Deployed 3-8 agents, CEO asks "how much are these actually saving us?" No independent measurement exists.
**Value:** ROI justification for entire AI agent investment. Cut underperforming agents, double down on effective ones.
**Price:** $2-5K/mo (noise against $200K/yr agent tooling budget)

### Buyer 2: CISO / Head of Security
**Problem:** Regulators, SOC 2 auditors, and insurance underwriters asking "how do you know your AI systems operate correctly?"
**Value:** Independent audit trail proving every autonomous action was verified. "We have independent outcome verification" vs "we trust the vendor's dashboard."
**Price:** $5K+/mo (compliance budget)

### Buyer 3: The Agent Vendor Themselves
**Problem:** Enterprise sales. Bank's CISO says "prove your agent works. Show me independent verification."
**Value:** Third-party certification that closes enterprise deals. "Our agent is AgentVerify-certified with 89% true-effectiveness across 200 deployments."
**Price:** Partnership/licensing deal

### Buyer 4: Insurance / Risk (18-24 months out)
**Problem:** Can't price cyber insurance policies for companies using autonomous AI agents without quantified risk data.
**Value:** Actuarial data on agent failure rates by type, domain, and vendor.
**Price:** Data licensing

---

## Why Agents Getting Better Doesn't Kill This

The better agents get, the more autonomy they're given. The more autonomy, the higher the stakes per action. The higher the stakes, the more verification matters.

Today: agent restarts a pod. If wrong, human catches it in 10 minutes.
In 2 years: agent rearchitects a failing service, migrates traffic, modifies schemas at 3am with no human in the loop.

That 5% failure rate on high-autonomy actions is catastrophically worse than 37% failure rate on low-stakes actions.

**We are the ratchet that enables increasing agent autonomy.** Without independent verification, agent autonomy hits a ceiling. No rational organization will let an agent take high-stakes actions without independent verification, no matter how good the agent claims to be. We raise that ceiling.

Commercial aviation analogy: Planes are 99.99% safe. We spend MORE on flight safety verification than when planes were dangerous — because we fly more, more often, with more people, in more conditions.

---

## YC Application Pitch

> "AI agents are taking real-world actions across infrastructure, security, data, and code. Every agent vendor reports success. Nobody independently verifies outcomes. We're building the verification layer — starting with SRE, where we can already show that 30-40% of 'resolved' incidents weren't actually fixed by the agent. Independent verification is how we raise the ceiling on agent autonomy."

### Revenue Potential
- 10K companies with production agents × $3K/mo avg = $360M ARR
- Grows with agent adoption (exponential)
- Expands ACV with each new domain (SRE → Security → Data → CI/CD)

### Key Risks

| Risk | Mitigation |
|---|---|
| Agent vendors build it in | Incentive misalignment prevents honest self-grading. Independence is the point. |
| Agents become provably correct | Decades away for real-world actions with messy state. Maybe never. |
| Too early, not enough agents deployed | Target companies already in production (they exist). Start with design partners. |
| Enterprise sales cycle is long | Land with SDK (engineering buyer, self-serve). Expand to compliance (enterprise buyer, high ACV). |
