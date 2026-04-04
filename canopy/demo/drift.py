"""
Canopy Statistical Drift Detection

Compares current behavioral fingerprint against baseline using proper
statistical tests. No heuristics, no magic thresholds — real hypothesis testing.

Tests used:
  - Welch's t-test: For continuous signals (tokens, latency, cost)
    "Is the current mean significantly different from baseline?"
  - Z-test for proportions: For rates (error_rate)
    "Is the current proportion significantly different from baseline?"
  - Jensen-Shannon divergence: For distributions (tool usage)
    "Has the distribution shape changed?"

All deterministic. All auditable. No AI.
"""

import math
from dataclasses import dataclass
from enum import Enum

from fingerprint import Fingerprint


class Severity(Enum):
    INFO = "info"
    WARNING = "warning"
    CRITICAL = "critical"


@dataclass
class DriftSignal:
    """A single detected drift in one metric."""
    agent_id: str
    metric: str           # e.g. "output_tokens_mean", "error_rate", "latency_p90_ms"
    baseline_value: float
    current_value: float
    change_pct: float     # percentage change
    p_value: float        # statistical significance
    test_used: str        # "welch_t", "z_proportion", "jensen_shannon"
    severity: Severity
    description: str

    def to_dict(self) -> dict:
        return {
            "agent_id": self.agent_id,
            "metric": self.metric,
            "baseline": round(self.baseline_value, 4),
            "current": round(self.current_value, 4),
            "change_pct": round(self.change_pct, 2),
            "p_value": round(self.p_value, 6),
            "test": self.test_used,
            "severity": self.severity.value,
            "description": self.description,
        }


# ── Statistical Tests ──────────────────────────────────────────────────────

def welch_t_test(
    mean1: float, std1: float, n1: int,
    mean2: float, std2: float, n2: int,
) -> float:
    """Welch's t-test for unequal variances. Returns approximate p-value.

    Tests H0: mean1 == mean2 (two-tailed).
    Uses Welch-Satterthwaite approximation for degrees of freedom.
    P-value approximated via t-distribution using normal approximation
    (good enough for df > 30, which we'll have).
    """
    if n1 < 2 or n2 < 2:
        return 1.0

    # Handle zero variance
    var1 = std1 ** 2
    var2 = std2 ** 2

    se = math.sqrt(var1 / n1 + var2 / n2)
    if se < 1e-10:
        return 1.0 if abs(mean1 - mean2) < 1e-10 else 0.0

    t_stat = (mean1 - mean2) / se

    # Welch-Satterthwaite degrees of freedom
    num = (var1 / n1 + var2 / n2) ** 2
    denom = (var1 / n1) ** 2 / (n1 - 1) + (var2 / n2) ** 2 / (n2 - 1)
    if denom < 1e-10:
        return 1.0
    df = num / denom

    # Approximate p-value using normal distribution for large df
    # For small df, use a simple approximation
    if df > 30:
        # Normal approximation (good for large df)
        p = 2.0 * _normal_cdf(-abs(t_stat))
    else:
        # Use Student's t approximation
        p = 2.0 * _student_t_cdf(-abs(t_stat), df)

    return max(0.0, min(1.0, p))


def z_test_proportions(
    p1: float, n1: int,
    p2: float, n2: int,
) -> float:
    """Two-proportion z-test. Returns p-value.

    Tests H0: p1 == p2 (two-tailed).
    """
    if n1 < 5 or n2 < 5:
        return 1.0

    # Pooled proportion
    p_pool = (p1 * n1 + p2 * n2) / (n1 + n2)
    if p_pool < 1e-10 or p_pool > 1 - 1e-10:
        return 1.0 if abs(p1 - p2) < 1e-10 else 0.0

    se = math.sqrt(p_pool * (1 - p_pool) * (1 / n1 + 1 / n2))
    if se < 1e-10:
        return 1.0

    z = (p1 - p2) / se
    p = 2.0 * _normal_cdf(-abs(z))
    return max(0.0, min(1.0, p))


def jensen_shannon_divergence(p: dict[str, float], q: dict[str, float]) -> float:
    """Jensen-Shannon divergence between two distributions.

    JSD is symmetric and bounded [0, 1] (using log base 2).
    Returns 0 for identical distributions, 1 for maximally different.
    """
    all_keys = set(p.keys()) | set(q.keys())
    if not all_keys:
        return 0.0

    # Create aligned distributions with smoothing
    epsilon = 1e-10
    p_aligned = [p.get(k, 0.0) + epsilon for k in all_keys]
    q_aligned = [q.get(k, 0.0) + epsilon for k in all_keys]

    # Normalize
    p_sum = sum(p_aligned)
    q_sum = sum(q_aligned)
    p_norm = [x / p_sum for x in p_aligned]
    q_norm = [x / q_sum for x in q_aligned]

    # M = (P + Q) / 2
    m = [(pi + qi) / 2 for pi, qi in zip(p_norm, q_norm)]

    # JSD = (KL(P||M) + KL(Q||M)) / 2
    kl_pm = sum(pi * math.log2(pi / mi) for pi, mi in zip(p_norm, m) if pi > 0)
    kl_qm = sum(qi * math.log2(qi / mi) for qi, mi in zip(q_norm, m) if qi > 0)

    jsd = (kl_pm + kl_qm) / 2
    return max(0.0, min(1.0, jsd))


# ── Helper: Normal CDF approximation ──────────────────────────────────────

def _normal_cdf(x: float) -> float:
    """Standard normal CDF using Abramowitz & Stegun approximation."""
    # Coefficients
    a1, a2, a3, a4, a5 = 0.254829592, -0.284496736, 1.421413741, -1.453152027, 1.061405429
    p = 0.3275911

    sign = 1.0 if x >= 0 else -1.0
    x = abs(x) / math.sqrt(2.0)
    t = 1.0 / (1.0 + p * x)
    y = 1.0 - (((((a5 * t + a4) * t) + a3) * t + a2) * t + a1) * t * math.exp(-x * x)
    return 0.5 * (1.0 + sign * y)


def _student_t_cdf(t: float, df: float) -> float:
    """Approximate Student's t CDF using normal approximation with correction."""
    # Cornish-Fisher expansion for small df
    g1 = (t ** 3 + t) / (4 * df)
    g2 = (5 * t ** 5 + 16 * t ** 3 + 3 * t) / (96 * df ** 2)
    z = t + g1 + g2 if df > 4 else t * (1 - 1 / (4 * df))
    return _normal_cdf(z)


# ── Drift Detection Engine ────────────────────────────────────────────────

# Significance thresholds
ALPHA_WARNING = 0.05   # p < 0.05 → warning
ALPHA_CRITICAL = 0.01  # p < 0.01 → critical

# Minimum change percentages to avoid alerting on trivial shifts
MIN_CHANGE_PCT = {
    "output_tokens_mean": 10,
    "input_tokens_mean": 10,
    "latency_mean_ms": 15,
    "latency_p90_ms": 20,
    "error_rate": 50,        # error rate must change by 50% relative (e.g. 2% → 3%)
    "tool_distribution": 0,  # JSD handles this
}


def detect_drift(
    current: Fingerprint,
    baselines: list[Fingerprint],
) -> list[DriftSignal]:
    """Compare current fingerprint against baseline fingerprints.

    Args:
        current: The latest fingerprint
        baselines: Historical fingerprints to compare against

    Returns:
        List of DriftSignal for any detected drift
    """
    if not baselines or current.sample_size < 5:
        return []

    # Aggregate baseline stats
    bl_n = sum(b.sample_size for b in baselines)
    signals = []

    # ── Continuous metrics (Welch's t-test) ──

    continuous_checks = [
        ("output_tokens_mean", "output_tokens_mean", "output_tokens_stdev", "Output tokens"),
        ("input_tokens_mean", "input_tokens_mean", "input_tokens_stdev", "Input tokens"),
        ("latency_mean_ms", "latency_mean_ms", "latency_stdev_ms", "Latency"),
        ("latency_p90_ms", "latency_p90_ms", "latency_stdev_ms", "Latency p90"),
    ]

    for metric, mean_attr, std_attr, label in continuous_checks:
        # Aggregate baseline mean and std
        bl_means = [getattr(b, mean_attr) for b in baselines]
        bl_stds = [getattr(b, std_attr) for b in baselines]
        bl_sizes = [b.sample_size for b in baselines]

        # Pooled baseline mean (weighted)
        total_n = sum(bl_sizes)
        bl_mean = sum(m * n for m, n in zip(bl_means, bl_sizes)) / total_n if total_n else 0

        # Pooled baseline std (approximate: average variance weighted by n)
        bl_var = sum((s ** 2) * n for s, n in zip(bl_stds, bl_sizes)) / total_n if total_n else 0
        bl_std = math.sqrt(bl_var)

        cur_mean = getattr(current, mean_attr)
        cur_std = getattr(current, std_attr)

        # Skip if baseline is essentially zero
        if bl_mean < 1e-6:
            continue

        change_pct = ((cur_mean - bl_mean) / bl_mean) * 100
        min_pct = MIN_CHANGE_PCT.get(metric, 10)

        if abs(change_pct) < min_pct:
            continue

        p_value = welch_t_test(
            cur_mean, cur_std, current.sample_size,
            bl_mean, bl_std, total_n,
        )

        if p_value < ALPHA_WARNING:
            direction = "increased" if change_pct > 0 else "decreased"
            severity = Severity.CRITICAL if p_value < ALPHA_CRITICAL else Severity.WARNING

            signals.append(DriftSignal(
                agent_id=current.agent_id,
                metric=metric,
                baseline_value=bl_mean,
                current_value=cur_mean,
                change_pct=change_pct,
                p_value=p_value,
                test_used="welch_t",
                severity=severity,
                description=f"{label} {direction} by {abs(change_pct):.1f}% "
                           f"(baseline: {bl_mean:.1f}, current: {cur_mean:.1f}, p={p_value:.4f})",
            ))

    # ── Proportion metrics (z-test) ──

    bl_error_rates = [b.error_rate for b in baselines]
    bl_error_n = [b.sample_size for b in baselines]
    total_bl_n = sum(bl_error_n)
    bl_error_rate = sum(r * n for r, n in zip(bl_error_rates, bl_error_n)) / total_bl_n if total_bl_n else 0

    cur_error_rate = current.error_rate

    if bl_error_rate > 1e-6:
        error_change_pct = ((cur_error_rate - bl_error_rate) / bl_error_rate) * 100
    elif cur_error_rate > 0:
        error_change_pct = 100.0  # went from ~0 to something
    else:
        error_change_pct = 0.0

    if abs(error_change_pct) >= MIN_CHANGE_PCT.get("error_rate", 50):
        p_value = z_test_proportions(
            cur_error_rate, current.sample_size,
            bl_error_rate, total_bl_n,
        )

        if p_value < ALPHA_WARNING:
            direction = "increased" if error_change_pct > 0 else "decreased"
            severity = Severity.CRITICAL if p_value < ALPHA_CRITICAL else Severity.WARNING

            signals.append(DriftSignal(
                agent_id=current.agent_id,
                metric="error_rate",
                baseline_value=bl_error_rate,
                current_value=cur_error_rate,
                change_pct=error_change_pct,
                p_value=p_value,
                test_used="z_proportion",
                severity=severity,
                description=f"Error rate {direction} by {abs(error_change_pct):.1f}% "
                           f"(baseline: {bl_error_rate:.3f}, current: {cur_error_rate:.3f}, p={p_value:.4f})",
            ))

    # ── Distribution metrics (Jensen-Shannon divergence) ──

    # Aggregate baseline tool distribution
    bl_tool_dists = [b.tool_distribution for b in baselines]
    bl_tool_combined: dict[str, float] = {}
    for dist in bl_tool_dists:
        for tool, proportion in dist.items():
            bl_tool_combined[tool] = bl_tool_combined.get(tool, 0) + proportion
    # Average
    if bl_tool_dists:
        bl_tool_combined = {t: v / len(bl_tool_dists) for t, v in bl_tool_combined.items()}

    cur_tool_dist = current.tool_distribution

    if bl_tool_combined or cur_tool_dist:
        jsd = jensen_shannon_divergence(bl_tool_combined, cur_tool_dist)

        # JSD > 0.1 is notable, > 0.3 is significant
        if jsd > 0.1:
            severity = Severity.CRITICAL if jsd > 0.3 else Severity.WARNING

            # Find which tools changed most
            all_tools = set(bl_tool_combined.keys()) | set(cur_tool_dist.keys())
            tool_changes = []
            for tool in all_tools:
                bl_val = bl_tool_combined.get(tool, 0)
                cur_val = cur_tool_dist.get(tool, 0)
                if abs(bl_val - cur_val) > 0.05:
                    direction = "+" if cur_val > bl_val else "-"
                    tool_changes.append(f"{tool}: {bl_val:.2f}→{cur_val:.2f}")

            signals.append(DriftSignal(
                agent_id=current.agent_id,
                metric="tool_distribution",
                baseline_value=0.0,
                current_value=jsd,
                change_pct=jsd * 100,
                p_value=0.0,  # JSD doesn't produce p-values
                test_used="jensen_shannon",
                severity=severity,
                description=f"Tool usage distribution shifted (JSD={jsd:.3f}). "
                           f"Changes: {', '.join(tool_changes) if tool_changes else 'subtle shift across tools'}",
            ))

    return signals
