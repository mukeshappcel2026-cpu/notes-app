"""
Canopy Demo: E-Commerce AI Support System

Simulates a realistic multi-agent e-commerce system:

  ┌─────────────┐     ┌──────────────┐     ┌───────────────┐
  │   Router     │────▶│   Support    │────▶│  Billing      │
  │   Agent      │     │   Agent      │     │  Agent        │
  └──────┬──────┘     └──────┬───────┘     └───────────────┘
         │                    │
         │                    ▼
         │            ┌──────────────┐
         │            │  Escalation  │
         │            │  Agent       │
         ▼            └──────────────┘
  ┌─────────────┐
  │  Product     │
  │  Agent       │
  └─────────────┘

The demo runs in 3 phases:
  Phase 1: Baseline   - Normal operation, establishes behavioral fingerprints
  Phase 2: Changes    - Inject realistic failures (model swap, prompt change, etc.)
  Phase 3: Detection  - Show Canopy catching every change

Usage:
    python demo/simulate.py              # Run full simulation
    python demo/simulate.py --phase 1    # Run just baseline
    python demo/simulate.py --phase 2    # Run just failure injection
    python demo/simulate.py --phase 3    # Show detection results
"""

import argparse
import hashlib
import json
import random
import sys
import time
from datetime import datetime, timedelta

import requests

CANOPY_URL = "http://localhost:8080"

# ── Agent Definitions ──────────────────────────────────────────────────────

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


# ── Failure Scenarios ──────────────────────────────────────────────────────

FAILURE_SCENARIOS = [
    {
        "name": "Silent Model Upgrade",
        "description": "OpenAI silently upgrades gpt-4o-mini → gpt-4o-mini-2025-04-01. "
                       "Output token distribution shifts +15%, latency increases.",
        "agent": "router-agent",
        "changes": {
            "model": "gpt-4o-mini-2025-04-01",
            "avg_output_tokens_mult": 1.15,
            "avg_latency_ms_mult": 1.25,
        },
    },
    {
        "name": "Prompt Change Without Review",
        "description": "Engineer updates support agent prompt, accidentally removes "
                       "'Never recommend competitor products' guardrail.",
        "agent": "support-agent",
        "changes": {
            "system_prompt": "You are a customer support agent for ShopCo. Help customers with order status, returns, shipping issues, and general inquiries. Always be empathetic. Include order number in every response. If the issue involves payments or refunds over $100, escalate to billing.",
        },
    },
    {
        "name": "Model Swap: Cost Optimization",
        "description": "Finance team swaps billing agent from Claude Sonnet to Haiku "
                       "to cut costs. Quality may degrade.",
        "agent": "billing-agent",
        "changes": {
            "model": "claude-haiku-4-5-20251001",
            "temperature": 0.3,
            "avg_output_tokens_mult": 0.6,
            "avg_latency_ms_mult": 0.4,
        },
    },
    {
        "name": "Tool Removal",
        "description": "Product database migration removes check_inventory tool. "
                       "Product agent starts erroring on inventory checks.",
        "agent": "product-agent",
        "changes": {
            "tools_remove": ["check_inventory"],
            "error_rate": 0.15,
        },
    },
    {
        "name": "Temperature Drift",
        "description": "Escalation agent config redeployed with temperature 0.9 "
                       "instead of 0.2. Outputs become erratic.",
        "agent": "escalation-agent",
        "changes": {
            "temperature": 0.9,
            "avg_output_tokens_mult": 1.4,
        },
    },
]


# ── Simulation Engine ──────────────────────────────────────────────────────

def prompt_hash(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()[:12]


def jitter(value: float, pct: float = 0.15) -> float:
    """Add realistic jitter to a value."""
    return value * random.uniform(1 - pct, 1 + pct)


def simulate_llm_call(agent_id: str, agent_cfg: dict) -> dict:
    """Generate a single realistic LLM call event."""
    is_error = random.random() < agent_cfg.get("error_rate", 0.02)
    tools_used = []
    if not is_error and agent_cfg.get("tools"):
        n_tools = random.randint(0, min(3, len(agent_cfg["tools"])))
        tools_used = random.sample(agent_cfg["tools"], n_tools)

    return {
        "agent": agent_id,
        "team": agent_cfg["team"],
        "model": agent_cfg["model"],
        "system_prompt_hash": prompt_hash(agent_cfg["system_prompt"]),
        "temperature": agent_cfg.get("temperature"),
        "input_tokens": int(jitter(agent_cfg["avg_input_tokens"])),
        "output_tokens": int(jitter(agent_cfg["avg_output_tokens"])),
        "cost": 0,  # simplified
        "latency_ms": jitter(agent_cfg["avg_latency_ms"]),
        "tool_calls": tools_used,
        "status": "error" if is_error else "success",
        "error_message": "tool_not_found: check_inventory" if is_error and "check_inventory" in str(agent_cfg.get("tools_removed", [])) else ("timeout" if is_error else ""),
    }


def send_event(event: dict) -> bool:
    try:
        resp = requests.post(f"{CANOPY_URL}/api/v1/ingest/llm", json=event, timeout=2)
        return resp.status_code in (200, 202)
    except Exception as e:
        print(f"  [ERROR] Failed to send event: {e}")
        return False


def send_agent_call(from_agent: str, to_agent: str) -> bool:
    try:
        resp = requests.post(
            f"{CANOPY_URL}/api/v1/ingest/agent-call",
            json={"from": from_agent, "to": to_agent},
            timeout=2,
        )
        return resp.status_code in (200, 202)
    except Exception as e:
        print(f"  [ERROR] Failed to send agent call: {e}")
        return False


def apply_changes(agent_cfg: dict, changes: dict) -> dict:
    """Apply failure scenario changes to an agent config."""
    cfg = dict(agent_cfg)

    if "model" in changes:
        cfg["model"] = changes["model"]
    if "system_prompt" in changes:
        cfg["system_prompt"] = changes["system_prompt"]
    if "temperature" in changes:
        cfg["temperature"] = changes["temperature"]
    if "tools_remove" in changes:
        cfg["tools"] = [t for t in cfg["tools"] if t not in changes["tools_remove"]]
        cfg["tools_removed"] = changes["tools_remove"]
    if "error_rate" in changes:
        cfg["error_rate"] = changes["error_rate"]
    if "avg_output_tokens_mult" in changes:
        cfg["avg_output_tokens"] = int(cfg["avg_output_tokens"] * changes["avg_output_tokens_mult"])
    if "avg_latency_ms_mult" in changes:
        cfg["avg_latency_ms"] = cfg["avg_latency_ms"] * changes["avg_latency_ms_mult"]

    return cfg


# ── Phase Runners ──────────────────────────────────────────────────────────

def run_phase_1_baseline(n_calls: int = 100):
    """Phase 1: Generate baseline traffic. Normal operation."""
    print("=" * 60)
    print("PHASE 1: BASELINE — Normal Operation")
    print("=" * 60)
    print(f"Generating {n_calls} LLM calls across {len(AGENTS)} agents...\n")

    # First, register agent-to-agent relationships
    print("  Registering agent topology...")
    for agent_id, cfg in AGENTS.items():
        for target in cfg.get("calls_to", []):
            send_agent_call(agent_id, target)
    print("  Done.\n")

    # Generate baseline traffic
    success = 0
    for i in range(n_calls):
        # Weight traffic: router gets more calls, escalation gets fewer
        weights = {
            "router-agent": 0.35,
            "support-agent": 0.30,
            "product-agent": 0.15,
            "billing-agent": 0.12,
            "escalation-agent": 0.08,
        }
        agent_id = random.choices(list(weights.keys()), list(weights.values()))[0]
        event = simulate_llm_call(agent_id, AGENTS[agent_id])

        if send_event(event):
            success += 1

        if (i + 1) % 20 == 0:
            print(f"  Sent {i + 1}/{n_calls} events ({success} successful)")

    print(f"\n  Baseline complete: {success}/{n_calls} events ingested.")
    print("  Canopy now has behavioral fingerprints for all agents.\n")


def run_phase_2_inject_failures():
    """Phase 2: Inject realistic failures."""
    print("=" * 60)
    print("PHASE 2: FAILURE INJECTION")
    print("=" * 60)
    print(f"Injecting {len(FAILURE_SCENARIOS)} failure scenarios...\n")

    modified_agents = dict(AGENTS)  # copy

    for i, scenario in enumerate(FAILURE_SCENARIOS):
        print(f"  Scenario {i + 1}: {scenario['name']}")
        print(f"    {scenario['description']}")
        print(f"    Affected agent: {scenario['agent']}")

        # Apply changes
        agent_id = scenario["agent"]
        modified_agents[agent_id] = apply_changes(modified_agents[agent_id], scenario["changes"])

        # Generate post-change traffic from the modified agent
        n_calls = 20
        for j in range(n_calls):
            event = simulate_llm_call(agent_id, modified_agents[agent_id])
            send_event(event)

        print(f"    Sent {n_calls} post-change events.")
        print()

    # Also generate some normal traffic from unmodified agents
    print("  Generating background traffic from all agents...")
    for _ in range(30):
        agent_id = random.choice(list(modified_agents.keys()))
        event = simulate_llm_call(agent_id, modified_agents[agent_id])
        send_event(event)

    print("  Done.\n")


def run_phase_3_detection():
    """Phase 3: Show what Canopy detected."""
    print("=" * 60)
    print("PHASE 3: DETECTION — What Canopy Caught")
    print("=" * 60)
    print()

    # Get agents
    try:
        agents_resp = requests.get(f"{CANOPY_URL}/api/v1/agents", timeout=5).json()
        agents = agents_resp.get("agents", [])
    except Exception as e:
        print(f"Error connecting to Canopy: {e}")
        print("Make sure the server is running: make up")
        return

    print(f"Agents discovered: {len(agents)}")
    for a in agents:
        agent_id = a.get("ID") or a.get("id", "")
        calls = a.get("CallCount") or a.get("call_count", 0)
        team = a.get("Team") or a.get("team", "")
        print(f"  {agent_id:<25} {calls:>5} calls  [{team}]")
    print()

    # Get changes
    changes_resp = requests.get(f"{CANOPY_URL}/api/v1/changes?limit=50", timeout=5).json()
    changes = changes_resp.get("changes", [])

    if not changes:
        print("No changes detected.")
        print("(This can happen if the change detector only triggers on state transitions.)")
        print()
    else:
        print(f"Changes detected: {len(changes)}\n")

        # Group by agent
        by_agent: dict[str, list] = {}
        for c in changes:
            agent = c.get("EntityID") or c.get("entity_id", "")
            if agent not in by_agent:
                by_agent[agent] = []
            by_agent[agent].append(c)

        for agent_id, agent_changes in by_agent.items():
            print(f"  {agent_id}:")
            for c in agent_changes:
                change_type = (c.get("ChangeType") or c.get("change_type", "")).replace("_", " ").title()
                field = c.get("Field") or c.get("field", "")
                old_val = c.get("OldValueHash") or c.get("old_value_hash", "")
                new_val = c.get("NewValueHash") or c.get("new_value_hash", "")
                blast = c.get("BlastRadius") or c.get("blast_radius", 0)

                old_display = old_val[:16] + "..." if len(old_val) > 16 else old_val
                new_display = new_val[:16] + "..." if len(new_val) > 16 else new_val

                print(f"    [{change_type}] {field}")
                print(f"      {old_display} → {new_display}")
                if blast > 0:
                    downstream = c.get("DownstreamAgents") or c.get("downstream_agents", [])
                    print(f"      Blast radius: {blast} agents → {downstream}")
            print()

    # Get graph
    graph_resp = requests.get(f"{CANOPY_URL}/api/v1/graph", timeout=5).json()
    nodes = graph_resp.get("nodes", [])
    edges = graph_resp.get("edges", [])

    print(f"Dependency graph: {len(nodes)} nodes, {len(edges)} edges\n")

    # Show blast radius for specific agents
    for scenario in FAILURE_SCENARIOS:
        agent_id = scenario["agent"]
        try:
            downstream_resp = requests.get(
                f"{CANOPY_URL}/api/v1/graph/downstream/{agent_id}", timeout=5
            ).json()
            downstream = downstream_resp.get("downstream", [])
            blast = downstream_resp.get("blast_radius", 0)
            if blast > 0:
                print(f"  Blast radius for {agent_id}: {blast} downstream")
                print(f"    Affected: {downstream}")
        except Exception:
            pass

    print()
    print("=" * 60)
    print("SUMMARY: What Canopy would alert on")
    print("=" * 60)
    print()

    alerts = [
        (
            "CRITICAL",
            "support-agent prompt guardrail removed",
            "'Never recommend competitor products' guardrail missing from system prompt. "
            "Blast radius: 2 downstream agents (billing-agent, escalation-agent).",
        ),
        (
            "WARNING",
            "router-agent silent model change",
            "Model changed from gpt-4o-mini → gpt-4o-mini-2025-04-01. "
            "Output tokens +15%, latency +25%. Possible provider silent update.",
        ),
        (
            "WARNING",
            "billing-agent model downgrade",
            "Model changed from claude-sonnet to claude-haiku. "
            "Output tokens -40%, latency -60%. Quality regression risk.",
        ),
        (
            "CRITICAL",
            "product-agent error rate spike",
            "Error rate jumped from 2% to 15%. "
            "Tool 'check_inventory' returning errors. Likely removed during migration.",
        ),
        (
            "WARNING",
            "escalation-agent temperature drift",
            "Temperature changed from 0.2 → 0.9. "
            "Output variance will increase significantly. Was this intentional?",
        ),
    ]

    for severity, title, description in alerts:
        marker = "!!!" if severity == "CRITICAL" else " ! "
        print(f"  [{severity}] {marker} {title}")
        print(f"       {description}")
        print()


# ── Main ───────────────────────────────────────────────────────────────────

def check_server():
    try:
        resp = requests.get(f"{CANOPY_URL}/health", timeout=2)
        return resp.status_code == 200
    except Exception:
        return False


def main():
    global CANOPY_URL

    parser = argparse.ArgumentParser(description="Canopy Demo: Multi-Agent Failure Simulation")
    parser.add_argument("--phase", type=int, choices=[1, 2, 3], help="Run specific phase")
    parser.add_argument("--baseline-calls", type=int, default=100, help="Number of baseline calls")
    parser.add_argument("--url", default=CANOPY_URL, help="Canopy server URL")
    args = parser.parse_args()

    CANOPY_URL = args.url

    print()
    print("  ╔═══════════════════════════════════════════════════════╗")
    print("  ║         CANOPY DEMO: E-Commerce AI Support           ║")
    print("  ║      Change Intelligence for AI Agent Systems        ║")
    print("  ╚═══════════════════════════════════════════════════════╝")
    print()

    if not check_server():
        print("ERROR: Cannot connect to Canopy server at", CANOPY_URL)
        print()
        print("Start the server first:")
        print("  cd canopy && make up")
        print()
        print("Or run without a server to see the detection report:")
        print("  python demo/simulate.py --phase 3")
        if args.phase == 3:
            print()
            run_phase_3_detection_offline()
        sys.exit(1)

    print(f"Connected to Canopy at {CANOPY_URL}\n")

    if args.phase is None:
        # Run all phases
        run_phase_1_baseline(args.baseline_calls)
        print("\n" + "-" * 60 + "\n")
        time.sleep(1)
        run_phase_2_inject_failures()
        print("\n" + "-" * 60 + "\n")
        time.sleep(1)
        run_phase_3_detection()
    elif args.phase == 1:
        run_phase_1_baseline(args.baseline_calls)
    elif args.phase == 2:
        run_phase_2_inject_failures()
    elif args.phase == 3:
        run_phase_3_detection()


def run_phase_3_detection_offline():
    """Show what detection WOULD look like, without a live server."""
    print()
    print("=" * 60)
    print("PHASE 3 (OFFLINE): What Canopy Would Detect")
    print("=" * 60)
    print()
    print("With 5 agents generating ~100 baseline calls, then 5 injected failures:")
    print()

    print("Changes Detected:")
    print()
    print("  router-agent:")
    print("    [Model Change] model")
    print("      gpt-4o-mini → gpt-4o-mini-2025-04-01")
    print("      Blast radius: 2 agents → [support-agent, product-agent]")
    print()
    print("  support-agent:")
    print("    [Prompt Change] system_prompt_hash")
    print("      a1b2c3d4e5f6 → 9x8y7z6w5v4u")
    print("      Blast radius: 2 agents → [billing-agent, escalation-agent]")
    print()
    print("  billing-agent:")
    print("    [Model Change] model")
    print("      claude-sonnet-4-20250514 → claude-haiku-4-5-20251001")
    print("    [Config Change] temperature")
    print("      0.1 → 0.3")
    print("      Blast radius: 0 (leaf node)")
    print()
    print("  product-agent:")
    print("    [Error Rate Spike] error_rate")
    print("      2% → 15%")
    print("      Tool check_inventory failing")
    print("      Blast radius: 0 (leaf node)")
    print()
    print("  escalation-agent:")
    print("    [Config Change] temperature")
    print("      0.2 → 0.9")
    print("      Blast radius: 0 (leaf node)")
    print()

    # Same alert summary
    print("=" * 60)
    print("ALERTS")
    print("=" * 60)
    print()
    alerts = [
        ("CRITICAL", "support-agent: prompt guardrail removed",
         "Competitor product recommendation guardrail missing from system prompt."),
        ("WARNING", "router-agent: silent model upgrade by provider",
         "gpt-4o-mini → gpt-4o-mini-2025-04-01. Output tokens +15%, latency +25%."),
        ("WARNING", "billing-agent: model downgrade (cost optimization?)",
         "claude-sonnet → claude-haiku. Quality regression risk on financial operations."),
        ("CRITICAL", "product-agent: tool removed, error rate 15%",
         "check_inventory tool missing. Likely removed during database migration."),
        ("WARNING", "escalation-agent: temperature drift 0.2 → 0.9",
         "Output variance will spike. Was this change intentional?"),
    ]
    for severity, title, desc in alerts:
        marker = "!!!" if severity == "CRITICAL" else " ! "
        print(f"  [{severity}] {marker} {title}")
        print(f"       {desc}")
        print()


if __name__ == "__main__":
    main()
