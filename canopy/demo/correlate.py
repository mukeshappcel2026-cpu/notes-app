"""
Canopy Root Cause Correlation Engine

The money feature. When drift is detected for an agent, this engine:
1. Finds all recent changes (for the agent, its upstream, its models, its tools)
2. Scores each change by temporal proximity and graph distance
3. Ranks probable root causes
4. Produces a human-readable diagnosis

No LLM needed. This is graph traversal + temporal correlation.
"""

import math
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any

from drift import DriftSignal, Severity


@dataclass
class RootCauseCandidate:
    """A potential root cause for observed drift."""
    change_event: dict           # The change event
    score: float                 # 0.0 to 1.0 confidence
    temporal_score: float        # How close in time
    graph_score: float           # How close in the graph
    relevance_score: float       # How relevant the change type is to the drift
    explanation: str             # Human-readable explanation

    def to_dict(self) -> dict:
        return {
            "change": {
                "type": self.change_event.get("change_type", ""),
                "entity": self.change_event.get("entity_id", ""),
                "field": self.change_event.get("field", ""),
                "old": self.change_event.get("old_value_hash", "")[:16],
                "new": self.change_event.get("new_value_hash", "")[:16],
            },
            "score": round(self.score, 3),
            "components": {
                "temporal": round(self.temporal_score, 3),
                "graph": round(self.graph_score, 3),
                "relevance": round(self.relevance_score, 3),
            },
            "explanation": self.explanation,
        }


@dataclass
class Diagnosis:
    """Complete diagnosis for a drift event."""
    agent_id: str
    drift_signals: list[DriftSignal]
    probable_causes: list[RootCauseCandidate]
    summary: str
    severity: Severity

    def to_dict(self) -> dict:
        return {
            "agent_id": self.agent_id,
            "severity": self.severity.value,
            "summary": self.summary,
            "drift_signals": [s.to_dict() for s in self.drift_signals],
            "probable_causes": [c.to_dict() for c in self.probable_causes],
        }


# ── Relevance Matrix ──────────────────────────────────────────────────────
# Maps (change_type, drift_metric) → relevance score
# Higher means this type of change is more likely to cause this type of drift

RELEVANCE_MATRIX: dict[tuple[str, str], float] = {
    # Model changes affect everything
    ("model_change", "output_tokens_mean"): 0.95,
    ("model_change", "input_tokens_mean"): 0.5,
    ("model_change", "latency_mean_ms"): 0.9,
    ("model_change", "latency_p90_ms"): 0.9,
    ("model_change", "error_rate"): 0.7,
    ("model_change", "tool_distribution"): 0.6,

    # Prompt changes primarily affect output patterns
    ("prompt_change", "output_tokens_mean"): 0.8,
    ("prompt_change", "input_tokens_mean"): 0.3,
    ("prompt_change", "latency_mean_ms"): 0.5,
    ("prompt_change", "latency_p90_ms"): 0.5,
    ("prompt_change", "error_rate"): 0.4,
    ("prompt_change", "tool_distribution"): 0.7,

    # Config changes (temperature, max_tokens) affect output variability
    ("config_change", "output_tokens_mean"): 0.7,
    ("config_change", "input_tokens_mean"): 0.1,
    ("config_change", "latency_mean_ms"): 0.4,
    ("config_change", "latency_p90_ms"): 0.5,
    ("config_change", "error_rate"): 0.3,
    ("config_change", "tool_distribution"): 0.4,

    # Tool changes primarily affect tool distribution and errors
    ("tool_change", "output_tokens_mean"): 0.3,
    ("tool_change", "input_tokens_mean"): 0.2,
    ("tool_change", "latency_mean_ms"): 0.4,
    ("tool_change", "latency_p90_ms"): 0.4,
    ("tool_change", "error_rate"): 0.9,
    ("tool_change", "tool_distribution"): 0.95,

    # Provider changes affect latency and output patterns
    ("provider_change", "output_tokens_mean"): 0.8,
    ("provider_change", "input_tokens_mean"): 0.2,
    ("provider_change", "latency_mean_ms"): 0.95,
    ("provider_change", "latency_p90_ms"): 0.95,
    ("provider_change", "error_rate"): 0.6,
    ("provider_change", "tool_distribution"): 0.3,
}

DEFAULT_RELEVANCE = 0.3


def _temporal_score(change_time: datetime, drift_time: datetime, lookback_hours: float = 24.0) -> float:
    """Score based on temporal proximity. More recent changes score higher.

    Uses exponential decay: score = exp(-hours_ago / half_life)
    Half-life of 4 hours: a change 4 hours ago scores 0.5.
    """
    if isinstance(change_time, str):
        change_time = datetime.fromisoformat(change_time.replace("Z", "+00:00"))
    if isinstance(drift_time, str):
        drift_time = datetime.fromisoformat(drift_time.replace("Z", "+00:00"))

    # Make both offset-aware or both naive
    if change_time.tzinfo is None:
        change_time = change_time.replace(tzinfo=timezone.utc)
    if drift_time.tzinfo is None:
        drift_time = drift_time.replace(tzinfo=timezone.utc)

    hours_ago = (drift_time - change_time).total_seconds() / 3600.0

    if hours_ago < 0:
        return 0.0  # Change happened after drift? Not a cause.
    if hours_ago > lookback_hours:
        return 0.0  # Too old

    half_life = 4.0  # hours
    return math.exp(-hours_ago * math.log(2) / half_life)


def _graph_score(
    change_entity: str,
    drift_agent: str,
    graph_edges: list[dict],
    agents: dict,
) -> float:
    """Score based on graph distance between change entity and drifting agent.

    Direct dependency: 1.0
    1 hop: 0.7
    2 hops: 0.5
    3+ hops: 0.3
    Unrelated: 0.1
    """
    if change_entity == drift_agent:
        return 1.0

    # BFS to find shortest path
    visited = {change_entity}
    frontier = [change_entity]
    depth = 0
    max_depth = 5

    while frontier and depth < max_depth:
        depth += 1
        next_frontier = []
        for node in frontier:
            for edge in graph_edges:
                # Check both directions since changes propagate downstream
                target = None
                if edge.get("from") == node and edge.get("type") == "CALLS":
                    target = edge["to"]
                elif edge.get("to") == node and edge.get("type") == "CALLS":
                    target = edge["from"]

                if target and target not in visited:
                    if target == drift_agent:
                        return max(0.3, 1.0 - (depth - 1) * 0.3)
                    visited.add(target)
                    next_frontier.append(target)
        frontier = next_frontier

    return 0.1  # Not connected


def _relevance_score(change_type: str, drift_metrics: list[str]) -> float:
    """How relevant is this change type to the observed drift metrics?"""
    if not drift_metrics:
        return DEFAULT_RELEVANCE

    scores = []
    for metric in drift_metrics:
        key = (change_type, metric)
        scores.append(RELEVANCE_MATRIX.get(key, DEFAULT_RELEVANCE))

    return max(scores)  # Use the highest relevance across all drift signals


def correlate(
    drift_agent: str,
    drift_signals: list[DriftSignal],
    change_events: list[dict],
    graph_edges: list[dict],
    agents: dict,
    drift_time: datetime | None = None,
) -> Diagnosis:
    """Correlate drift signals with change events to find root causes.

    Args:
        drift_agent: The agent experiencing drift
        drift_signals: Detected drift signals for this agent
        change_events: All recent change events
        graph_edges: Graph edge list from the in-memory store
        agents: Agent registry dict
        drift_time: When the drift was detected (defaults to now)

    Returns:
        Diagnosis with ranked probable causes
    """
    if drift_time is None:
        drift_time = datetime.now(timezone.utc)

    drift_metrics = [s.metric for s in drift_signals]
    candidates = []

    for change in change_events:
        change_entity = change.get("entity_id", "")
        change_type = change.get("change_type", "")
        change_time = change.get("timestamp", "")

        t_score = _temporal_score(change_time, drift_time)
        if t_score < 0.01:
            continue  # Too old, skip

        g_score = _graph_score(change_entity, drift_agent, graph_edges, agents)
        r_score = _relevance_score(change_type, drift_metrics)

        # Combined score: weighted geometric mean
        # Temporal: 30%, Graph: 35%, Relevance: 35%
        combined = (t_score ** 0.30) * (g_score ** 0.35) * (r_score ** 0.35)

        # Build explanation
        explanation = _build_explanation(change, drift_agent, drift_signals, g_score)

        candidates.append(RootCauseCandidate(
            change_event=change,
            score=combined,
            temporal_score=t_score,
            graph_score=g_score,
            relevance_score=r_score,
            explanation=explanation,
        ))

    # Sort by score descending
    candidates.sort(key=lambda c: c.score, reverse=True)

    # Take top 5
    top_causes = candidates[:5]

    # Determine overall severity
    if drift_signals:
        worst = max(drift_signals, key=lambda s: 0 if s.severity == Severity.INFO else (1 if s.severity == Severity.WARNING else 2))
        severity = worst.severity
    else:
        severity = Severity.INFO

    summary = _build_summary(drift_agent, drift_signals, top_causes)

    return Diagnosis(
        agent_id=drift_agent,
        drift_signals=drift_signals,
        probable_causes=top_causes,
        summary=summary,
        severity=severity,
    )


def _build_explanation(
    change: dict,
    drift_agent: str,
    drift_signals: list[DriftSignal],
    graph_score: float,
) -> str:
    """Build a human-readable explanation for why this change might be the cause."""
    change_type = change.get("change_type", "unknown")
    change_entity = change.get("entity_id", "unknown")
    field = change.get("field", "")
    old_val = change.get("old_value_hash", "")[:20]
    new_val = change.get("new_value_hash", "")[:20]

    # Direct vs indirect
    if change_entity == drift_agent:
        relation = f"{change_entity}'s own {field}"
    elif graph_score >= 0.7:
        relation = f"direct upstream dependency {change_entity}'s {field}"
    elif graph_score >= 0.4:
        relation = f"transitive dependency {change_entity}'s {field}"
    else:
        relation = f"possibly related change in {change_entity}'s {field}"

    # What drifted
    drift_desc = ", ".join(s.metric.replace("_", " ") for s in drift_signals[:3])

    type_labels = {
        "model_change": "model was changed",
        "prompt_change": "system prompt was modified",
        "config_change": "configuration was changed",
        "tool_change": "tool was added/removed",
        "provider_change": "provider was changed",
    }
    action = type_labels.get(change_type, f"{change_type} occurred")

    return f"{relation}: {action} ({old_val} → {new_val}). This correlates with drift in {drift_desc}."


def _build_summary(
    drift_agent: str,
    drift_signals: list[DriftSignal],
    top_causes: list[RootCauseCandidate],
) -> str:
    """Build a concise summary of the diagnosis."""
    if not drift_signals:
        return f"No significant drift detected for {drift_agent}."

    # Describe what drifted
    drift_parts = []
    for s in drift_signals[:3]:
        drift_parts.append(f"{s.metric.replace('_', ' ')} {'+' if s.change_pct > 0 else ''}{s.change_pct:.0f}%")
    drift_desc = ", ".join(drift_parts)

    if not top_causes:
        return f"{drift_agent} shows drift ({drift_desc}) but no correlated changes found."

    # Describe the top cause
    top = top_causes[0]
    cause_entity = top.change_event.get("entity_id", "unknown")
    cause_type = top.change_event.get("change_type", "").replace("_", " ")
    cause_field = top.change_event.get("field", "")

    confidence = "high" if top.score > 0.7 else ("medium" if top.score > 0.4 else "low")

    if cause_entity == drift_agent:
        cause_desc = f"own {cause_type} ({cause_field})"
    else:
        cause_desc = f"{cause_type} in {cause_entity} ({cause_field})"

    summary = f"{drift_agent}: {drift_desc}. Probable cause ({confidence} confidence): {cause_desc}."

    if len(top_causes) > 1:
        summary += f" {len(top_causes) - 1} additional possible cause(s)."

    return summary
