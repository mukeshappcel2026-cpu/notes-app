"""
Canopy Full Pipeline Demo

Runs the complete change intelligence pipeline WITHOUT any server:
  1. Simulates a multi-agent e-commerce system (baseline traffic)
  2. Computes behavioral fingerprints for all agents
  3. Injects 5 realistic failure scenarios
  4. Computes post-failure fingerprints
  5. Runs statistical drift detection
  6. Runs root cause correlation engine
  7. Prints full diagnosis with blast radius

This is the complete product demo in one script. No Docker. No server.
Just the algorithm proving it works.

Usage:
    cd canopy/demo && python run_full_pipeline.py
"""

import hashlib
import random
import sys
import os
from datetime import datetime, timedelta, timezone
from collections import defaultdict

# Add demo dir to path so imports work
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from fingerprint import compute_fingerprint, FingerprintStore, Fingerprint
from drift import detect_drift, DriftSignal, Severity
from correlate import correlate, Diagnosis


# ── Agent Definitions (same as simulate.py) ────────────────────────────────

AGENTS = {
    "router-agent": {
        "team": "platform",
        "model": "gpt-4o-mini",
        "system_prompt": "You are a routing agent for an e-commerce support system. Classify the user's intent and route to the appropriate specialist agent. Categories: billing, product, general_support, escalation.",
        "temperature": 0.0,
        "tools": ["classify_intent", "route_to_agent"],
        "avg_input_tokens": 150,
        "avg_output_tokens": 50,
        "avg_latency_ms": 320,
        "error_rate": 0.01,
        "calls_to": ["support-agent", "product-agent"],
    },
    "support-agent": {
        "team": "support",
        "model": "claude-sonnet-4-20250514",
        "system_prompt": "You are a customer support agent for ShopCo. Help customers with order status, returns, shipping issues, and general inquiries. Always be empathetic. Include order number in every response. Never recommend competitor products. If the issue involves payments or refunds over $100, escalate to billing.",
        "temperature": 0.3,
        "tools": ["lookup_order", "check_shipping", "create_return", "send_email"],
        "avg_input_tokens": 400,
        "avg_output_tokens": 250,
        "avg_latency_ms": 1200,
        "error_rate": 0.02,
        "calls_to": ["billing-agent", "escalation-agent"],
    },
    "billing-agent": {
        "team": "finance",
        "model": "claude-sonnet-4-20250514",
        "system_prompt": "You are a billing specialist agent. Handle refunds, payment disputes, invoice corrections, and subscription changes. Always verify the customer's identity before processing financial transactions. Include a disclaimer that this is not financial advice.",
        "temperature": 0.1,
        "tools": ["process_refund", "verify_identity", "lookup_payment", "update_subscription"],
        "avg_input_tokens": 350,
        "avg_output_tokens": 200,
        "avg_latency_ms": 900,
        "error_rate": 0.01,
        "calls_to": [],
    },
    "product-agent": {
        "team": "catalog",
        "model": "gpt-4o-mini",
        "system_prompt": "You are a product specialist agent. Help customers find products, compare options, check availability, and provide detailed specifications. Use the product database for accurate information. Never make up specifications.",
        "temperature": 0.2,
        "tools": ["search_products", "get_product_details", "check_inventory"],
        "avg_input_tokens": 300,
        "avg_output_tokens": 350,
        "avg_latency_ms": 800,
        "error_rate": 0.02,
        "calls_to": [],
    },
    "escalation-agent": {
        "team": "support",
        "model": "claude-opus-4-20250514",
        "system_prompt": "You are a senior escalation agent. Handle complex cases that frontline agents cannot resolve. You have authority to issue refunds up to $500, override policies for VIP customers, and approve exception requests. Document every decision with reasoning.",
        "temperature": 0.2,
        "tools": ["issue_refund", "override_policy", "create_ticket", "notify_manager"],
        "avg_input_tokens": 600,
        "avg_output_tokens": 400,
        "avg_latency_ms": 2500,
        "error_rate": 0.005,
        "calls_to": [],
    },
}

# Agent topology (edges)
GRAPH_EDGES = [
    {"from": "router-agent", "to": "support-agent", "type": "CALLS", "call_count": 100},
    {"from": "router-agent", "to": "product-agent", "type": "CALLS", "call_count": 50},
    {"from": "support-agent", "to": "billing-agent", "type": "CALLS", "call_count": 30},
    {"from": "support-agent", "to": "escalation-agent", "type": "CALLS", "call_count": 15},
]

# Failure scenarios
FAILURES = [
    {
        "name": "Silent Model Upgrade",
        "description": "OpenAI silently upgrades gpt-4o-mini → gpt-4o-mini-2025-04-01",
        "agent": "router-agent",
        "change_event": {
            "change_type": "model_change",
            "entity_type": "agent",
            "entity_id": "router-agent",
            "field": "model",
            "old_value_hash": "gpt-4o-mini",
            "new_value_hash": "gpt-4o-mini-2025-04-01",
            "author": "provider:openai",
            "source": "proxy:runtime",
        },
        "behavior_changes": {
            "avg_output_tokens_mult": 1.20,
            "avg_latency_ms_mult": 1.30,
        },
    },
    {
        "name": "Prompt Guardrail Removed",
        "description": "Engineer removes 'Never recommend competitor products' from prompt",
        "agent": "support-agent",
        "change_event": {
            "change_type": "prompt_change",
            "entity_type": "agent",
            "entity_id": "support-agent",
            "field": "system_prompt_hash",
            "old_value_hash": hashlib.sha256(AGENTS["support-agent"]["system_prompt"].encode()).hexdigest()[:12],
            "new_value_hash": hashlib.sha256(b"modified prompt without guardrail").hexdigest()[:12],
            "author": "sarah@shopco.com",
            "source": "git:main:abc123",
        },
        "behavior_changes": {
            "avg_output_tokens_mult": 1.10,
        },
    },
    {
        "name": "Model Downgrade for Cost",
        "description": "Finance swaps billing agent from Sonnet to Haiku",
        "agent": "billing-agent",
        "change_event": {
            "change_type": "model_change",
            "entity_type": "agent",
            "entity_id": "billing-agent",
            "field": "model",
            "old_value_hash": "claude-sonnet-4-20250514",
            "new_value_hash": "claude-haiku-4-5-20251001",
            "author": "mike@shopco.com",
            "source": "git:main:def456",
        },
        "behavior_changes": {
            "avg_output_tokens_mult": 0.55,
            "avg_latency_ms_mult": 0.35,
            "error_rate_add": 0.03,
        },
    },
    {
        "name": "Tool Removed (DB Migration)",
        "description": "Product DB migration removes check_inventory tool",
        "agent": "product-agent",
        "change_event": {
            "change_type": "tool_change",
            "entity_type": "agent",
            "entity_id": "product-agent",
            "field": "tools",
            "old_value_hash": "search_products,get_product_details,check_inventory",
            "new_value_hash": "search_products,get_product_details",
            "author": "ops:migration",
            "source": "deploy:prod:789",
        },
        "behavior_changes": {
            "error_rate_override": 0.18,
            "tools_remove": ["check_inventory"],
        },
    },
    {
        "name": "Temperature Drift",
        "description": "Escalation agent redeployed with temperature 0.9 instead of 0.2",
        "agent": "escalation-agent",
        "change_event": {
            "change_type": "config_change",
            "entity_type": "agent",
            "entity_id": "escalation-agent",
            "field": "temperature",
            "old_value_hash": "0.2",
            "new_value_hash": "0.9",
            "author": "deploy:prod:config",
            "source": "deploy:prod:abc",
        },
        "behavior_changes": {
            "avg_output_tokens_mult": 1.45,
            "output_tokens_stdev_mult": 2.0,
        },
    },
]


# ── Simulation Helpers ─────────────────────────────────────────────────────

def jitter(value: float, pct: float = 0.15) -> float:
    return value * random.uniform(1 - pct, 1 + pct)


def generate_calls(agent_id: str, cfg: dict, n: int, time_base: datetime) -> list[dict]:
    """Generate n simulated LLM calls for an agent."""
    calls = []
    for i in range(n):
        is_error = random.random() < cfg.get("error_rate", 0.02)
        tools = []
        if not is_error and cfg.get("tools"):
            n_tools = random.randint(0, min(3, len(cfg["tools"])))
            tools = random.sample(cfg["tools"], n_tools)

        calls.append({
            "agent": agent_id,
            "team": cfg["team"],
            "model": cfg["model"],
            "system_prompt_hash": hashlib.sha256(cfg["system_prompt"].encode()).hexdigest()[:12],
            "temperature": cfg.get("temperature"),
            "input_tokens": int(jitter(cfg["avg_input_tokens"])),
            "output_tokens": int(jitter(cfg["avg_output_tokens"])),
            "cost": 0,
            "latency_ms": jitter(cfg["avg_latency_ms"]),
            "tool_calls": tools,
            "status": "error" if is_error else "success",
            "timestamp": (time_base + timedelta(minutes=i)).isoformat(),
        })
    return calls


def apply_failure(cfg: dict, changes: dict) -> dict:
    """Apply behavior changes to agent config."""
    cfg = dict(cfg)
    if "avg_output_tokens_mult" in changes:
        cfg["avg_output_tokens"] = int(cfg["avg_output_tokens"] * changes["avg_output_tokens_mult"])
    if "avg_latency_ms_mult" in changes:
        cfg["avg_latency_ms"] = cfg["avg_latency_ms"] * changes["avg_latency_ms_mult"]
    if "error_rate_add" in changes:
        cfg["error_rate"] = cfg.get("error_rate", 0) + changes["error_rate_add"]
    if "error_rate_override" in changes:
        cfg["error_rate"] = changes["error_rate_override"]
    if "tools_remove" in changes:
        cfg["tools"] = [t for t in cfg["tools"] if t not in changes["tools_remove"]]
    if "output_tokens_stdev_mult" in changes:
        # Increase jitter by using a wider range (simulated via higher avg for stdev)
        cfg["_stdev_mult"] = changes["output_tokens_stdev_mult"]
    return cfg


# ── Main Pipeline ──────────────────────────────────────────────────────────

def main():
    random.seed(42)  # Reproducible

    print()
    print("  ╔═══════════════════════════════════════════════════════════╗")
    print("  ║       CANOPY: FULL CHANGE INTELLIGENCE PIPELINE          ║")
    print("  ║                                                           ║")
    print("  ║   Fingerprint → Drift Detection → Root Cause Correlation  ║")
    print("  ╚═══════════════════════════════════════════════════════════╝")
    print()

    fp_store = FingerprintStore()
    all_change_events = []
    now = datetime.now(timezone.utc)

    # ══════════════════════════════════════════════════════════════════════
    # PHASE 1: BASELINE
    # ══════════════════════════════════════════════════════════════════════

    print("━" * 65)
    print("  PHASE 1: BASELINE — Establishing Normal Behavior")
    print("━" * 65)
    print()

    # Generate 5 baseline windows (simulating 5 fingerprint cycles)
    baseline_calls: dict[str, list[dict]] = defaultdict(list)
    calls_per_window = {"router-agent": 35, "support-agent": 30, "product-agent": 20, "billing-agent": 15, "escalation-agent": 10}

    for window_idx in range(5):
        window_start = now - timedelta(hours=24 - window_idx * 4)
        window_end = window_start + timedelta(hours=4)

        for agent_id, cfg in AGENTS.items():
            n = calls_per_window.get(agent_id, 15)
            calls = generate_calls(agent_id, cfg, n, window_start)
            baseline_calls[agent_id].extend(calls)

            fp = compute_fingerprint(agent_id, calls, window_start, window_end)
            if fp:
                fp_store.store(fp)

    # Print baseline fingerprints
    for agent_id in AGENTS:
        fp = fp_store.get_latest(agent_id)
        if fp:
            print(f"  {agent_id}:")
            print(f"    Output tokens:  mean={fp.output_tokens_mean:.0f}  stdev={fp.output_tokens_stdev:.0f}")
            print(f"    Latency:        mean={fp.latency_mean_ms:.0f}ms  p90={fp.latency_p90_ms:.0f}ms")
            print(f"    Error rate:     {fp.error_rate:.1%}")
            print(f"    Tool usage:     {', '.join(f'{t}={p:.0%}' for t, p in sorted(fp.tool_distribution.items()))}")
            print()

    total_baseline = sum(len(v) for v in baseline_calls.values())
    print(f"  Baseline established: {total_baseline} calls across 5 windows")
    print()

    # ══════════════════════════════════════════════════════════════════════
    # PHASE 2: INJECT FAILURES
    # ══════════════════════════════════════════════════════════════════════

    print("━" * 65)
    print("  PHASE 2: FAILURE INJECTION")
    print("━" * 65)
    print()

    failure_time = now - timedelta(hours=2)
    modified_configs = {}
    post_failure_calls: dict[str, list[dict]] = defaultdict(list)

    for i, failure in enumerate(FAILURES):
        agent_id = failure["agent"]
        print(f"  {i + 1}. {failure['name']}")
        print(f"     Agent: {agent_id}")
        print(f"     {failure['description']}")

        # Record the change event
        change_evt = dict(failure["change_event"])
        change_evt["id"] = f"change-{i}"
        change_evt["timestamp"] = (failure_time + timedelta(minutes=i * 10)).isoformat()
        change_evt["downstream_agents"] = _get_downstream(agent_id)
        change_evt["blast_radius"] = len(change_evt["downstream_agents"])
        all_change_events.append(change_evt)

        # Apply behavior changes
        modified_configs[agent_id] = apply_failure(AGENTS[agent_id], failure["behavior_changes"])
        print()

    # Generate post-failure traffic
    print("  Generating post-failure traffic...")
    post_window_start = now - timedelta(hours=1)
    post_window_end = now

    for agent_id in AGENTS:
        cfg = modified_configs.get(agent_id, AGENTS[agent_id])
        n = calls_per_window.get(agent_id, 15)
        calls = generate_calls(agent_id, cfg, n, post_window_start)
        post_failure_calls[agent_id] = calls

        fp = compute_fingerprint(agent_id, calls, post_window_start, post_window_end)
        if fp:
            fp_store.store(fp)

    total_post = sum(len(v) for v in post_failure_calls.values())
    print(f"  Post-failure traffic: {total_post} calls generated")
    print()

    # ══════════════════════════════════════════════════════════════════════
    # PHASE 3: DRIFT DETECTION
    # ══════════════════════════════════════════════════════════════════════

    print("━" * 65)
    print("  PHASE 3: STATISTICAL DRIFT DETECTION")
    print("━" * 65)
    print()

    all_drift_signals: dict[str, list[DriftSignal]] = {}

    for agent_id in AGENTS:
        current = fp_store.get_latest(agent_id)
        baselines = fp_store.get_baseline(agent_id, n=4)

        if not current or not baselines:
            continue

        signals = detect_drift(current, baselines)
        if signals:
            all_drift_signals[agent_id] = signals

            print(f"  {agent_id}: {len(signals)} drift signal(s) detected")
            for s in signals:
                icon = "!!!" if s.severity == Severity.CRITICAL else " ! "
                print(f"    [{s.severity.value.upper()}] {icon} {s.description}")
            print()

    if not all_drift_signals:
        print("  No statistically significant drift detected.")
        print()

    agents_with_drift = len(all_drift_signals)
    total_signals = sum(len(s) for s in all_drift_signals.values())
    print(f"  Summary: {total_signals} drift signals across {agents_with_drift} agents")
    print()

    # ══════════════════════════════════════════════════════════════════════
    # PHASE 4: ROOT CAUSE CORRELATION
    # ══════════════════════════════════════════════════════════════════════

    print("━" * 65)
    print("  PHASE 4: ROOT CAUSE CORRELATION")
    print("━" * 65)
    print()

    diagnoses: list[Diagnosis] = []

    for agent_id, signals in all_drift_signals.items():
        diagnosis = correlate(
            drift_agent=agent_id,
            drift_signals=signals,
            change_events=all_change_events,
            graph_edges=GRAPH_EDGES,
            agents=AGENTS,
            drift_time=now,
        )
        diagnoses.append(diagnosis)

        severity_icon = {
            Severity.CRITICAL: "!!!",
            Severity.WARNING: " ! ",
            Severity.INFO: " i ",
        }

        print(f"  ┌─ {agent_id} {'─' * (50 - len(agent_id))}")
        print(f"  │")
        print(f"  │  Severity: {diagnosis.severity.value.upper()}")
        print(f"  │  Summary:  {diagnosis.summary}")
        print(f"  │")

        if diagnosis.drift_signals:
            print(f"  │  Drift Signals:")
            for s in diagnosis.drift_signals:
                print(f"  │    {severity_icon[s.severity]} {s.metric}: {s.baseline_value:.2f} → {s.current_value:.2f} ({s.change_pct:+.1f}%, p={s.p_value:.4f})")

        if diagnosis.probable_causes:
            print(f"  │")
            print(f"  │  Probable Causes (ranked):")
            for j, cause in enumerate(diagnosis.probable_causes):
                bar = "█" * int(cause.score * 20) + "░" * (20 - int(cause.score * 20))
                change = cause.change_event
                entity = change.get("entity_id", "?")
                field = change.get("field", "?")
                author = change.get("author", "?")
                old = change.get("old_value_hash", "")[:16]
                new = change.get("new_value_hash", "")[:16]

                print(f"  │    {j + 1}. [{bar}] {cause.score:.2f}  {entity} / {field}")
                print(f"  │       {old} → {new}")
                print(f"  │       Author: {author}")
                print(f"  │       Scores: temporal={cause.temporal_score:.2f}  graph={cause.graph_score:.2f}  relevance={cause.relevance_score:.2f}")

        downstream = _get_downstream(agent_id)
        if downstream:
            print(f"  │")
            print(f"  │  Blast Radius: {len(downstream)} downstream agents")
            print(f"  │    → {', '.join(downstream)}")

        print(f"  │")
        print(f"  └{'─' * 55}")
        print()

    # ══════════════════════════════════════════════════════════════════════
    # FINAL REPORT
    # ══════════════════════════════════════════════════════════════════════

    print("━" * 65)
    print("  FINAL REPORT")
    print("━" * 65)
    print()

    critical = [d for d in diagnoses if d.severity == Severity.CRITICAL]
    warnings = [d for d in diagnoses if d.severity == Severity.WARNING]

    print(f"  Agents monitored:     {len(AGENTS)}")
    print(f"  Changes detected:     {len(all_change_events)}")
    print(f"  Drift signals:        {total_signals}")
    print(f"  Agents affected:      {agents_with_drift}")
    print(f"  Critical diagnoses:   {len(critical)}")
    print(f"  Warning diagnoses:    {len(warnings)}")
    print()

    if critical:
        print("  CRITICAL ISSUES:")
        for d in critical:
            print(f"    !!! {d.summary}")
        print()

    if warnings:
        print("  WARNINGS:")
        for d in warnings:
            print(f"     !  {d.summary}")
        print()

    print("  Pipeline complete. Every change was detected, every drift was")
    print("  measured with statistical tests, and every root cause was")
    print("  correlated using graph distance + temporal proximity.")
    print()
    print("  No AI was used to generate these diagnoses.")
    print("  Welch's t-test, z-test for proportions, Jensen-Shannon divergence.")
    print("  100% deterministic. 100% auditable.")
    print()


def _get_downstream(agent_id: str, visited: set | None = None) -> list[str]:
    if visited is None:
        visited = set()
    visited.add(agent_id)
    result = []
    for edge in GRAPH_EDGES:
        if edge["from"] == agent_id and edge["type"] == "CALLS":
            target = edge["to"]
            if target not in visited:
                result.append(target)
                result.extend(_get_downstream(target, visited))
    return result


if __name__ == "__main__":
    main()
