"""
Canopy Validation: Prove change detection works end-to-end.

This script tests the detection logic WITHOUT requiring Docker/servers.
It sends events to a running Canopy server and verifies changes are detected.

Usage:
    # With server running:
    python demo/validate_detection.py

    # Dry run (print what would happen):
    python demo/validate_detection.py --dry-run
"""

import hashlib
import json
import sys
import time

import requests

CANOPY_URL = "http://localhost:8080"


def prompt_hash(text: str) -> str:
    return hashlib.sha256(text.encode()).hexdigest()[:12]


def send(path: str, data: dict) -> dict | None:
    try:
        resp = requests.post(f"{CANOPY_URL}{path}", json=data, timeout=3)
        return {"status": resp.status_code, "body": resp.json()}
    except Exception as e:
        return {"status": 0, "error": str(e)}


def get(path: str) -> dict | None:
    try:
        resp = requests.get(f"{CANOPY_URL}{path}", timeout=3)
        return resp.json()
    except Exception as e:
        return None


class TestResult:
    def __init__(self, name: str):
        self.name = name
        self.passed = False
        self.message = ""

    def ok(self, msg: str = ""):
        self.passed = True
        self.message = msg

    def fail(self, msg: str):
        self.passed = False
        self.message = msg


def test_health() -> TestResult:
    """Test 1: Server is reachable."""
    t = TestResult("Server Health")
    try:
        resp = requests.get(f"{CANOPY_URL}/health", timeout=2)
        if resp.status_code == 200:
            t.ok()
        else:
            t.fail(f"Status {resp.status_code}")
    except Exception as e:
        t.fail(f"Cannot connect: {e}")
    return t


def test_ingest_llm_call() -> TestResult:
    """Test 2: Can ingest an LLM call."""
    t = TestResult("LLM Call Ingestion")
    event = {
        "agent": "test-agent-validation",
        "team": "test",
        "model": "test-model-v1",
        "system_prompt_hash": prompt_hash("test prompt v1"),
        "temperature": 0.5,
        "input_tokens": 100,
        "output_tokens": 50,
        "cost": 0.001,
        "latency_ms": 500,
        "tool_calls": ["test_tool"],
        "status": "success",
    }
    result = send("/api/v1/ingest/llm", event)
    if result and result["status"] in (200, 202):
        t.ok()
    else:
        t.fail(f"Ingestion failed: {result}")
    return t


def test_agent_registered() -> TestResult:
    """Test 3: Agent appears in registry after ingestion."""
    t = TestResult("Agent Registration")
    data = get("/api/v1/agents")
    if not data:
        t.fail("Could not fetch agents")
        return t

    agents = data.get("agents", [])
    found = any(
        (a.get("ID") or a.get("id", "")) == "test-agent-validation"
        for a in agents
    )
    if found:
        t.ok("test-agent-validation found in registry")
    else:
        t.fail(f"test-agent-validation not found in {len(agents)} agents")
    return t


def test_graph_populated() -> TestResult:
    """Test 4: Graph contains agent and model nodes."""
    t = TestResult("Graph Population")
    data = get("/api/v1/graph")
    if not data:
        t.fail("Could not fetch graph")
        return t

    nodes = data.get("nodes", [])
    edges = data.get("edges", [])

    agent_nodes = [n for n in nodes if n.get("type") == "Agent"]
    model_nodes = [n for n in nodes if n.get("type") == "Model"]

    if agent_nodes and model_nodes and edges:
        t.ok(f"{len(agent_nodes)} agents, {len(model_nodes)} models, {len(edges)} edges")
    else:
        t.fail(f"Incomplete graph: {len(agent_nodes)} agents, {len(model_nodes)} models, {len(edges)} edges")
    return t


def test_model_change_detected() -> TestResult:
    """Test 5: Changing the model triggers a change event."""
    t = TestResult("Model Change Detection")

    # Send a call with a DIFFERENT model for the same agent
    event = {
        "agent": "test-agent-validation",
        "team": "test",
        "model": "test-model-v2-changed",
        "system_prompt_hash": prompt_hash("test prompt v1"),
        "temperature": 0.5,
        "input_tokens": 100,
        "output_tokens": 50,
        "cost": 0.001,
        "latency_ms": 500,
        "tool_calls": ["test_tool"],
        "status": "success",
    }
    result = send("/api/v1/ingest/llm", event)
    if not result or result["status"] not in (200, 202):
        t.fail(f"Could not send changed event: {result}")
        return t

    # Check for change events
    time.sleep(0.5)
    data = get("/api/v1/changes?limit=50")
    if not data:
        t.fail("Could not fetch changes")
        return t

    changes = data.get("changes", [])
    model_changes = [
        c for c in changes
        if ((c.get("EntityID") or c.get("entity_id", "")) == "test-agent-validation"
            and (c.get("Field") or c.get("field", "")) == "model")
    ]

    if model_changes:
        c = model_changes[0]
        old = c.get("OldValueHash") or c.get("old_value_hash", "")
        new = c.get("NewValueHash") or c.get("new_value_hash", "")
        t.ok(f"Detected: {old} → {new}")
    else:
        t.fail(f"No model change detected in {len(changes)} change events")
    return t


def test_prompt_change_detected() -> TestResult:
    """Test 6: Changing the system prompt triggers a change event."""
    t = TestResult("Prompt Change Detection")

    event = {
        "agent": "test-agent-validation",
        "team": "test",
        "model": "test-model-v2-changed",
        "system_prompt_hash": prompt_hash("COMPLETELY DIFFERENT PROMPT"),
        "temperature": 0.5,
        "input_tokens": 100,
        "output_tokens": 50,
        "cost": 0.001,
        "latency_ms": 500,
        "tool_calls": ["test_tool"],
        "status": "success",
    }
    result = send("/api/v1/ingest/llm", event)
    if not result or result["status"] not in (200, 202):
        t.fail(f"Could not send changed event: {result}")
        return t

    time.sleep(0.5)
    data = get("/api/v1/changes?limit=50")
    if not data:
        t.fail("Could not fetch changes")
        return t

    changes = data.get("changes", [])
    prompt_changes = [
        c for c in changes
        if ((c.get("EntityID") or c.get("entity_id", "")) == "test-agent-validation"
            and (c.get("Field") or c.get("field", "")) == "system_prompt_hash")
    ]

    if prompt_changes:
        t.ok("Prompt hash change detected")
    else:
        t.fail(f"No prompt change detected in {len(changes)} change events")
    return t


def test_agent_call_graph() -> TestResult:
    """Test 7: Agent-to-agent calls appear in graph."""
    t = TestResult("Agent Call Graph")

    send("/api/v1/ingest/agent-call", {"from": "test-agent-validation", "to": "test-downstream-agent"})
    time.sleep(0.5)

    data = get("/api/v1/graph/downstream/test-agent-validation")
    if not data:
        t.fail("Could not fetch downstream")
        return t

    downstream = data.get("downstream", [])
    if "test-downstream-agent" in downstream:
        t.ok(f"Downstream: {downstream}")
    else:
        t.fail(f"test-downstream-agent not in downstream: {downstream}")
    return t


def test_blast_radius() -> TestResult:
    """Test 8: Blast radius calculation works."""
    t = TestResult("Blast Radius")

    data = get("/api/v1/graph/downstream/test-agent-validation")
    if not data:
        t.fail("Could not fetch downstream")
        return t

    blast = data.get("blast_radius", 0)
    if blast >= 1:
        t.ok(f"Blast radius: {blast}")
    else:
        t.fail(f"Expected blast_radius >= 1, got {blast}")
    return t


def run_all(dry_run: bool = False):
    print()
    print("  ╔═══════════════════════════════════════════════════════╗")
    print("  ║       CANOPY VALIDATION SUITE                        ║")
    print("  ║       Testing change detection end-to-end            ║")
    print("  ╚═══════════════════════════════════════════════════════╝")
    print()

    if dry_run:
        print("  [DRY RUN] Would run 8 tests against", CANOPY_URL)
        print()
        tests = [
            "1. Server Health — GET /health",
            "2. LLM Call Ingestion — POST event for test-agent-validation",
            "3. Agent Registration — verify agent appears in GET /agents",
            "4. Graph Population — verify nodes/edges in GET /graph",
            "5. Model Change Detection — swap model, check /changes",
            "6. Prompt Change Detection — change prompt hash, check /changes",
            "7. Agent Call Graph — register a→b call, check /downstream",
            "8. Blast Radius — verify downstream count >= 1",
        ]
        for t in tests:
            print(f"    {t}")
        print()
        return

    tests = [
        test_health,
        test_ingest_llm_call,
        test_agent_registered,
        test_graph_populated,
        test_model_change_detected,
        test_prompt_change_detected,
        test_agent_call_graph,
        test_blast_radius,
    ]

    results = []
    for test_fn in tests:
        result = test_fn()
        results.append(result)
        status = "PASS" if result.passed else "FAIL"
        icon = " + " if result.passed else " X "
        msg = f" — {result.message}" if result.message else ""
        print(f"  [{status}]{icon}{result.name}{msg}")

        if not result.passed and result.name == "Server Health":
            print()
            print("  Server not running. Start with: cd canopy && make up")
            print("  Or run: python demo/validate_detection.py --dry-run")
            sys.exit(1)

    passed = sum(1 for r in results if r.passed)
    total = len(results)
    print()
    print(f"  Results: {passed}/{total} passed")

    if passed == total:
        print("  All validations passed. Canopy change detection is working.")
    else:
        print("  Some tests failed. Check server logs: make logs")
    print()

    sys.exit(0 if passed == total else 1)


if __name__ == "__main__":
    dry_run = "--dry-run" in sys.argv
    run_all(dry_run)
