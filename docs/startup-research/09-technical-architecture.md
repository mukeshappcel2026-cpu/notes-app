# Change Intelligence for AI Systems: Technical Architecture & Build Phases

## System Name: **ChangeInt** (working name)

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              USER INTERFACES                                │
│                                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐    │
│  │   CLI    │  │   Web    │  │  CI/CD   │  │  Slack   │  │   API    │    │
│  │  Tool    │  │Dashboard │  │  Plugin  │  │   Bot    │  │(REST/gRPC)│   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘    │
│       └──────────────┴──────────┬──┴──────────────┴──────────────┘         │
│                                 │                                           │
├─────────────────────────────────┼───────────────────────────────────────────┤
│                          CORE SERVICES                                      │
│                                 │                                           │
│  ┌──────────────┐  ┌───────────▼──────────┐  ┌──────────────────────┐     │
│  │   Registry   │  │    Change Event      │  │    Query Engine      │     │
│  │   Service    │  │    Processor         │  │    (GraphQL)         │     │
│  │              │  │                      │  │                      │     │
│  │ • Agent CRUD │  │ • Ingest changes     │  │ • Dependency queries │     │
│  │ • Tool CRUD  │  │ • Correlate events   │  │ • Blast radius       │     │
│  │ • Model CRUD │  │ • Trigger alerts     │  │ • Change timeline    │     │
│  │ • Data src   │  │ • Trigger evals      │  │ • Drift reports      │     │
│  └──────┬───────┘  └──────────┬───────────┘  └──────────┬───────────┘     │
│         │                     │                          │                  │
│  ┌──────▼─────────────────────▼──────────────────────────▼───────────┐     │
│  │                      DATA LAYER                                   │     │
│  │                                                                   │     │
│  │  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐  │     │
│  │  │  Graph DB   │  │  Time-Series │  │  Change Event Store    │  │     │
│  │  │  (Neo4j /   │  │  (ClickHouse │  │  (Postgres + event     │  │     │
│  │  │  Memgraph)  │  │  / TimescaleDB│ │   sourcing)            │  │     │
│  │  │             │  │  )           │  │                        │  │     │
│  │  │ • Topology  │  │ • Metrics    │  │ • Every change ever    │  │     │
│  │  │ • Deps      │  │ • Fingerprts │  │ • Attribution          │  │     │
│  │  │ • Versions  │  │ • SLO data   │  │ • Causation links      │  │     │
│  │  └─────────────┘  └──────────────┘  └────────────────────────┘  │     │
│  └───────────────────────────────────────────────────────────────────┘     │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                          INGESTION LAYER                                    │
│                                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │  Python  │  │    TS    │  │  CI/CD   │  │   Git    │  │ Provider │   │
│  │   SDK    │  │   SDK    │  │ Webhooks │  │ Scanner  │  │ Monitor  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│   Runtime       Runtime       Deploy-time    Config-time   Continuous     │
│   traces        traces        changes        changes       health         │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Tech Stack Decisions

| Component | Choice | Why |
|---|---|---|
| **Language** | Go (services), Python + TypeScript (SDKs) | Go for performance/deployment simplicity; SDKs in the languages agent developers use |
| **Graph DB** | Memgraph (start), Neo4j (scale) | Memgraph is in-memory, fast, Cypher-compatible, easy to self-host. Migrate to Neo4j cluster at scale |
| **Time-Series** | TimescaleDB (start), ClickHouse (scale) | TimescaleDB = Postgres extension, reduces infra. ClickHouse when query volume demands it |
| **Event Store** | Postgres with JSONB + event sourcing pattern | Simple, reliable, good enough for v0. Every change is an immutable event |
| **Message Queue** | NATS (start), Kafka (scale) | NATS is lightweight, embedded-friendly. Kafka when throughput exceeds 100K events/sec |
| **API** | GraphQL (queries), gRPC (SDK ingestion), REST (webhooks) | GraphQL is perfect for graph queries with variable depth. gRPC for high-throughput SDK telemetry |
| **Web UI** | React + D3.js (graph viz) + Tremor (dashboards) | D3 for interactive dependency graph. Tremor for metrics dashboards |
| **CI/CD Integration** | GitHub Actions, GitLab CI, generic webhook | GitHub first (largest market), expand to GitLab/Jenkins |
| **Deployment** | Single binary (Go) + Docker Compose (self-hosted) + managed cloud | Single binary for CLI/OSS. Docker Compose for self-hosted. Cloud for SaaS |

---
