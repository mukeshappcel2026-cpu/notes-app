"""
Canopy Behavioral Fingerprinting Engine

Computes statistical fingerprints for each agent over time windows.
A fingerprint captures "how this agent normally behaves" — token distributions,
latency profiles, error rates, tool usage patterns.

This is pure math. No LLM. No AI judging AI.
"""

import math
import statistics
from dataclasses import dataclass, field
from datetime import datetime, timedelta, timezone
from typing import Any


@dataclass
class Fingerprint:
    """Statistical snapshot of agent behavior over a time window."""
    agent_id: str
    window_start: datetime
    window_end: datetime
    sample_size: int

    # Token distributions
    input_tokens_mean: float = 0.0
    input_tokens_stdev: float = 0.0
    output_tokens_mean: float = 0.0
    output_tokens_stdev: float = 0.0
    output_tokens_p50: float = 0.0
    output_tokens_p90: float = 0.0
    output_tokens_p99: float = 0.0

    # Latency
    latency_mean_ms: float = 0.0
    latency_stdev_ms: float = 0.0
    latency_p50_ms: float = 0.0
    latency_p90_ms: float = 0.0
    latency_p99_ms: float = 0.0

    # Rates (proportions)
    error_rate: float = 0.0
    error_count: int = 0
    success_count: int = 0

    # Tool usage distribution: tool_name → proportion of calls using it
    tool_distribution: dict[str, float] = field(default_factory=dict)

    # Cost
    cost_mean: float = 0.0
    cost_total: float = 0.0

    def to_dict(self) -> dict:
        return {
            "agent_id": self.agent_id,
            "window_start": self.window_start.isoformat(),
            "window_end": self.window_end.isoformat(),
            "sample_size": self.sample_size,
            "input_tokens": {"mean": self.input_tokens_mean, "stdev": self.input_tokens_stdev},
            "output_tokens": {
                "mean": self.output_tokens_mean, "stdev": self.output_tokens_stdev,
                "p50": self.output_tokens_p50, "p90": self.output_tokens_p90,
                "p99": self.output_tokens_p99,
            },
            "latency_ms": {
                "mean": self.latency_mean_ms, "stdev": self.latency_stdev_ms,
                "p50": self.latency_p50_ms, "p90": self.latency_p90_ms,
                "p99": self.latency_p99_ms,
            },
            "error_rate": self.error_rate,
            "tool_distribution": self.tool_distribution,
            "cost": {"mean": self.cost_mean, "total": self.cost_total},
        }


def percentile(data: list[float], pct: float) -> float:
    """Calculate percentile. pct is 0-100."""
    if not data:
        return 0.0
    sorted_data = sorted(data)
    k = (len(sorted_data) - 1) * pct / 100.0
    f = math.floor(k)
    c = math.ceil(k)
    if f == c:
        return sorted_data[int(k)]
    return sorted_data[f] * (c - k) + sorted_data[c] * (k - f)


def compute_fingerprint(
    agent_id: str,
    calls: list[dict],
    window_start: datetime,
    window_end: datetime,
) -> Fingerprint | None:
    """Compute a behavioral fingerprint from a set of LLM calls.

    Args:
        agent_id: The agent to fingerprint
        calls: List of LLM call dicts for this agent within the window
        window_start: Start of the observation window
        window_end: End of the observation window

    Returns:
        Fingerprint or None if insufficient data (< 5 calls)
    """
    # Filter to this agent and window
    agent_calls = [
        c for c in calls
        if c.get("agent") == agent_id
    ]

    if len(agent_calls) < 5:
        return None

    n = len(agent_calls)

    # Extract numeric arrays
    input_tokens = [c.get("input_tokens", 0) for c in agent_calls]
    output_tokens = [c.get("output_tokens", 0) for c in agent_calls]
    latencies = [c.get("latency_ms", 0) for c in agent_calls]
    costs = [c.get("cost", 0) or 0 for c in agent_calls]

    # Error rate
    errors = sum(1 for c in agent_calls if c.get("status") != "success")
    successes = n - errors

    # Tool distribution
    tool_counts: dict[str, int] = {}
    for c in agent_calls:
        for tool in c.get("tool_calls", []):
            tool_counts[tool] = tool_counts.get(tool, 0) + 1
    tool_dist = {t: count / n for t, count in tool_counts.items()}

    # Build fingerprint
    fp = Fingerprint(
        agent_id=agent_id,
        window_start=window_start,
        window_end=window_end,
        sample_size=n,
    )

    # Token stats
    fp.input_tokens_mean = statistics.mean(input_tokens)
    fp.input_tokens_stdev = statistics.stdev(input_tokens) if n > 1 else 0.0
    fp.output_tokens_mean = statistics.mean(output_tokens)
    fp.output_tokens_stdev = statistics.stdev(output_tokens) if n > 1 else 0.0
    fp.output_tokens_p50 = percentile(output_tokens, 50)
    fp.output_tokens_p90 = percentile(output_tokens, 90)
    fp.output_tokens_p99 = percentile(output_tokens, 99)

    # Latency stats
    fp.latency_mean_ms = statistics.mean(latencies)
    fp.latency_stdev_ms = statistics.stdev(latencies) if n > 1 else 0.0
    fp.latency_p50_ms = percentile(latencies, 50)
    fp.latency_p90_ms = percentile(latencies, 90)
    fp.latency_p99_ms = percentile(latencies, 99)

    # Rates
    fp.error_rate = errors / n
    fp.error_count = errors
    fp.success_count = successes

    # Tool distribution
    fp.tool_distribution = tool_dist

    # Cost
    fp.cost_mean = statistics.mean(costs) if costs else 0.0
    fp.cost_total = sum(costs)

    return fp


class FingerprintStore:
    """Stores and retrieves fingerprints per agent."""

    def __init__(self):
        self.fingerprints: dict[str, list[Fingerprint]] = {}  # agent_id → list

    def store(self, fp: Fingerprint):
        if fp.agent_id not in self.fingerprints:
            self.fingerprints[fp.agent_id] = []
        self.fingerprints[fp.agent_id].append(fp)

    def get_latest(self, agent_id: str) -> Fingerprint | None:
        fps = self.fingerprints.get(agent_id, [])
        return fps[-1] if fps else None

    def get_baseline(self, agent_id: str, n: int = 5) -> list[Fingerprint]:
        """Get the last N fingerprints as baseline (excluding most recent)."""
        fps = self.fingerprints.get(agent_id, [])
        if len(fps) <= 1:
            return []
        return fps[-(n + 1):-1]  # exclude the latest

    def get_all(self, agent_id: str) -> list[Fingerprint]:
        return self.fingerprints.get(agent_id, [])
