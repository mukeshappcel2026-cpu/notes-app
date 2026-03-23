# AI SBOM — Competitive Analysis

**Last updated:** March 2026

---

## Executive Summary

The AI security market is undergoing rapid consolidation. In the past 18 months, every major pure-play AI security startup has been acquired by a platform incumbent: **Protect AI → Palo Alto Networks (~$500M+)**, **Robust Intelligence → Cisco (~$400M)**, **Lakera → Check Point**, **Prompt Security → SentinelOne**, **Promptfoo → OpenAI**. This leaves a narrow window where incumbents are still integrating acquisitions, and a greenfield opportunity exists for a product that stitches together **multi-vector AI discovery** (network + code + SaaS + procurement) — something none of them do.

---

## Market Context

### Market Size

| Segment | 2024 | 2025 | Projected | CAGR |
|---|---|---|---|---|
| AI in Cybersecurity (broad) | $26.5B | $34.1B | $234.6B by 2032 | 31.7% |
| AI-SPM (narrow) | $1.0B | — | $1.6B by 2031 | 6.5–18.4% |
| DSPM | — | $2.1B | $10.4B by 2030 | 38.3% |
| Security Posture Mgmt (broad) | — | $26.6B | $53.3B by 2030 | 14.9% |
| InfoSec total (Gartner) | $193B | $213B | $240B by 2026 | 12.5% |

### Regulatory Drivers

- **EU AI Act** — compliance deadlines creating urgency for AI inventory and risk classification
- **NIST AI RMF** — voluntary framework but increasingly referenced in procurement requirements
- **Gartner** named "AI Security Platforms" a **Top Strategic Technology Trend for 2026**
- 86% of organizations experienced AI-related security incidents (Cisco 2025 Cybersecurity Readiness Index)
- 85% of organizations using some form of AI; 74% using managed AI services (Wiz State of AI 2025)
- Only 19% have full visibility into where and how AI is used across development (Cycode 2026)

### M&A Wave (2024–2026)

| Target | Acquirer | Price | Date |
|---|---|---|---|
| Robust Intelligence | Cisco | ~$400M | Oct 2024 |
| Protect AI | Palo Alto Networks | ~$500M+ | Jul 2025 |
| Lakera | Check Point | Undisclosed | Sep 2025 |
| Prompt Security | SentinelOne | Undisclosed | Aug 2025 |
| Promptfoo | OpenAI | Undisclosed | Mar 2026 |

**Takeaway:** The acqui-hire spree validates the category but also means these capabilities are being absorbed into large platforms that move slowly on integration. The next 12–18 months is an integration tax window.

---

## Competitor Deep Dives

### 1. Wiz AI-SPM — The Primary Threat

**What they are:** Cloud security platform (CNAPP), valued at $12B+, acquired by Google for $32B (announced 2025). Named Leader with Highest Current Offering Score in Forrester Wave CNAPP Q1 2026.

**AI-SPM Launch:** November 2023 — first CNAPP to include AI-SPM. Significantly expanded at Wizdom 2025 (November 2025).

**What they do well:**

| Capability | Details |
|---|---|
| **AI-BOM** | Agentless discovery of AI services, models, SDKs, libraries, dependencies across cloud |
| **Agent Inventory** | Visualizes agents, models, tools, MCP connections, data (training + knowledge base) |
| **Attack Surface Mapping** | Maps external-facing AI endpoints → workloads → owners via Security Graph |
| **Misconfiguration Detection** | Built-in rules for SageMaker, Vertex AI, Bedrock, Azure OpenAI misconfigs |
| **DSPM for AI** | Detects sensitive training data, flags data leakage risks |
| **Attack Path Analysis** | Links vulnerabilities, identities, network exposure, secrets to AI models |
| **Runtime Monitoring** | Detects rogue agents, suspicious DNS from AI workloads, drift from baselines |
| **MCP Discovery** | Discovers Model Context Protocol usage across environments |
| **OWASP LLM Top 10** | Flags issues mapped to OWASP LLM Top 10 |

**What they miss:**

| Gap | Why It Matters |
|---|---|
| **Cloud-only scope** | Zero visibility into SaaS-embedded AI (Notion AI, Slack AI, Salesforce Einstein), browser extensions, individual subscriptions |
| **No network-level detection** | Can't see DNS calls to `api.openai.com` from on-prem or non-cloud workloads |
| **No procurement/expense data** | Misses shadow AI purchased via credit cards, expensed individually |
| **No code dependency scanning** | Doesn't parse `requirements.txt`, `package.json` for AI SDK usage across repos |
| **Enterprise-only** | Priced for large enterprises with significant cloud footprints; SMB/mid-market gap |
| **Google acquisition** | Post-acquisition focus may shift to GCP-first; multi-cloud customers may seek alternatives |

**Threat level: HIGH** — They have the brand, the graph, and the distribution. But their AI-SPM is a feature of their CNAPP, not a standalone product. They'll never care about expense-report-level shadow AI.

---

### 2. Palo Alto Networks (Prisma AIRS + Protect AI)

**What they are:** Largest pure-play cybersecurity company. Acquired Protect AI for ~$500M+ (July 2025).

**What they offer:**

- **Prisma AIRS** — "industry's most comprehensive AI security platform," built on Protect AI acquisition
- **AI-BOM capabilities** — model scanning across 35+ formats, supply chain security for ML models
- **AI Runtime Security** — model vulnerability detection, prompt injection defense
- **Guardian** — open-source tool for scanning ML model files for malicious payloads
- **ModelScan** — supply chain security for serialized model files
- Previously acquired **IBoss** and other companies for broader security platform

**What they miss:**

| Gap | Why It Matters |
|---|---|
| **Model-focused, not discovery-focused** | Protect AI was about securing models in the pipeline, not discovering shadow AI usage across an org |
| **No SaaS/shadow AI discovery** | No equivalent of scanning OAuth grants, IdP logs, or expense reports for unsanctioned AI |
| **No network-level AI detection** | Firewalls could do this but PAN hasn't productized DNS/SNI-based AI discovery |
| **Integration tax** | Protect AI acquisition is < 1 year old; integration into Prisma platform is ongoing |
| **Complexity** | PAN products require significant deployment effort; not self-serve |

**Threat level: MEDIUM** — They have the model security story but lack the discovery/inventory lens. Could build it, but their focus is on securing the AI pipeline, not inventorying shadow AI.

---

### 3. Cisco (AI Defense + Robust Intelligence)

**What they are:** Networking/security giant. Acquired Robust Intelligence for ~$400M (Oct 2024).

**What they offer:**

- **Cisco AI Defense** — AI security at the network level, embedded in Cisco Security Cloud
- **AI Firewall** — industry's first, inherited from Robust Intelligence
- **Algorithmic Red Teaming** — automated testing of AI models
- **AI Supply Chain Risk Management** — security controls for AI artifacts
- **Foundation AI** — open-source reasoning model for security applications
- **Network-level enforcement** — can inspect AI traffic flowing through Cisco infrastructure

**What they miss:**

| Gap | Why It Matters |
|---|---|
| **Network-centric, not inventory-centric** | AI Defense is about blocking/inspecting AI traffic, not building a comprehensive AI asset inventory |
| **No SaaS discovery** | Doesn't scan OAuth, IdP, or SaaS audit logs for AI feature activation |
| **No code scanning** | Doesn't parse repos for AI dependencies |
| **No compliance/governance** | No policy engine for EU AI Act, no board-ready reporting |
| **Cisco sales motion** | Requires existing Cisco infrastructure; enterprise-only, long sales cycles |

**Threat level: MEDIUM** — The network-level AI detection is strong and overlaps with our DNS/SNI approach. But they're solving "block bad AI traffic," not "show me all AI in my org."

---

### 4. Reco — The Closest Competitor

**What they are:** SaaS security platform with strong shadow AI discovery. $85M total funding, $30M Series B (Feb 2026). 500% growth 2023–2024, 400% growth 2025.

**What they offer:**

| Capability | Details |
|---|---|
| **Shadow AI Discovery** | Detects AI tools via IdP integration (Azure AD, Okta), email metadata analysis, NLP classification |
| **AI Governance** | Discovers AI tools, agents, copilots; governs SaaS-embedded AI (Salesforce Einstein, Microsoft Copilot) |
| **App Discovery Engine** | Classifies 50,000+ applications including AI tools |
| **Identity & Access** | Consolidates identities across SaaS apps, finds over-permissioned users |
| **Threat Detection** | Alerts on data theft, account compromise, config drift |
| **MCP Monitoring** | Monitors MCP interactions within SaaS |
| **220+ SaaS integrations** | Deep integration breadth |

**What they miss:**

| Gap | Why It Matters |
|---|---|
| **No network-level detection** | Can't see direct API calls to AI providers from code/microservices |
| **No code/dependency scanning** | Doesn't parse repos for AI SDK usage |
| **No cloud billing integration** | Misses managed AI service costs (Bedrock, Vertex AI) |
| **SaaS-only lens** | Strong on SaaS shadow AI, weak on infrastructure/code-level AI usage |
| **No AI-BOM standard alignment** | No SPDX 3.0 or CycloneDX output for compliance |
| **No EU AI Act compliance** | No risk classification or regulatory reporting |

**Threat level: HIGH** — Reco is the closest to our vision on the SaaS/shadow AI side. Their growth numbers are real. But they approach from SaaS security and have no code or network detection.

---

### 5. Other Players

#### CrowdStrike (Charlotte AI + Falcon)
- Cloud security with some AI asset discovery
- Focused on threat detection, not AI inventory
- **Gap:** No dedicated AI-BOM or shadow AI discovery product

#### Orca Security
- Cloud security with AI-SPM features
- Similar scope to Wiz (cloud-only)
- **Gap:** Same cloud-only limitations as Wiz; smaller market share

#### SentinelOne (+ Prompt Security)
- Acquired Prompt Security (Aug 2025) for GenAI security
- Purple AI for threat hunting
- **Gap:** Prompt Security was about prompt injection/guardrails, not AI inventory

#### Check Point (+ Lakera)
- Acquired Lakera (Sep 2025) for AI security
- Integrated into Infinity Platform, CloudGuard WAF
- **Gap:** Lakera was prompt injection defense, not AI discovery/BOM

#### HiddenLayer (Independent)
- ML model security: adversarial attack detection, model scanning (35+ formats)
- $50M Series A led by Microsoft M12
- **Gap:** Model security only, no discovery or inventory capabilities

#### Snyk
- Developer security with some AI-BOM thought leadership
- **Gap:** No shipping AI-SPM product; focused on traditional AppSec

---

## Competitive Positioning Matrix

```
                    AI INVENTORY/DISCOVERY BREADTH
                    (how many vectors do you scan?)

              Low                                    High
         ┌─────────────────────────────────────────────┐
    High │                    │                         │
         │   Wiz              │                         │
    S    │   Orca             │      ★ AI SBOM          │
    E    │                    │      (our target)       │
    C    │   CrowdStrike      │                         │
    U    ├────────────────────┼─────────────────────────┤
    R    │                    │                         │
    I    │   HiddenLayer      │      Reco               │
    T    │   PAN/Protect AI   │                         │
    Y    │   Cisco/Robust Int │                         │
         │   SentinelOne      │                         │
    D    │   Check Point      │                         │
    E    │                    │                         │
    P    │                    │                         │
    T    │                    │                         │
    H    │                    │                         │
    Low  └─────────────────────────────────────────────┘
```

**Our bet:** No one occupies the top-right quadrant — high discovery breadth AND deep security. Wiz has depth but only in cloud. Reco has breadth but only in SaaS. We stitch together network + code + SaaS + cloud + procurement into one AI asset inventory.

---

## Our Differentiation

### What We Do That Nobody Else Does

| Capability | Wiz | PAN | Cisco | Reco | AI SBOM (Us) |
|---|---|---|---|---|---|
| Network (DNS/SNI) AI detection | — | — | ✓ (firewall) | — | ✓ |
| Git repo dependency scanning | — | — | — | — | ✓ |
| Cloud billing integration | — | — | — | — | ✓ |
| SaaS/IdP AI discovery | — | — | — | ✓ | ✓ (Phase 2) |
| SaaS audit log scanning | — | — | — | ✓ | ✓ (Phase 2) |
| Expense/procurement detection | — | — | — | — | ✓ (Phase 2) |
| Browser extension detection | — | — | — | — | ✓ (Phase 3) |
| Entity resolution (cross-source) | — | — | — | — | ✓ |
| EU AI Act compliance reporting | — | — | — | — | ✓ (Phase 3) |
| AI-BOM (SPDX 3.0 export) | ✓ (proprietary) | ✓ (model-focused) | — | — | ✓ (Phase 3) |
| Policy engine | ✓ (cloud-scoped) | — | — | — | ✓ (Phase 3) |

### The Core Insight

Every competitor approaches AI security from their existing lens:
- **Cloud security vendors** (Wiz, Orca) → see AI in cloud, miss everything else
- **Model security vendors** (HiddenLayer, ex-Protect AI) → secure the model, don't discover usage
- **AI firewall vendors** (ex-Robust Intelligence, ex-Lakera) → block bad prompts, don't inventory
- **SaaS security vendors** (Reco) → see SaaS AI, miss code and network

**Nobody builds the unified AI asset inventory across all vectors.** That's the product.

---

## Strategic Implications

### Window of Opportunity

1. **Integration tax** — PAN, Cisco, Check Point, SentinelOne are all 6–18 months into integrating acquisitions. Their AI security stories are fragmented.
2. **Category creation** — "AI SBOM" as a category (vs. AI-SPM which is cloud-centric) lets us own the narrative of comprehensive AI inventory.
3. **Regulatory tailwind** — EU AI Act compliance deadlines are creating budget for exactly this. Compliance buyers want inventory + reporting, not model firewalls.
4. **SMB/mid-market gap** — Wiz, PAN, Cisco are enterprise-only. Reco is growing but SaaS-focused.

### Risks

1. **Wiz expands** — They could add non-cloud discovery (SaaS, code, network). They have the engineering talent and the graph infrastructure. The Google acquisition may slow or accelerate this.
2. **Reco adds depth** — They could add network and code scanning to their existing SaaS discovery. Their growth trajectory is aggressive.
3. **New entrant** — A well-funded startup could emerge with the same multi-vector thesis.
4. **Commoditization** — If SPDX 3.0 AI profiles become widely adopted, the BOM format becomes a commodity and the value shifts entirely to discovery breadth.

### Recommended Actions

1. **Ship Phase 1 fast** — DNS + Git + Cloud billing gives a demo-ready product with a unique cross-vector story nobody else has
2. **Get 3 design partners before Q3 2026** — lock in feedback loops before incumbents integrate
3. **Own "AI SBOM" messaging** — position against Wiz's "AI-SPM" (which is cloud-scoped) with "AI SBOM" (which implies completeness, like a real bill of materials)
4. **Target compliance buyers** — EU AI Act is the forcing function; sell to GRC teams, not just security teams
5. **Build entity resolution early** — the ability to stitch one AI asset from 3+ sources is the core defensible IP

---

## Sources

- [Wiz AI-SPM Product Page](https://www.wiz.io/solutions/ai-spm)
- [Wiz AI-SPM Launch Blog](https://www.wiz.io/blog/ai-security-posture-management)
- [Wiz AI Agent Security (Wizdom 2025)](https://www.wiz.io/blog/wiz-ai-spm-secures-ai-agents)
- [Wiz AI-BOM Academy](https://www.wiz.io/academy/ai-security/ai-bom-ai-bill-of-materials)
- [Wiz WIN AI Partnerships 2026](https://www.wiz.io/blog/win-ai-partnerships)
- [Wiz AI Application Visibility](https://www.wiz.io/blog/complete-ai-application-visibility-wiz)
- [PAN Acquires Protect AI](https://www.paloaltonetworks.com/company/press/2025/palo-alto-networks-completes-acquisition-of-protect-ai)
- [Cisco Acquires Robust Intelligence](https://blogs.cisco.com/news/fortifying-the-future-of-security-for-ai-cisco-announces-intent-to-acquire-robust-intelligence)
- [Cisco AI Defense](https://newsroom.cisco.com/c/r/newsroom/en/us/a/y2025/m04/cisco-security-reimagine-ai-rsac.html)
- [Reco Shadow AI Discovery](https://www.reco.ai/use-cases/shadow-ai-discovery)
- [Reco $30M Series B](https://www.calcalistech.com/ctechnews/article/hk42i2ddbg)
- [Check Point Acquires Lakera](https://www.lakera.ai/news/lakera-raises-20m-series-a-to-secure-generative-ai-applications)
- [SentinelOne Acquires Prompt Security](https://prompt.security/press/prompt-security-raises-18m-series-a-to-accelerate-its-mission-to-secure-genai-in-enterprises)
- [OpenAI Acquires Promptfoo](https://www.securityweek.com/openai-to-acquire-ai-security-startup-promptfoo/)
- [OWASP AIBOM Initiative](https://owasp.org/www-project-aibom/)
- [Linux Foundation AI-BOM with SPDX 3.0](https://www.linuxfoundation.org/research/ai-bom)
- [Gartner InfoSec Forecast 2025](https://www.gartner.com/en/newsroom/press-releases/2025-07-29-gartner-forecasts-worldwide-end-user-spending-on-information-security-to-total-213-billion-us-dollars-in-2025)
- [Gartner AI Security Platforms — Top Strategic Trend 2026](https://www.gartner.com/en/documents/7014998)
- [Menlo Ventures: Security for AI Startup Landscape](https://menlovc.com/perspective/security-for-ai-genai-risks-and-the-emerging-startup-landscape/)
- [AI-SPM Market Report (QY Research)](https://www.qyresearch.com/reports/4408663/ai-security-posture-management--ai-spm)
- [SPM Market Size (MarketsandMarkets)](https://www.marketsandmarkets.com/PressReleases/security-posture-management-spm.asp)
