# AI SBOM — Product Document

**One-liner:** A live inventory of every AI system, model, and AI-powered feature your organization uses, how data flows to each, and whether it's compliant.

---

## Problem

Enterprises have no single pane of glass for AI usage. AI is adopted bottom-up — individual developers call OpenAI APIs, teams enable Notion AI, employees expense ChatGPT Plus — and top-down visibility is zero. CISOs, compliance teams, and procurement have no way to answer:

- **What AI are we using?** (shadow AI, sanctioned AI, embedded AI features in SaaS)
- **What data flows into it?** (PII, trade secrets, regulated data)
- **Are we compliant?** (EU AI Act, GDPR, internal policy, vendor DPAs)
- **What does it cost?** (direct API spend, SaaS AI add-ons, individual subscriptions)

AI SBOM answers all four questions continuously.

---

## Data Sources

This is the moat — breadth and depth of integrations.

```
┌──────────────────────────────────────────────────────┐
│                      AI SBOM                         │
│            (unified inventory + policy)               │
├──────────────────────────────────────────────────────┤
│                                                       │
│  Network            SaaS              Code            │
│  ──────────         ──────────        ────────        │
│  DNS logs           OAuth grants      Git repos       │
│  SNI inspection     SCIM/IdP          CI/CD configs   │
│  Flow logs          SaaS APIs         package.json    │
│  Proxy logs         Marketplace       import graphs   │
│                     audit logs                        │
│                                                       │
│  Endpoint           Procurement       Cloud           │
│  ──────────         ──────────────    ────────        │
│  Browser exts       Contracts         AWS Bedrock     │
│  IDE plugins        Invoices          Azure OpenAI    │
│  Local models       Expense reports   Vertex AI       │
│  (Ollama etc)       Credit card       GCP/AWS bills   │
│                                                       │
└──────────────────────────────────────────────────────┘
```

### What Each Source Discovers

| Source | What It Finds | Example |
|---|---|---|
| **Network (DNS/SNI)** | Direct API calls to AI providers | Dev calling `api.openai.com` from a microservice |
| **OAuth/IdP** | SaaS apps with AI features granted access | "Notion AI" authorized via Okta for 340 users |
| **SaaS audit logs** | AI feature activation inside existing tools | Slack AI enabled on 12 channels |
| **Git repos** | AI SDKs in dependencies, model files, prompt templates | `anthropic` in requirements.txt across 7 repos |
| **CI/CD** | AI calls embedded in pipelines | GitHub Action using GPT-4 for PR review |
| **Browser extensions** | AI-powered extensions installed on endpoints | Grammarly, Monica, Merlin on 200 machines |
| **Cloud billing** | Managed AI service usage | $4,200/month on Bedrock nobody approved |
| **Procurement** | Contracted AI vendors | Signed contract with Jasper AI, legal hasn't reviewed data terms |
| **Expense reports** | Individual AI subscriptions | 47 employees expensing ChatGPT Plus |

---

## Core Data Model — The AI Asset

Every discovered AI usage becomes an **AI Asset**:

```json
{
  "id": "ai-asset-0042",
  "name": "OpenAI GPT-4 via payments-api",
  "category": "direct_api",
  "provider": "openai",
  "model": "gpt-4",
  "discoveredVia": ["network_dns", "git_dependency"],
  "firstSeen": "2026-01-15",
  "lastSeen": "2026-03-22",
  "owner": {
    "team": "payments",
    "contact": "jane@acme.com"
  },
  "dataFlow": {
    "inputDataTypes": ["customer_email", "dispute_text"],
    "outputUsage": "auto-generated dispute response",
    "piiExposure": "high",
    "dataResidency": "US (OpenAI)"
  },
  "compliance": {
    "approved": false,
    "euAiActRiskLevel": "high",
    "dpaInPlace": false,
    "securityReviewComplete": false,
    "policiesViolated": ["AI-POL-003", "DATA-POL-001"]
  },
  "usage": {
    "estimatedMonthlyRequests": 12000,
    "estimatedMonthlyCost": "$340",
    "activeUsers": 3
  }
}
```

### Asset Categories

| Category | Description | Example |
|---|---|---|
| `direct_api` | Team calling an AI provider's API directly | OpenAI, Anthropic, Cohere API keys in code |
| `saas_embedded` | AI feature inside a SaaS product | Notion AI, Salesforce Einstein, Slack AI |
| `managed_cloud` | AI service via cloud provider | AWS Bedrock, Azure OpenAI, Vertex AI |
| `self_hosted` | AI model running on internal infrastructure | Ollama, vLLM, TGI on internal clusters |
| `individual_subscription` | Personal AI tool paid by employee | ChatGPT Plus, Copilot, Perplexity Pro |
| `browser_extension` | AI-powered browser plugin | Grammarly, Monica, Merlin |
| `ci_cd_integration` | AI embedded in build/deploy pipelines | CodeRabbit, GPT-based PR reviewers |

---

## Why This Is Hard (and Therefore Defensible)

1. **Integration depth** — 15-20 integrations needed, each with different APIs, auth models, and data formats. Okta works differently from Azure AD. Slack audit logs look nothing like Notion's. Grinding, unsexy work that compounds.

2. **Classification** — When you see `pip install transformers` in a repo, is that AI usage? What about `import scipy`? A taxonomy of "what counts as AI" that evolves weekly as new tools ship.

3. **Entity resolution** — Same AI usage appears in 3 sources: DNS logs show calls to `api.openai.com`, Git shows `openai` in `package.json`, expense reports show a ChatGPT Team subscription. Stitching them into one asset is non-trivial.

4. **Continuous discovery** — New AI tools launch daily. An employee signs up for a new AI coding assistant on Tuesday. How fast do you detect it? The registry of "what counts as AI" is a living data asset.

---

## Go-To-Market Sequence

### Phase 1 — Discovery (3 data sources)

**Sources:** Network (DNS) + Git repos + Cloud billing

- DNS detection: already built (Shadow AI Detector)
- Git repo scanning: solved problem (clone, parse dependency files, match against AI package list)
- Cloud billing APIs: well-documented (AWS Cost Explorer, Azure Cost Management)

**Answers:** "Which teams are calling AI APIs and what's it costing us?"

**Milestone:** Enough to demo and get design partners.

### Phase 2 — SaaS Visibility

**Sources:** IdP/OAuth + SaaS audit logs

- Okta/Azure AD integration: pull all authorized third-party apps, flag ones with AI capabilities
- Start with top 10 SaaS apps that have AI features (Notion, Slack, Salesforce, etc.)

**Answers:** "Which AI-powered SaaS features are enabled in our org?"

### Phase 3 — Compliance & Monetization

**Capabilities:** Policy engine + reporting

- Define policies: "all high-risk AI must have DPA signed," "no AI with EU data unless GDPR compliant"
- Auto-evaluate every AI asset against policies
- Generate board-ready reports and EU AI Act compliance artifacts

**This is where aggressive monetization starts.**

---

## Relationship to Shadow AI Detector

The existing Shadow AI Detector (`shadow-ai-detector/`) becomes Phase 1's **network discovery module**. Specifically:

| Shadow AI Detector Component | AI SBOM Role |
|---|---|
| DNS query analysis | Network discovery integration |
| Provider registry (AI domains) | Seed data for AI provider taxonomy |
| CloudFormation deployment | Reusable infra-as-code pattern |
| Deduplication logic | Foundation for entity resolution |
| Alert system | Feeds into unified notification layer |

The standalone SaaS framing goes away. It becomes the first integration in the AI SBOM platform.

---

## Competitive Landscape

| Player | What They Do | AI SBOM Gap |
|---|---|---|
| **Reco, Grip Security** | SaaS security, some AI discovery | AI isn't their focus, no dependency scanning or compliance mapping |
| **Zylo, Productiv** | SaaS management, license tracking | Could add AI tagging but no network or code-level detection |
| **Wiz, Orca** | Cloud security | Could scan for AI services in cloud but miss SaaS and shadow usage |
| **Nobody** | All of the above stitched together | AI-specific lens + compliance mapping across all vectors |

The gap is real. The question is getting to 5-6 integrations and 3 design partners before an incumbent decides to care.

---

## Key Metrics

| Metric | What It Measures |
|---|---|
| AI assets discovered | Total inventory size — proves breadth |
| Discovery sources active | Integration depth — proves moat |
| Time to detect new AI usage | Speed — shadow AI appears, how fast do we flag it? |
| Policy violations flagged | Compliance value — justifies budget |
| Data sources per asset | Entity resolution quality — same asset seen from multiple angles |
| Coverage (% of org monitored) | Deployment depth — all teams, all infra |

---

## Pricing Model

| Tier | Target | Includes |
|---|---|---|
| **Starter** | SMB / single team | Network + Git discovery, 3 integrations, basic inventory |
| **Business** | Mid-market | All discovery sources, policy engine, compliance reports |
| **Enterprise** | Large orgs | Custom integrations, EU AI Act compliance pack, SSO, dedicated support |

Pricing anchor: per-employee/month (like SaaS security tools). AI SBOM is a compliance product — compliance budgets are large and non-discretionary.
