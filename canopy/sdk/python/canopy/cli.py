"""Canopy CLI: Discover, monitor, and debug your AI agent systems.

Usage:
    canopy discover     - Show all agents, models, tools, and dependencies
    canopy status       - Show current agent health and stats
    canopy changes      - Show recent changes detected across agents
    canopy graph        - Show the dependency graph
"""

import argparse
import json
import os
import sys
from datetime import datetime

import requests


CANOPY_SERVER_URL = os.getenv("CANOPY_SERVER_URL", "http://localhost:8080")


def get(path: str) -> dict:
    url = f"{CANOPY_SERVER_URL.rstrip('/')}{path}"
    try:
        resp = requests.get(url, timeout=5)
        resp.raise_for_status()
        return resp.json()
    except requests.ConnectionError:
        print(f"Error: Cannot connect to Canopy server at {CANOPY_SERVER_URL}")
        print("Is the server running? Try: docker compose up")
        sys.exit(1)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


def cmd_discover():
    """Discover all agents, their dependencies, and potential issues."""
    print("Discovering agents...\n")

    # Get agents
    data = get("/api/v1/agents")
    agents = data.get("agents", [])

    if not agents:
        print("No agents discovered yet.")
        print("Start sending LLM calls through the Canopy callback to populate.")
        return

    # Get graph
    graph = get("/api/v1/graph")
    nodes = graph.get("nodes", [])
    edges = graph.get("edges", [])

    agent_nodes = [n for n in nodes if n["type"] == "Agent"]
    model_nodes = [n for n in nodes if n["type"] == "Model"]
    tool_nodes = [n for n in nodes if n["type"] == "Tool"]

    print(f"Discovered {len(agent_nodes)} agents, {len(tool_nodes)} tools, {len(model_nodes)} models")
    print(f"{len(edges)} dependency edges mapped\n")

    # List agents
    for agent in agents:
        models = agent.get("ModelsUsed") or agent.get("models_used") or []
        tools = agent.get("ToolsUsed") or agent.get("tools_used") or []
        calls = agent.get("CallCount") or agent.get("call_count", 0)
        team = agent.get("Team") or agent.get("team", "")
        agent_id = agent.get("ID") or agent.get("id", "")

        team_str = f" [{team}]" if team else ""
        print(f"  {agent_id}{team_str}")
        print(f"    Calls: {calls}")
        if models:
            print(f"    Models: {', '.join(models)}")
        if tools:
            print(f"    Tools: {', '.join(tools)}")
        print()

    # Findings / warnings
    findings = []

    # Check for single-model dependencies
    for agent in agents:
        agent_id = agent.get("ID") or agent.get("id", "")
        models = agent.get("ModelsUsed") or agent.get("models_used") or []
        if len(models) == 1:
            findings.append(f"{agent_id} has single model dependency ({models[0]}) — no fallback")

    # Check for circular dependencies
    agent_calls = [(e["from"], e["to"]) for e in edges if e["type"] == "CALLS"]
    for from_a, to_a in agent_calls:
        for from_b, to_b in agent_calls:
            if from_a == to_b and to_a == from_b:
                findings.append(f"{from_a} ↔ {to_a} — circular dependency")

    if findings:
        print("⚠ Findings:")
        for f in findings:
            print(f"  - {f}")


def cmd_status():
    """Show current agent status."""
    data = get("/api/v1/agents")
    agents = data.get("agents", [])

    if not agents:
        print("No agents found.")
        return

    print(f"{'Agent':<30} {'Calls':>8} {'Models':>10} {'Tools':>8} {'Team':<15}")
    print("-" * 75)

    for agent in agents:
        agent_id = agent.get("ID") or agent.get("id", "")
        calls = agent.get("CallCount") or agent.get("call_count", 0)
        models = agent.get("ModelsUsed") or agent.get("models_used") or []
        tools = agent.get("ToolsUsed") or agent.get("tools_used") or []
        team = agent.get("Team") or agent.get("team", "")

        print(f"  {agent_id:<28} {calls:>8} {len(models):>10} {len(tools):>8} {team:<15}")


def cmd_changes():
    """Show recent changes detected across agents."""
    data = get("/api/v1/changes?limit=20")
    changes = data.get("changes", [])

    if not changes:
        print("No changes detected yet.")
        return

    print("Recent Changes:\n")

    for change in changes:
        ts = change.get("Timestamp") or change.get("timestamp", "")
        if isinstance(ts, str) and "T" in ts:
            try:
                dt = datetime.fromisoformat(ts.replace("Z", "+00:00"))
                ts = dt.strftime("%b %d %H:%M")
            except Exception:
                pass

        change_type = (change.get("ChangeType") or change.get("change_type", "")).upper()
        entity = change.get("EntityID") or change.get("entity_id", "")
        field = change.get("Field") or change.get("field", "")
        old_val = change.get("OldValueHash") or change.get("old_value_hash", "")
        new_val = change.get("NewValueHash") or change.get("new_value_hash", "")
        author = change.get("Author") or change.get("author", "")
        blast = change.get("BlastRadius") or change.get("blast_radius", 0)

        print(f"  {ts}  {change_type:<15} {author:<20} {entity}")
        print(f"    {field}: {old_val[:8]}... → {new_val[:8]}...")
        if blast > 0:
            print(f"    Blast radius: {blast} downstream agents")
        print()


def cmd_graph():
    """Show the dependency graph in text form."""
    graph = get("/api/v1/graph")
    nodes = graph.get("nodes", [])
    edges = graph.get("edges", [])

    if not nodes:
        print("Graph is empty. No agents discovered yet.")
        return

    print("Dependency Graph:\n")

    # Group edges by source
    by_source: dict[str, list] = {}
    for edge in edges:
        src = edge["from"]
        if src not in by_source:
            by_source[src] = []
        by_source[src].append(edge)

    for source, source_edges in sorted(by_source.items()):
        print(f"  {source}")
        for edge in source_edges:
            rel = edge["type"].replace("USES_", "→ ").replace("CALLS", "→")
            count = edge.get("call_count", 0)
            count_str = f" ({count} calls)" if count > 0 else ""
            print(f"    {rel} {edge['to']}{count_str}")
        print()


def main():
    parser = argparse.ArgumentParser(
        prog="canopy",
        description="Canopy: Change Intelligence for AI Agent Systems",
    )
    subparsers = parser.add_subparsers(dest="command")

    subparsers.add_parser("discover", help="Discover agents, dependencies, and issues")
    subparsers.add_parser("status", help="Show agent health and stats")
    subparsers.add_parser("changes", help="Show recent changes")
    subparsers.add_parser("graph", help="Show dependency graph")

    args = parser.parse_args()

    commands = {
        "discover": cmd_discover,
        "status": cmd_status,
        "changes": cmd_changes,
        "graph": cmd_graph,
    }

    if args.command in commands:
        commands[args.command]()
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
