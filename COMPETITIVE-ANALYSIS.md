# AI SBOM — Competitive Analysis

**Last updated:** March 2026

---

## Executive Summary

The AI security market is undergoing rapid consolidation. In the past 18 months, every major pure-play AI security startup has been acquired by a platform incumbent: **Protect AI → Palo Alto Networks (~$500M+)**, **Robust Intelligence → Cisco (~$400M)**, **Lakera → Check Point**, **Prompt Security → SentinelOne**, **Promptfoo → OpenAI**. This leaves a narrow window where incumbents are still integrating acquisitions, and a greenfield opportunity exists for a product that stitches together **multi-vector AI discovery** (network + code + SaaS + procurement) — something none of them do.

---

## Market Context

### Market Size

| Segment | 2024 | 2025/2026 | Projected | CAGR |
|---|---|---|---|---|
| AI in Cybersecurity (broad) | $26.5B | $35.4B (2026) | $234.6B by 2032 | 31.7% |
| AI Governance Platforms | — | $492M (2026, Gartner) | $1B+ by 2030 | 28–36% |
| AI-SPM (narrow) | $1.0B | — | $1.6B by 2031 | 6.5–18.4% |
| DSPM | — | $2.1B | $10.4B by 2030 | 38.3% |
| Security Posture Mgmt (broad) | — | $26.6B | $53.3B by 2030 | 14.9% |
| InfoSec total (Gartner) | $193B | $213B (2025) | $240B by 2026 | 12.5% |

**Key stat:** Only 13 companies focus specifically on securing AI systems/LLMs/agentic apps, with total funding of just $414M — less than 5% of the $8.5B broader AI security ecosystem (2024–2025).

### Regulatory Drivers

- **EU AI Act** — **August 2, 2026 deadline** for full enforcement on high-risk AI systems. Non-compliance penalty: up to **7% of global annual revenue**. Most enterprises face significant compliance gaps.
- **NIST AI RMF** — voluntary but increasingly referenced by US sector regulators (CFPB, FDA, SEC, FTC, EEOC). Federal contractors must follow NIST-aligned governance. Version 1.1 updates expected in 2026.
- **Gartner** named "AI Security Platforms" a **Top Strategic Technology Trend for 2026** — predicts >50% of enterprises will use AI security platforms by 2028 (up from <10% today)
- **Gartner** predicts manual AI compliance processes will expose 75% of regulated organizations to fines exceeding 5% of global revenue through 2027
- 86% of organizations experienced AI-related security incidents (Cisco 2025 Cybersecurity Readiness Index)
- 85% of organizations using some form of AI; 74% using managed AI services (Wiz State of AI 2025)
- Only 19% have full visibility into where and how AI is used across development (Cycode 2026)
- 91% of AI tools operate without IT oversight or approval (Reco)
- 67% of business leaders increasing AI investment; 78% of executives plan to increase cyber spending in 2026

### Funding & M&A Context

**VC funding:** $18B invested in cybersecurity seed-through-growth rounds in 2025 (highest in 3 years, up 26% YoY). AI security ecosystem specifically: $8.5B across 175 companies over 24 months.

**Cybersecurity M&A hit $102B in 2025** — a 300% increase over 2024 (400+ deals).

**Mega-deals:**

| Target | Acquirer | Price | Date |
|---|---|---|---|
| Wiz | Google/Alphabet | $32B | 2025 |
| Robust Intelligence | Cisco | ~$400M | Oct 2024 |
| Protect AI | Palo Alto Networks | ~$500M+ | Jul 2025 |
| Aim Security | Cato Networks | Undisclosed | 2025 |
| Lakera | Check Point | Undisclosed | Nov 2025 |
| Prompt Security | SentinelOne | Undisclosed | Aug 2025 |
| SGNL | CrowdStrike | $740M | Jan 2026 |
| Promptfoo | OpenAI | Undisclosed | Mar 2026 |

**Takeaway:** The acqui-hire spree validates the category but also means these capabilities are being absorbed into large platforms that move slowly on integration. The next 12–18 months is an integration tax window. Notably, Forrester warns enterprises will defer 25% of planned AI spend to 2027 as CFOs demand ROI — compliance-driven products (like AI inventory) are more resilient to this correction than discretionary AI security tools.

---

## Competitor Deep Dives

### 1. Wiz AI-SPM — The Primary Threat

**What they are:** Cloud security platform (CNAPP), acquired by Google for **$32B** (completed March 2026). Named Leader with Highest Current Offering Score in Forrester Wave CNAPP Q1 2026. **40% of Fortune 100** use Wiz. Scaled from $1M to $100M ARR in 18 months.

**AI-SPM Timeline:**
- **Nov 2023** — launched AI-SPM (first CNAPP to do so)
- **Jan 2024** — OpenAI SaaS connector (first CNAPP for OpenAI customers)
- **May 2024** — model scanning for self-hosted models
- **Nov 2025 (Wizdom 2025)** — major expansion: AI agent discovery, MCP discovery, Agent Inventory View
- **Mar 2026** — Google acquisition completed; confirmed multi-cloud commitment

**What they do well:**

| Capability | Details |
|---|---|
| **AI-BOM** | Agentless discovery of AI services, models, SDKs, libraries, dependencies across cloud. Flags as approved/unwanted/unreviewed |
| **Agent Inventory** | Visualizes agents, models, tools, MCP connections, data (training + knowledge base). Contextualizes blast radius per agent |
| **Attack Surface Mapping** | Maps external-facing AI endpoints → workloads → owners via Security Graph. Dynamic scanner validates live exposure |
| **Misconfiguration Detection** | Built-in rules for SageMaker, Vertex AI, Bedrock, Azure OpenAI misconfigs. IaC scanning extension |
| **DSPM for AI** | Detects sensitive training data, flags data leakage risks, proactively removes attack paths to training data |
| **Attack Path Analysis** | Links vulnerabilities, identities, network exposure, secrets to AI models. "Toxic combination" detection |
| **Runtime Monitoring** | Detects rogue agents, suspicious DNS from AI workloads, drift from baselines |
| **MCP Discovery** | Discovers Model Context Protocol usage across environments |
| **Model Scanning** | Scans self-hosted models for malicious content (42% → 75% of orgs now self-host) |
| **OWASP LLM Top 10** | Built-in policies for prompt injection, data poisoning, insecure output handling |
| **Mika AI** | Natural language queries against security graph ("Which LLMs have access to production databases?") |

**Architecture:** Graph database (Amazon Neptune) maps all cloud resources, identities, vulnerabilities, AI assets as interconnected nodes. AI-SPM is fully embedded — not a bolt-on.

**Clouds supported:** AWS, Azure, GCP, OCI, Alibaba Cloud, VMware vSphere, Kubernetes, OpenShift.

**AI services detected:** SageMaker, Bedrock, Azure OpenAI, Vertex AI, OpenAI (SaaS), TensorFlow, Hugging Face, LangChain, DeepSeek, Mistral, BERT, Qwen2.

**Pricing:** No public pricing; bundled within CNAPP (not sold separately). Large environments (~3,000 people) report >$100K. Available via AWS Marketplace. No free tier.

**Key stat from Wiz research:** Only **13% of organizations have adopted AI-specific security controls**. 25% don't know what AI services are running in their environment.

**What they miss:**

| Gap | Why It Matters |
|---|---|
| **Cloud-only scope** | Zero visibility into SaaS-embedded AI (Notion AI, Slack AI, Salesforce Einstein), browser extensions, individual subscriptions |
| **No network-level detection** | Can't see DNS calls to `api.openai.com` from on-prem or non-cloud workloads |
| **No procurement/expense data** | Misses shadow AI purchased via credit cards, expensed individually |
| **No code dependency scanning** | Doesn't parse `requirements.txt`, `package.json` for AI SDK usage across repos (they scan cloud workloads, not git repos) |
| **Bundled-only pricing** | Must buy full CNAPP; no standalone AI inventory product. Prices out SMB/mid-market |
| **Google acquisition risk** | Multi-cloud commitment stated but untested long-term; GCP-first incentives are real |
| **SaaS connector gap** | Only OpenAI and M365 SaaS connectors; no Notion, Slack, Salesforce AI feature detection |

**Threat level: HIGH** — They have the brand, the graph, and the distribution. But their AI-SPM is a feature of their CNAPP, not a standalone product. They'll never care about expense-report-level shadow AI. Their own data says 87% of organizations lack AI-specific security controls — the market is wide open beyond cloud-native orgs.

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

### 5. Grip Security — Identity-First SaaS + AI

**What they are:** Identity-centric SaaS security platform. $66M total funding ($41M Series B, Aug 2023, led by Third Point Ventures). ~147 employees. Independent.

**What they offer:**
- Identity-first SaaS + AI discovery (analyzes email flows, browser activity, IdP/SSO data)
- Shadow AI detection: discovers AI tools, flags AI features embedded in approved SaaS
- AI agent mapping, access control, policy enforcement
- Agentless deployment — no network changes
- License optimization and cost management
- Browser extension for user behavior insights

**Key stat:** Their research found 91% of AI tools are unmanaged; average enterprise operates 3,891 SaaS+AI environments; 96% show ChatGPT presence despite bans.

**Threat level: MEDIUM** — Similar SaaS-first approach as Reco but smaller and less AI-focused. Identity angle is interesting but doesn't extend to network/code/cloud billing.

---

### 6. Other Players

#### HiddenLayer (Independent — $56M funding)
- **Most comprehensive independent AI security company remaining**
- AISec Platform 2.0 (April 2025): AI Discovery, Supply Chain Security, Attack Simulation, Runtime Security, Model Genealogy, AI-BOM
- $50M Series A led by Microsoft M12; investors include IBM Ventures, Capital One Ventures
- ~164 employees. Strong government/defense foothold (MDA SHIELD contract with $151B ceiling)
- Gartner Cool Vendor for AI Security
- **Gap:** Enterprise/government focused; no SaaS shadow AI discovery or network-level detection; expensive

#### Orca Security (Independent — $640M funding, $1.8B valuation)
- Cloud CNAPP with AI-SPM add-on. Agentless SideScanning for AI model discovery
- Covers Azure OpenAI, Bedrock, SageMaker, Vertex AI, 50+ AI packages
- Acquired Opus (May 2025) for agentic AI remediation
- **Gap:** Same cloud-only limitations as Wiz; smaller market share; AI-SPM is a feature, not the product

#### CrowdStrike (Public — acquired SGNL for $740M, Pangea, Onum)
- Building agentic AI security capabilities through acquisitions
- Charlotte AI for threat hunting
- **Gap:** No dedicated AI-BOM or shadow AI discovery product; focused on threat detection

#### SentinelOne (+ Prompt Security acquisition)
- Acquired Prompt Security (Aug 2025) — GenAI runtime security, prompt injection defense, data leak protection
- Prompt Security had transparent pricing: $120 per 1,000 requests/year
- **Gap:** Prompt Security was guardrails, not discovery/inventory. Integration ongoing

#### Check Point (+ Lakera acquisition)
- Acquired Lakera (Nov 2025) — forming Global Center of Excellence for AI Security in Zurich
- Lakera Guard: 98%+ prompt injection detection, sub-50ms latency, 100+ languages
- Gandalf game generated 80M+ adversarial prompts for threat intelligence
- **Gap:** Lakera was prompt injection defense, not AI discovery/BOM. Integration into Infinity Platform ongoing

#### F5 (+ CalypsoAI acquisition — $180M)
- Acquired CalypsoAI (Sep 2025) — inference-layer AI security
- AI-agent-driven red teaming, real-time guardrails, usage monitoring
- Strong government/defense customer base (Lockheed Martin investor)
- **Gap:** Inference-only focus; no discovery or inventory capabilities

#### Snyk
- Developer security with some AI-BOM thought leadership
- **Gap:** No shipping AI-SPM product; focused on traditional AppSec

---

## Competitive Positioning Matrix

### Funding & Status Summary

| Company | Status | Funding | Focus |
|---|---|---|---|
| **Wiz** | Acquired (Google, $32B) | $1.9B+ | Cloud AI-SPM (CNAPP feature) |
| **Palo Alto Networks** | Public ($120B+ mktcap) | N/A | Full AI security platform (Prisma AIRS) |
| **Cisco** | Public | N/A | Network-level AI defense |
| **Reco** | Independent | $85M | SaaS + shadow AI discovery |
| **Grip Security** | Independent | $66M | Identity-first SaaS + AI |
| **HiddenLayer** | Independent | $56M | Full AI lifecycle security |
| **Orca Security** | Independent | $640M | Cloud CNAPP + AI-SPM |
| **CrowdStrike** | Public | N/A | Threat detection + agentic AI |
| **SentinelOne** | Public (+ Prompt Security) | N/A | Endpoint + GenAI guardrails |
| **Check Point** | Public (+ Lakera) | N/A | Network + prompt injection |
| **F5** | Public (+ CalypsoAI, $180M) | N/A | Inference-layer AI security |

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
    T    │   PAN/Protect AI   │      Grip               │
    Y    │   Cisco/Robust Int │                         │
         │   SentinelOne      │                         │
    D    │   Check Point      │                         │
    E    │   F5/CalypsoAI     │                         │
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
- [Grip Security — AI Security](https://www.grip.security/ai-security)
- [Grip Security — Series B](https://www.grip.security/press-release/grip-security-raising-41-million-series-b-led-by-third-point-ventures)
- [HiddenLayer AISec Platform 2.0](https://hiddenlayer.com/innovation-hub/hiddenlayer-unveils-aisec-platform-2-0-to-deliver-unmatched-context-visibility-and-observability-for-enterprise-ai-security/)
- [HiddenLayer 2026 AI Threat Report](https://www.prnewswire.com/news-releases/hiddenlayer-releases-the-2026-ai-threat-landscape-report-spotlighting-the-rise-of-agentic-ai-and-the-expanding-attack-surface-of-autonomous-systems-302716687.html)
- [F5 Acquires CalypsoAI ($180M)](https://www.geekwire.com/2025/f5-paying-180m-to-acquire-calypsoai-to-boost-ai-enterprise-security-offerings/)
- [Orca Security AI-SPM](https://orca.security/platform/ai-security-posture-management/)
- [Orca Acquires Opus](https://orca.security/resources/blog/orca-security-acquires-opus-agentic-ai-cnapp/)
- [PAN Prisma AIRS Launch](https://www.paloaltonetworks.com/company/press/2025/palo-alto-networks-introduces-prisma-airs--the-foundation-on-which-ai-security-thrives)
- [PAN Prisma AIRS 2.0](https://www.paloaltonetworks.com/blog/2025/10/prisma-airs-powering-secure-ai-innovation/)
- [Gartner AI Governance Platforms Market](https://www.gartner.com/en/newsroom/press-releases/2026-02-17-gartner-global-ai-regulations-fuel-billion-dollar-market-for-ai-governance-platforms)
- [Gartner Top Cybersecurity Trends 2026](https://www.gartner.com/en/newsroom/press-releases/2026-02-05-gartner-identifies-the-top-cybersecurity-trends-for-2026)
- [EU AI Act Compliance Deadlines](https://www.legalnodes.com/article/eu-ai-act-2026-updates-compliance-requirements-and-business-risks)
- [Cybersecurity M&A Hit $102B in 2025](https://www.hsfkramer.com/insights/reports/2026/global-ma-report-2026/sector-perspectives/cybersecurity)
- [AI Security VC Funding 2025 ($8.5B)](https://softwarestrategiesblog.com/2025/12/30/ai-security-startups-funding-2025/)
- [Cybersecurity Startup Investment 2025](https://news.crunchbase.com/venture/cybersecurity-startup-investment-up-ye-2025/)
- [Knostic — Shadow AI Detection Tools](https://www.knostic.ai/blog/shadow-ai-detection-tools)
