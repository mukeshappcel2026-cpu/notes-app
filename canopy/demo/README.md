# Canopy Demo: E-Commerce AI Support System

Simulates a realistic 5-agent e-commerce support system, injects 5 different
failure types, and shows how Canopy detects every change.

## Architecture

```
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
```

## Failure Scenarios Injected

| # | Scenario | Agent | What Changes |
|---|----------|-------|-------------|
| 1 | Silent Model Upgrade | router-agent | OpenAI bumps gpt-4o-mini version |
| 2 | Prompt Guardrail Removed | support-agent | "Never recommend competitors" deleted |
| 3 | Cost-Optimization Downgrade | billing-agent | Claude Sonnet → Haiku |
| 4 | Tool Removal (DB Migration) | product-agent | check_inventory tool removed |
| 5 | Temperature Drift | escalation-agent | Temperature 0.2 → 0.9 |

## Running

```bash
# Start Canopy
cd canopy && make up

# Run full demo (baseline → inject → detect)
python demo/simulate.py

# Run phases individually
python demo/simulate.py --phase 1   # Baseline (100 normal calls)
python demo/simulate.py --phase 2   # Inject 5 failures
python demo/simulate.py --phase 3   # Show detection results

# Use Canopy CLI to explore
canopy discover
canopy changes
canopy graph
canopy status
```
