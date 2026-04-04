"""Lightweight event emitter for signals the proxy can't capture.

Usage:
    from canopy.events import emit

    # Record agent-to-agent communication
    emit("agent_call", {"from": "support-agent", "to": "billing-agent"})

    # Record user feedback
    emit("user_feedback", {"agent": "support-agent", "type": "thumbs_up"})

    # Record data source access
    emit("data_access", {"agent": "support-agent", "source": "customers_db"})
"""

import os
from typing import Any

import requests


CANOPY_SERVER_URL = os.getenv("CANOPY_SERVER_URL", "http://localhost:8080")

_session = requests.Session()


def emit(event_type: str, data: dict[str, Any], server_url: str | None = None) -> None:
    """Emit an event to the Canopy server.

    Supported event types:
        - agent_call: {"from": "agent-a", "to": "agent-b"}
        - user_feedback: {"agent": "agent-id", "type": "thumbs_up"|"thumbs_down"}
        - data_access: {"agent": "agent-id", "source": "db-name"}
    """
    url = (server_url or CANOPY_SERVER_URL).rstrip("/")

    try:
        if event_type == "agent_call":
            _session.post(
                f"{url}/api/v1/ingest/agent-call",
                json={"from": data["from"], "to": data["to"]},
                timeout=2,
            )
        else:
            # Generic event endpoint (future)
            _session.post(
                f"{url}/api/v1/ingest/event",
                json={"type": event_type, **data},
                timeout=2,
            )
    except Exception:
        pass  # Never break the user's application
