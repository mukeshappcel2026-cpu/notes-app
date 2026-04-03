"""LiteLLM callback for Canopy ingestion.

Usage:
    import litellm
    from canopy.callback import CanopyCallback

    litellm.callbacks = [CanopyCallback()]

    # Then every LLM call is automatically captured:
    response = litellm.completion(
        model="claude-sonnet-4-6",
        messages=[...],
        metadata={"agent": "my-agent", "team": "platform"}
    )
"""

import hashlib
import json
import os
import time
from typing import Any

import requests


CANOPY_SERVER_URL = os.getenv("CANOPY_SERVER_URL", "http://localhost:8080")


class CanopyCallback:
    """LiteLLM callback that sends every LLM call to Canopy for tracking."""

    def __init__(self, server_url: str | None = None):
        self.server_url = (server_url or CANOPY_SERVER_URL).rstrip("/")
        self._session = requests.Session()

    def log_success_event(
        self,
        kwargs: dict[str, Any],
        response_obj: Any,
        start_time: Any,
        end_time: Any,
    ) -> None:
        try:
            self._send_event(kwargs, response_obj, start_time, end_time, "success")
        except Exception:
            pass  # Never break the user's LLM call

    def log_failure_event(
        self,
        kwargs: dict[str, Any],
        response_obj: Any,
        start_time: Any,
        end_time: Any,
    ) -> None:
        try:
            error_msg = str(kwargs.get("exception", ""))
            self._send_event(
                kwargs, response_obj, start_time, end_time, "error", error_msg
            )
        except Exception:
            pass

    def _send_event(
        self,
        kwargs: dict[str, Any],
        response_obj: Any,
        start_time: Any,
        end_time: Any,
        status: str,
        error_message: str = "",
    ) -> None:
        metadata = kwargs.get("metadata", {}) or {}
        messages = kwargs.get("messages", [])

        # Extract system prompt and hash it
        system_prompt = ""
        for msg in messages:
            if msg.get("role") == "system":
                system_prompt = msg.get("content", "")
                break
        system_prompt_hash = (
            hashlib.sha256(system_prompt.encode()).hexdigest()[:12]
            if system_prompt
            else ""
        )

        # Extract tool calls from response
        tool_calls = []
        if response_obj and hasattr(response_obj, "choices"):
            for choice in response_obj.choices:
                if hasattr(choice, "message") and hasattr(
                    choice.message, "tool_calls"
                ):
                    if choice.message.tool_calls:
                        for tc in choice.message.tool_calls:
                            tool_calls.append(tc.function.name)

        # Calculate latency
        latency_ms = 0.0
        if start_time and end_time:
            if hasattr(start_time, "timestamp"):
                latency_ms = (
                    end_time.timestamp() - start_time.timestamp()
                ) * 1000
            else:
                latency_ms = (end_time - start_time) * 1000

        # Extract token usage
        input_tokens = 0
        output_tokens = 0
        if response_obj and hasattr(response_obj, "usage"):
            input_tokens = getattr(response_obj.usage, "prompt_tokens", 0) or 0
            output_tokens = (
                getattr(response_obj.usage, "completion_tokens", 0) or 0
            )

        event = {
            "timestamp": (
                start_time.isoformat()
                if hasattr(start_time, "isoformat")
                else str(start_time)
            ),
            "agent": metadata.get("agent", "unknown"),
            "team": metadata.get("team", ""),
            "model": kwargs.get("model", "unknown"),
            "provider": kwargs.get("custom_llm_provider", ""),
            "system_prompt_hash": system_prompt_hash,
            "temperature": kwargs.get("temperature"),
            "max_tokens": kwargs.get("max_tokens"),
            "input_tokens": input_tokens,
            "output_tokens": output_tokens,
            "cost": kwargs.get("response_cost", 0) or 0,
            "latency_ms": latency_ms,
            "tool_calls": tool_calls,
            "status": status,
            "error_message": error_message,
            "metadata": {
                k: v
                for k, v in metadata.items()
                if k not in ("agent", "team")
            },
        }

        self._session.post(
            f"{self.server_url}/api/v1/ingest/llm",
            json=event,
            timeout=2,
        )
