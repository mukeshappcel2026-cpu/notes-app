"""
Canopy In-Memory Validation Server

A lightweight Python server that implements the same API as the Go backend,
using in-memory storage. This lets us validate the full detection pipeline
without Docker, Postgres, or Memgraph.

Usage:
    python demo/inmemory_server.py &    # Start server on :8080
    python demo/simulate.py              # Run demo against it
    python demo/validate_detection.py    # Run validation suite
"""

import hashlib
import json
import threading
import uuid
from collections import defaultdict
from datetime import datetime, timezone
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs


# ── In-Memory Storage ──────────────────────────────────────────────────────

class Store:
    def __init__(self):
        self.lock = threading.Lock()
        self.llm_calls = []
        self.agents = {}          # id → agent dict
        self.change_events = []
        self.graph_nodes = {}     # id → {id, type, label, team}
        self.graph_edges = []     # [{from, to, type, call_count}]

        # Change detection state
        self.last_prompt_hash = {}  # agent → hash
        self.last_model = {}        # agent → model
        self.last_temperature = {}  # agent → temp

    def ingest_llm_call(self, event: dict) -> dict:
        with self.lock:
            call_id = str(uuid.uuid4())
            event["id"] = call_id
            event["timestamp"] = event.get("timestamp", datetime.now(timezone.utc).isoformat())
            self.llm_calls.append(event)

            agent_id = event.get("agent", "unknown")
            team = event.get("team", "")
            model = event.get("model", "unknown")
            tools = event.get("tool_calls", [])

            # Upsert agent registry
            if agent_id not in self.agents:
                self.agents[agent_id] = {
                    "id": agent_id,
                    "first_seen": event["timestamp"],
                    "last_seen": event["timestamp"],
                    "team": team,
                    "call_count": 0,
                    "models_used": [],
                    "tools_used": [],
                }
            a = self.agents[agent_id]
            a["last_seen"] = event["timestamp"]
            a["call_count"] += 1
            if team:
                a["team"] = team
            if model and model not in a["models_used"]:
                a["models_used"].append(model)
            for t in tools:
                if t not in a["tools_used"]:
                    a["tools_used"].append(t)

            # Update graph
            self._ensure_node(agent_id, "Agent", agent_id, team)
            self._ensure_node(f"model:{model}", "Model", model, "")
            self._add_edge(agent_id, f"model:{model}", "USES_MODEL")
            for t in tools:
                self._ensure_node(f"tool:{t}", "Tool", t, "")
                self._add_edge(agent_id, f"tool:{t}", "USES_TOOL")

            # Change detection
            self._detect_changes(event)

            return {"status": "accepted", "id": call_id}

    def ingest_agent_call(self, from_agent: str, to_agent: str) -> dict:
        with self.lock:
            self._ensure_node(from_agent, "Agent", from_agent, "")
            self._ensure_node(to_agent, "Agent", to_agent, "")
            self._add_edge(from_agent, to_agent, "CALLS")
            return {"status": "accepted"}

    def _ensure_node(self, node_id: str, node_type: str, label: str, team: str):
        if node_id not in self.graph_nodes:
            self.graph_nodes[node_id] = {
                "id": node_id,
                "type": node_type,
                "label": label,
                "team": team,
            }

    def _add_edge(self, from_id: str, to_id: str, rel_type: str):
        for edge in self.graph_edges:
            if edge["from"] == from_id and edge["to"] == to_id and edge["type"] == rel_type:
                edge["call_count"] += 1
                return
        self.graph_edges.append({
            "from": from_id,
            "to": to_id,
            "type": rel_type,
            "call_count": 1,
        })

    def _detect_changes(self, event: dict):
        agent_id = event.get("agent", "unknown")
        prompt_hash = event.get("system_prompt_hash", "")
        model = event.get("model", "")
        temperature = event.get("temperature")

        # Prompt change
        if prompt_hash:
            if agent_id in self.last_prompt_hash and self.last_prompt_hash[agent_id] != prompt_hash:
                self._emit_change(
                    agent_id, "prompt_change", "system_prompt_hash",
                    self.last_prompt_hash[agent_id], prompt_hash,
                )
            self.last_prompt_hash[agent_id] = prompt_hash

        # Model change
        if model:
            if agent_id in self.last_model and self.last_model[agent_id] != model:
                self._emit_change(
                    agent_id, "model_change", "model",
                    self.last_model[agent_id], model,
                )
            self.last_model[agent_id] = model

        # Temperature change
        if temperature is not None:
            if agent_id in self.last_temperature and self.last_temperature[agent_id] != temperature:
                self._emit_change(
                    agent_id, "config_change", "temperature",
                    str(self.last_temperature[agent_id]), str(temperature),
                )
            self.last_temperature[agent_id] = temperature

    def _emit_change(self, agent_id: str, change_type: str, field: str, old_val: str, new_val: str):
        downstream = self._get_downstream(agent_id)
        self.change_events.append({
            "id": str(uuid.uuid4()),
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "change_type": change_type,
            "entity_type": "agent",
            "entity_id": agent_id,
            "field": field,
            "old_value_hash": old_val,
            "new_value_hash": new_val,
            "author": "proxy:runtime",
            "source": "proxy:runtime",
            "downstream_agents": downstream,
            "blast_radius": len(downstream),
        })

    def _get_downstream(self, agent_id: str, visited: set | None = None) -> list[str]:
        if visited is None:
            visited = set()
        visited.add(agent_id)
        result = []
        for edge in self.graph_edges:
            if edge["from"] == agent_id and edge["type"] == "CALLS":
                target = edge["to"]
                if target not in visited:
                    result.append(target)
                    result.extend(self._get_downstream(target, visited))
        return result

    def _get_upstream(self, agent_id: str, visited: set | None = None) -> list[str]:
        if visited is None:
            visited = set()
        visited.add(agent_id)
        result = []
        for edge in self.graph_edges:
            if edge["to"] == agent_id and edge["type"] == "CALLS":
                source = edge["from"]
                if source not in visited:
                    result.append(source)
                    result.extend(self._get_upstream(source, visited))
        return result

    def get_agents(self) -> list:
        with self.lock:
            return sorted(self.agents.values(), key=lambda a: a["call_count"], reverse=True)

    def get_agent(self, agent_id: str) -> dict | None:
        with self.lock:
            return self.agents.get(agent_id)

    def get_changes(self, limit: int = 50) -> list:
        with self.lock:
            return list(reversed(self.change_events[-limit:]))

    def get_graph(self) -> dict:
        with self.lock:
            return {
                "nodes": list(self.graph_nodes.values()),
                "edges": list(self.graph_edges),
            }

    def get_downstream(self, agent_id: str) -> list[str]:
        with self.lock:
            return self._get_downstream(agent_id)

    def get_upstream(self, agent_id: str) -> list[str]:
        with self.lock:
            return self._get_upstream(agent_id)


# ── HTTP Server ────────────────────────────────────────────────────────────

store = Store()


class CanopyHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass  # Suppress request logging

    def _json(self, data: dict, status: int = 200):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(data).encode())

    def _read_body(self) -> dict:
        length = int(self.headers.get("Content-Length", 0))
        return json.loads(self.rfile.read(length)) if length else {}

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path
        params = parse_qs(parsed.query)

        if path == "/health":
            self._json({"status": "ok"})

        elif path == "/api/v1/agents":
            self._json({"agents": store.get_agents()})

        elif path.startswith("/api/v1/agents/"):
            agent_id = path.split("/api/v1/agents/")[1]
            agent = store.get_agent(agent_id)
            if agent:
                self._json(agent)
            else:
                self._json({"error": "not found"}, 404)

        elif path == "/api/v1/changes":
            limit = int(params.get("limit", [50])[0])
            self._json({"changes": store.get_changes(limit)})

        elif path == "/api/v1/graph":
            self._json(store.get_graph())

        elif path.startswith("/api/v1/graph/downstream/"):
            agent_id = path.split("/api/v1/graph/downstream/")[1]
            downstream = store.get_downstream(agent_id)
            self._json({"agent": agent_id, "downstream": downstream, "blast_radius": len(downstream)})

        elif path.startswith("/api/v1/graph/upstream/"):
            agent_id = path.split("/api/v1/graph/upstream/")[1]
            upstream = store.get_upstream(agent_id)
            self._json({"agent": agent_id, "upstream": upstream})

        else:
            self._json({"error": "not found"}, 404)

    def do_POST(self):
        path = urlparse(self.path).path
        body = self._read_body()

        if path == "/api/v1/ingest/llm":
            if not body.get("agent"):
                self._json({"error": "agent field required"}, 400)
                return
            if not body.get("model"):
                self._json({"error": "model field required"}, 400)
                return
            result = store.ingest_llm_call(body)
            self._json(result, 202)

        elif path == "/api/v1/ingest/agent-call":
            if not body.get("from") or not body.get("to"):
                self._json({"error": "from and to required"}, 400)
                return
            result = store.ingest_agent_call(body["from"], body["to"])
            self._json(result, 202)

        else:
            self._json({"error": "not found"}, 404)


def run_server(port: int = 8080):
    server = HTTPServer(("0.0.0.0", port), CanopyHandler)
    print(f"  Canopy in-memory server running on :{port}")
    print(f"  Press Ctrl+C to stop\n")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\n  Server stopped.")
        server.server_close()


if __name__ == "__main__":
    import sys
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8080
    run_server(port)
