# Phase 1 Implementation Plan

**Goal:** Ship the first cross-vector AI discovery product — DNS/network + Git dependency scanning + cloud billing — enough to demo and get 3 design partners.

**Timeline target:** Demo-ready by mid-2026. Design partners signed by Q3 2026.

---

## What Exists Today

The `shadow-ai-detector/` module is fully built and covers **DNS/network detection**:

| Component | Status | What It Does |
|---|---|---|
| `detection.service.js` | Done | Processes DNS/SNI events, matches against provider registry, deduplicates, creates findings |
| `registry.service.js` | Done | AI provider domain registry (17 providers, ~50 domains), per-tenant custom providers |
| `ai-providers.json` | Done | Seed data: OpenAI, Anthropic, Google, Azure, Bedrock, Cohere, Mistral, HuggingFace, Replicate, Perplexity, Groq, Together, DeepSeek, Stability, Midjourney, ElevenLabs, Cursor, Copilot |
| `dedup.service.js` | Done | Groups events by (sourceIp, providerId) with configurable time windows |
| `findings.service.js` | Done | CRUD for findings in DynamoDB |
| `enrichment.service.js` | Done | EC2 tag lookup + tenant service catalog for IP → team/service attribution |
| `alerting.service.js` | Done | Webhook/SNS alerts on new findings |
| `allowlist.service.js` | Done | Suppress known-good IPs (approved AI gateways) |
| `customer-stack.yaml` | Done | CloudFormation: DNS Firewall domain list, EventBridge forwarding, auto-sync Lambda |
| API routes | Done | Full REST API: ingest, findings, groups, registry, allowlist, enrichment, admin |

**Bottom line:** DNS detection is production-grade. Phase 1 is about adding the two other vectors (git + cloud billing) and stitching all three into a unified AI asset inventory.

---

## Why Three Sources, Not One

DNS detection alone tells you "someone at IP `10.0.3.42` called `api.openai.com`." That's useful but incomplete. Each source catches things the others miss:

```
┌─────────────────────────────────────────────────────────────┐
│                    WHAT EACH SOURCE SEES                     │
├──────────────┬──────────────────────────────────────────────┤
│              │                                              │
│  DNS/Network │  Runtime API calls to AI providers           │
│  (built)     │  ✓ Production traffic to api.openai.com      │
│              │  ✓ Shadow AI: dev laptop hitting Claude API   │
│              │  ✗ Can't see which codebase is making the call│
│              │  ✗ Can't see AI SDKs that haven't been        │
│              │    deployed yet (sitting in a branch)         │
│              │  ✗ Can't see self-hosted models (no external  │
│              │    API call to intercept)                     │
│              │                                              │
├──────────────┼──────────────────────────────────────────────┤
│              │                                              │
│  Git repos   │  AI SDKs declared in dependency files        │
│  (to build)  │  ✓ Which exact codebase uses which AI SDK    │
│              │  ✓ SDK version (matters for vulns)           │
│              │  ✓ Code not yet deployed (upcoming risk)     │
│              │  ✓ Self-hosted model code (torch, vllm,      │
│              │    transformers — never calls external API)  │
│              │  ✓ AI in CI/CD (GitHub Action using GPT      │
│              │    for PR review — no VPC traffic)           │
│              │  ✗ Can't see runtime usage (SDK in code      │
│              │    doesn't mean it's being called)           │
│              │  ✗ Can't see AI tools used without code      │
│              │    (console-provisioned SageMaker endpoint)  │
│              │                                              │
├──────────────┼──────────────────────────────────────────────┤
│              │                                              │
│  Cloud       │  AI service spend on the cloud bill          │
│  billing     │  ✓ Console-provisioned AI (SageMaker         │
│  (to build)  │    endpoint someone spun up via AWS console) │
│              │  ✓ Cost attribution (which account, how much)│
│              │  ✓ Catches managed services that don't       │
│              │    generate DNS alerts (intra-AWS calls)     │
│              │  ✗ 24-48 hour delay (billing data is stale)  │
│              │  ✗ Account-level, not service-level           │
│              │  ✗ Only sees cloud-provider AI services,     │
│              │    not third-party API spend (OpenAI bills   │
│              │    you directly, not through AWS)            │
│              │                                              │
└──────────────┴──────────────────────────────────────────────┘
```

**The value is correlation.** When DNS says `10.0.3.42` is calling `api.openai.com`, git says `payments-api` repo has `openai` in `package.json`, and enrichment says `10.0.3.42` runs `payments-api` — that's a high-confidence asset corroborated by three independent sources. Much harder to dismiss than a single DNS log.

---

## How Team Attribution Works

Every discovery source provides a different signal about who owns the AI usage. No single source is definitive. The asset service layers them together.

### Attribution signals by source

```
┌──────────────────────────────────────────────────────────────────────┐
│ SOURCE          │ WHAT IT TELLS US ABOUT OWNERSHIP                  │
├──────────────────────────────────────────────────────────────────────┤
│                 │                                                    │
│ DNS/Network     │ Source IP → EC2 instance → instance tags           │
│ (enrichment     │                                                    │
│  service, built)│ enrichment.service.js does two lookups:            │
│                 │                                                    │
│                 │ 1. EC2 tag lookup (describeInstances by private IP)│
│                 │    Tag "service" or "app"  → serviceName           │
│                 │    Tag "team" or "owner"   → team                  │
│                 │    Tag "env" or "stage"    → environment           │
│                 │                                                    │
│                 │ 2. Tenant service catalog (fallback)               │
│                 │    Customer uploads IP/CIDR → service mappings     │
│                 │    Useful for k8s/ECS where IPs are ephemeral      │
│                 │    Example: "10.0.3.0/24 = payments namespace"     │
│                 │                                                    │
│                 │ Confidence: MEDIUM                                 │
│                 │ Tags can be stale. IPs get reassigned.             │
│                 │ Many orgs don't tag consistently.                  │
│                 │                                                    │
├──────────────────────────────────────────────────────────────────────┤
│                 │                                                    │
│ Git repos       │ Repo metadata → team/service                       │
│                 │                                                    │
│                 │ 1. Repo name (by convention)                       │
│                 │    "acme-corp/payments-api" → service=payments-api │
│                 │                                                    │
│                 │ 2. CODEOWNERS file (if present)                    │
│                 │    "* @acme-corp/payments-team" → team=payments    │
│                 │                                                    │
│                 │ 3. GitHub/GitLab teams/permissions                  │
│                 │    Which team has write access to this repo?       │
│                 │                                                    │
│                 │ 4. Last committers (weaker signal)                 │
│                 │    Recent commit authors → individual attribution  │
│                 │                                                    │
│                 │ Confidence: HIGH                                   │
│                 │ If openai SDK is in acme-corp/payments-api, the    │
│                 │ payments team owns it. No inference needed.        │
│                 │                                                    │
├──────────────────────────────────────────────────────────────────────┤
│                 │                                                    │
│ Cloud billing   │ AWS account → team                                 │
│                 │                                                    │
│                 │ 1. AWS Organizations account name/tags             │
│                 │    Account 111111111111 = "payments-prod"          │
│                 │    If it's spending on Bedrock → payments team     │
│                 │                                                    │
│                 │ 2. Cost allocation tags (if enabled)               │
│                 │    Some orgs tag resources with team/project       │
│                 │    Cost Explorer can group by these tags           │
│                 │                                                    │
│                 │ Confidence: MEDIUM-HIGH                            │
│                 │ Account-level, not resource-level. An account      │
│                 │ might host multiple teams' workloads.              │
│                 │                                                    │
└──────────────────────────────────────────────────────────────────────┘
```

### How entity resolution merges these signals

When a new discovery arrives, the asset service tries to match it to an existing asset. The match key is **provider + owner identity**, tried from most specific to least specific:

```
Discovery arrives: { provider: "openai", source: "git", repo: "acme-corp/payments-api" }

Step 1: Extract attribution from the source
        → repo name "payments-api" → serviceName = "payments-api"

Step 2: Try to find existing asset with same provider + serviceName
        → Found: ASSET#openai::payments-api (created earlier from DNS finding)
        → MERGE: add git discovery details to existing asset

Step 3: Asset now has two corroborating sources:
        discoveredVia: ["network_dns", "git_dependency"]
        owner.confidence: "high" (two sources agree on serviceName)
```

**When sources disagree or can't be linked:**

```
Discovery arrives: { provider: "openai", source: "network", sourceIp: "10.0.5.99" }

Step 1: Extract attribution from the source
        → EC2 lookup returns no tags (untagged instance)
        → Service catalog has no mapping for this IP
        → serviceName = null, team = null

Step 2: Try to find existing asset
        → No match (no serviceName to match on, no repo to match on)

Step 3: Create new asset with status "unreviewed"
        → ASSET#openai::unattributed-10.0.5.99
        → Surfaces in dashboard as "unattributed AI usage — needs review"
        → Human assigns owner manually via PATCH /api/v1/assets/:id
```

**This is the honest part:** shadow AI is shadow precisely because it's untagged, unmanaged, and unattributed. The product doesn't pretend to magically know who owns everything. It surfaces the unknown and asks humans to claim it. The value is that it found it at all.

### Attribution confidence levels

| Sources that agree | Confidence | What happens |
|---|---|---|
| 3 sources (DNS + git + billing) all point to same team | `confirmed` | Auto-assigned, high trust |
| 2 sources agree (e.g., DNS enrichment + git repo name) | `high` | Auto-assigned, shown as corroborated |
| 1 source with strong signal (git repo with CODEOWNERS) | `medium` | Auto-assigned, flagged for verification |
| 1 source with weak signal (EC2 tag only) | `low` | Suggested owner, needs manual confirmation |
| 0 sources (no enrichment data, no repo context) | `none` | Unattributed, surfaces as "unknown owner" for review |

---

## What Needs to Be Built

### Module 1: Git Repository Scanner

**Purpose:** Discover AI SDK usage in source code by scanning dependency files across all repos. Answers "which codebases have AI baked in?" — something DNS can never tell you.

#### 1a. AI Package Registry

Create `shadow-ai-detector/src/data/ai-packages.json` — a lookup table of known AI packages across ecosystems.

```json
{
  "npm": [
    { "name": "openai", "provider": "openai", "category": "general_llm" },
    { "name": "@anthropic-ai/sdk", "provider": "anthropic", "category": "general_llm" },
    { "name": "@google/generative-ai", "provider": "google-ai", "category": "general_llm" },
    { "name": "@aws-sdk/client-bedrock-runtime", "provider": "aws-bedrock", "category": "general_llm" },
    { "name": "@azure/openai", "provider": "azure-openai", "category": "general_llm" },
    { "name": "cohere-ai", "provider": "cohere", "category": "general_llm" },
    { "name": "@mistralai/mistralai", "provider": "mistral", "category": "general_llm" },
    { "name": "replicate", "provider": "replicate", "category": "model_hosting" },
    { "name": "@huggingface/inference", "provider": "huggingface", "category": "model_hosting" },
    { "name": "langchain", "provider": "langchain", "category": "ai_framework" },
    { "name": "@langchain/core", "provider": "langchain", "category": "ai_framework" },
    { "name": "llamaindex", "provider": "llamaindex", "category": "ai_framework" }
  ],
  "pypi": [
    { "name": "openai", "provider": "openai", "category": "general_llm" },
    { "name": "anthropic", "provider": "anthropic", "category": "general_llm" },
    { "name": "google-generativeai", "provider": "google-ai", "category": "general_llm" },
    { "name": "boto3", "provider": "aws-bedrock", "category": "general_llm", "matchHint": "bedrock" },
    { "name": "cohere", "provider": "cohere", "category": "general_llm" },
    { "name": "mistralai", "provider": "mistral", "category": "general_llm" },
    { "name": "replicate", "provider": "replicate", "category": "model_hosting" },
    { "name": "huggingface-hub", "provider": "huggingface", "category": "model_hosting" },
    { "name": "transformers", "provider": "huggingface", "category": "ml_framework" },
    { "name": "torch", "provider": "pytorch", "category": "ml_framework" },
    { "name": "tensorflow", "provider": "tensorflow", "category": "ml_framework" },
    { "name": "langchain", "provider": "langchain", "category": "ai_framework" },
    { "name": "langchain-core", "provider": "langchain", "category": "ai_framework" },
    { "name": "llama-index", "provider": "llamaindex", "category": "ai_framework" },
    { "name": "vllm", "provider": "vllm", "category": "inference_server" }
  ],
  "go": [
    { "name": "github.com/sashabaranov/go-openai", "provider": "openai", "category": "general_llm" },
    { "name": "github.com/liushuangls/go-anthropic", "provider": "anthropic", "category": "general_llm" }
  ],
  "nuget": [
    { "name": "Azure.AI.OpenAI", "provider": "azure-openai", "category": "general_llm" },
    { "name": "OpenAI", "provider": "openai", "category": "general_llm" }
  ]
}
```

**File types to scan:**

| Ecosystem | Files | Parser |
|---|---|---|
| npm | `package.json` | JSON → check `dependencies` + `devDependencies` keys |
| Python | `requirements.txt`, `pyproject.toml`, `Pipfile`, `setup.py`, `setup.cfg` | Line-by-line / TOML / regex |
| Go | `go.mod` | Line-by-line `require` block |
| .NET | `*.csproj` | XML `<PackageReference>` |
| Ruby | `Gemfile` | Regex |
| Java | `pom.xml`, `build.gradle` | XML / regex |

#### 1b. Git Scanner Service

New file: `shadow-ai-detector/src/services/git-scanner.service.js`

**Integration points:**
- GitHub API (repos, contents) via `@octokit/rest`
- GitLab API via `@gitbeaker/rest`
- Bitbucket API (stretch goal)

**Core flow:**

```
1. List all repos for the org (paginated)
2. For each repo, check for dependency files (GET /repos/:owner/:repo/contents/package.json etc.)
3. Parse each dependency file → extract package names
4. Match against ai-packages.json
5. For each match → create an AI asset discovery event
```

**Service API:**

```javascript
// Scan all repos for a tenant's connected Git org
async function scanOrganization(tenantId, connection) → { repos, matches, errors }

// Scan a single repo
async function scanRepository(tenantId, connection, repoFullName) → { matches }

// Parse a dependency file and return AI package matches
function parseDependencyFile(filename, content, packageRegistry) → [{ package, version, provider, category }]
```

**Key design decisions:**
- **API-only, no cloning.** Use GitHub/GitLab content APIs to fetch specific files. Don't clone repos — it's slow, needs disk, and most of the repo content is irrelevant.
- **Incremental scanning.** Store `lastScannedAt` per repo. On subsequent scans, use commit timestamps to skip unchanged repos. Full rescan weekly.
- **Rate limiting.** GitHub API: 5,000 req/hr per token. With ~500 repos and ~6 file checks each = 3,000 requests. Fits in one scan cycle. For larger orgs, queue and batch.

#### 1c. Git Connection Management

New routes: `shadow-ai-detector/src/routes/git.routes.js`

```
POST   /api/v1/git/connections          — Register a Git org (GitHub/GitLab token + org name)
GET    /api/v1/git/connections          — List connected orgs
DELETE /api/v1/git/connections/:id      — Remove a connection
POST   /api/v1/git/connections/:id/scan — Trigger manual scan
GET    /api/v1/git/scan-results         — Get latest scan results (repos, matches, coverage)
```

**Connection stored in DynamoDB:**

```json
{
  "PK": "TENANT#acme",
  "SK": "GIT_CONNECTION#github-acme-corp",
  "platform": "github",
  "orgName": "acme-corp",
  "tokenEncrypted": "...",
  "lastScannedAt": "2026-03-22T10:00:00Z",
  "reposScanned": 142,
  "aiPackagesFound": 23,
  "status": "active"
}
```

#### 1d. Scheduled Scanning

- EventBridge rule: trigger scan every 6 hours
- Lambda handler invokes `scanOrganization()` for each active connection
- Results flow into the same findings/asset pipeline as DNS detections

---

### Module 2: Cloud Billing Scanner

**Purpose:** Discover AI service spending that bypasses code and network detection (e.g., someone spins up a SageMaker endpoint via the console).

#### 2a. AWS Cost Explorer Integration

New file: `shadow-ai-detector/src/services/billing-scanner.service.js`

**AWS AI services to flag:**

```javascript
const AWS_AI_SERVICES = [
  { serviceCode: 'AmazonBedrock', provider: 'aws-bedrock', category: 'managed_cloud' },
  { serviceCode: 'AmazonSageMaker', provider: 'aws-sagemaker', category: 'managed_cloud' },
  { serviceCode: 'AmazonRekognition', provider: 'aws-rekognition', category: 'vision_ai' },
  { serviceCode: 'AmazonComprehend', provider: 'aws-comprehend', category: 'nlp' },
  { serviceCode: 'AmazonTranscribe', provider: 'aws-transcribe', category: 'speech_ai' },
  { serviceCode: 'AmazonPolly', provider: 'aws-polly', category: 'speech_ai' },
  { serviceCode: 'AmazonTextract', provider: 'aws-textract', category: 'document_ai' },
  { serviceCode: 'AmazonTranslate', provider: 'aws-translate', category: 'translation' },
  { serviceCode: 'AmazonLex', provider: 'aws-lex', category: 'conversational_ai' },
  { serviceCode: 'AmazonKendra', provider: 'aws-kendra', category: 'ai_search' },
  { serviceCode: 'AmazonQBusiness', provider: 'aws-q', category: 'general_llm' },
];
```

**Azure equivalents** (via Azure Cost Management API):

```javascript
const AZURE_AI_SERVICES = [
  { meterCategory: 'Azure OpenAI', provider: 'azure-openai', category: 'general_llm' },
  { meterCategory: 'Cognitive Services', provider: 'azure-cognitive', category: 'ai_platform' },
  { meterCategory: 'Azure Machine Learning', provider: 'azure-ml', category: 'managed_cloud' },
  { meterCategory: 'Azure AI Search', provider: 'azure-ai-search', category: 'ai_search' },
];
```

**GCP equivalents** (via BigQuery billing export):

```javascript
const GCP_AI_SERVICES = [
  { serviceDescription: 'Vertex AI', provider: 'gcp-vertex', category: 'managed_cloud' },
  { serviceDescription: 'Cloud Natural Language', provider: 'gcp-nlp', category: 'nlp' },
  { serviceDescription: 'Cloud Vision', provider: 'gcp-vision', category: 'vision_ai' },
  { serviceDescription: 'Cloud Translation', provider: 'gcp-translate', category: 'translation' },
];
```

#### 2b. Billing Scanner Service

**Core flow:**

```
1. Call AWS Cost Explorer GetCostAndUsage (grouped by SERVICE, LINKED_ACCOUNT)
2. Filter for AI service codes
3. For each match → amount, account, time period
4. Optionally drill down with GetCostAndUsageWithResources for resource-level detail
5. Create AI asset discovery event
```

**Service API:**

```javascript
// Scan AWS billing for AI services (last 30 days by default)
async function scanAWSBilling(tenantId, connection, options) → { services, totalSpend, accounts }

// Scan Azure billing
async function scanAzureBilling(tenantId, connection, options) → { services, totalSpend, subscriptions }

// Scan GCP billing (via BigQuery)
async function scanGCPBilling(tenantId, connection, options) → { services, totalSpend, projects }
```

**Key design decisions:**
- **Start with AWS only.** Azure and GCP are Phase 1.5 — same pattern, different API.
- **Cross-account via AWS Organizations.** Cost Explorer can query the management account for all linked accounts. One connection covers the whole org.
- **Daily scan cadence.** Billing data is delayed ~24 hours. Daily scan at 6am UTC is sufficient.
- **Cost threshold alerts.** Flag new AI services that appear for the first time, or existing services that spike >50%.

#### 2c. Cloud Connection Management

New routes: `shadow-ai-detector/src/routes/cloud.routes.js`

```
POST   /api/v1/cloud/connections          — Register a cloud account (IAM role ARN for AWS)
GET    /api/v1/cloud/connections          — List connected cloud accounts
DELETE /api/v1/cloud/connections/:id      — Remove a connection
POST   /api/v1/cloud/connections/:id/scan — Trigger manual scan
GET    /api/v1/cloud/billing-results      — Get latest billing scan results
```

**AWS connection model:** Customer creates a cross-account IAM role with `ce:GetCostAndUsage` + `ce:GetCostAndUsageWithResources` + `organizations:ListAccounts`. We assume the role to query.

---

### Module 3: Unified AI Asset Inventory

**Purpose:** Stitch findings from DNS, Git, and billing into a single AI asset per (provider, team/service).

#### 3a. Asset Service

New file: `shadow-ai-detector/src/services/asset.service.js`

This is the **entity resolution** layer — the core IP of the product.

**Asset data model:**

```json
{
  "PK": "TENANT#acme",
  "SK": "ASSET#openai::payments-api",
  "assetId": "ai-asset-0042",
  "provider": "openai",
  "providerName": "OpenAI",
  "model": "gpt-4",
  "category": "direct_api",

  "discoveredVia": ["network_dns", "git_dependency", "cloud_billing"],
  "discoveryDetails": {
    "network": {
      "sourceIps": ["10.0.3.42"],
      "domains": ["api.openai.com"],
      "lastSeen": "2026-03-22T15:30:00Z",
      "eventCount": 1247
    },
    "git": {
      "repos": ["acme-corp/payments-api"],
      "packages": [{ "name": "openai", "version": "4.28.0", "ecosystem": "npm" }],
      "lastScanned": "2026-03-22T06:00:00Z"
    },
    "billing": {
      "monthlySpend": "$340",
      "accounts": ["123456789012"],
      "services": ["AmazonBedrock"],
      "lastScanned": "2026-03-22T06:00:00Z"
    }
  },

  "owner": {
    "team": "payments",
    "service": "payments-api",
    "contact": null,
    "confidence": "high"
  },

  "status": "unreviewed",
  "riskTier": "high",
  "firstSeen": "2026-01-15T00:00:00Z",
  "lastSeen": "2026-03-22T15:30:00Z",
  "createdAt": "2026-01-15T10:00:00Z",
  "updatedAt": "2026-03-22T15:30:00Z"
}
```

#### 3b. Entity Resolution Logic

The hard part: when a DNS finding, a git match, and a billing line item all refer to the same AI usage, merge them into one asset. See "How Team Attribution Works" above for the full walkthrough of how each source contributes ownership signals, how they get merged, and what happens when sources disagree.

**Resolution keys (from most to least specific):**

| Priority | Match Key | Example | When It Fires |
|---|---|---|---|
| 1 | `provider + service_name` | `openai::payments-api` | EC2 tags or git repo name both resolve to same service |
| 2 | `provider + repo_name` | `openai::acme-corp/payments-api` | Git match but DNS enrichment only has IP (no service tag) |
| 3 | `provider + team` | `openai::payments-team` | Multiple services from same team, can't distinguish which |
| 4 | `provider + account_id` | `openai::111111111111` | Billing-only discovery, no DNS or git data |
| 5 | `provider + unattributed` | `openai::unattributed-10.0.5.99` | DNS saw traffic but no enrichment, no git match — flagged for human review |

**Rules:**
- Same provider + same resolution key → **merge** (add new source to existing asset)
- Same provider + no attribution data → **create** with `status: "unreviewed"` for manual triage
- Different providers + same repo → **separate assets** (one repo can use OpenAI and Anthropic — those are two assets)

**Service API:**

```javascript
// Merge a discovery event into the asset inventory
async function ingestDiscovery(tenantId, discovery) → { asset, action: 'created' | 'merged' | 'updated' }

// Get the full asset inventory
async function getAssets(tenantId, filters) → { assets, count, nextKey }

// Get a single asset with full history
async function getAsset(tenantId, assetId) → { asset, discoveryTimeline }

// Get inventory stats
async function getInventoryStats(tenantId) → { totalAssets, byProvider, byCategory, bySource, byTeam, byRisk }
```

#### 3c. Asset Routes

New routes: `shadow-ai-detector/src/routes/assets.routes.js`

```
GET    /api/v1/assets              — List all AI assets (filterable by provider, team, source, risk, status)
GET    /api/v1/assets/:id          — Get single asset with discovery details
PATCH  /api/v1/assets/:id          — Update asset (set owner, status, notes)
GET    /api/v1/assets/stats        — Inventory dashboard stats
GET    /api/v1/assets/export       — Export as JSON (CSV and SPDX in Phase 3)
```

---

### Module 4: Dashboard API

**Purpose:** Power the frontend dashboard (frontend itself is Phase 1.5 — start with API + CLI).

New routes: `shadow-ai-detector/src/routes/dashboard.routes.js`

```
GET /api/v1/dashboard/summary
```

**Response:**

```json
{
  "inventory": {
    "totalAssets": 47,
    "byCategory": { "direct_api": 12, "managed_cloud": 8, "saas_embedded": 0, "self_hosted": 3 },
    "byRiskTier": { "high": 15, "medium": 22, "low": 10 },
    "byStatus": { "unreviewed": 31, "approved": 12, "blocked": 4 }
  },
  "discovery": {
    "sourcesActive": 3,
    "sources": {
      "network_dns": { "status": "active", "lastEvent": "2026-03-22T15:30:00Z", "findingsTotal": 234 },
      "git_dependency": { "status": "active", "lastScan": "2026-03-22T06:00:00Z", "reposScanned": 142, "matchesFound": 23 },
      "cloud_billing": { "status": "active", "lastScan": "2026-03-22T06:00:00Z", "aiServicesFound": 5, "monthlySpend": "$4,200" }
    }
  },
  "recentActivity": [
    { "type": "new_asset", "asset": "...", "timestamp": "..." },
    { "type": "new_source", "asset": "...", "source": "git_dependency", "timestamp": "..." }
  ],
  "topTeams": [
    { "team": "payments", "assetCount": 8, "monthlySpend": "$1,200" },
    { "team": "search", "assetCount": 5, "monthlySpend": "$2,800" }
  ]
}
```

---

## Implementation Order

| Step | What | New Files | Depends On | Effort |
|---|---|---|---|---|
| **1** | AI package registry data file | `src/data/ai-packages.json` | Nothing | Small |
| **2** | Git scanner service | `src/services/git-scanner.service.js` | Step 1 | Medium |
| **3** | Git connection routes | `src/routes/git.routes.js` | Step 2 | Small |
| **4** | Asset service (entity resolution) | `src/services/asset.service.js` | Nothing | Large — this is the hard part |
| **5** | Asset routes | `src/routes/assets.routes.js` | Step 4 | Small |
| **6** | Wire DNS findings → asset service | Modify `detection.service.js` | Step 4 | Small |
| **7** | Wire git matches → asset service | Modify `git-scanner.service.js` | Steps 2, 4 | Small |
| **8** | Billing scanner service (AWS) | `src/services/billing-scanner.service.js` | Nothing | Medium |
| **9** | Cloud connection routes | `src/routes/cloud.routes.js` | Step 8 | Small |
| **10** | Wire billing → asset service | Modify `billing-scanner.service.js` | Steps 4, 8 | Small |
| **11** | Dashboard summary API | `src/routes/dashboard.routes.js` | Step 4 | Small |
| **12** | Scheduled scanning (EventBridge + Lambda) | `infra/` updates | Steps 2, 8 | Medium |
| **13** | Tests for all new services | `tests/unit/`, `tests/integration/` | All above | Medium |

**Critical path:** Steps 1 → 2 → 4 → 6 → 7 (git working end-to-end with entity resolution)

**Parallel track:** Steps 8 → 10 (billing can be built independently, wired in at step 10)

---

## New Dependencies

```json
{
  "@octokit/rest": "^21.0.0",
  "@aws-sdk/client-cost-explorer": "^3.500.0",
  "@aws-sdk/client-organizations": "^3.500.0",
  "@aws-sdk/client-sts": "^3.500.0",
  "toml": "^3.0.0"
}
```

GitLab (`@gitbeaker/rest`) and Azure/GCP billing SDKs added in Phase 1.5 when those integrations ship.

---

## DynamoDB Table Changes

**Existing tables** (no schema changes needed — single-table design already supports new record types):

| Record Type | PK | SK | Purpose |
|---|---|---|---|
| Git connection | `TENANT#<id>` | `GIT_CONNECTION#<connection-id>` | Stored GitHub/GitLab org credentials |
| Git scan result | `TENANT#<id>` | `GIT_SCAN#<timestamp>#<repo>` | Per-repo scan results |
| Cloud connection | `TENANT#<id>` | `CLOUD_CONNECTION#<connection-id>` | Stored cloud account credentials |
| Billing scan result | `TENANT#<id>` | `BILLING_SCAN#<timestamp>` | Billing scan summary |
| AI Asset | `TENANT#<id>` | `ASSET#<resolution-key>` | Unified AI asset |

**New GSI** (for asset queries):

| GSI | PK | SK | Purpose |
|---|---|---|---|
| `GSI-AssetsByProvider` | `TENANT#<id>` | `PROVIDER#<provider-id>` | List assets by provider |
| `GSI-AssetsByTeam` | `TENANT#<id>` | `TEAM#<team-name>` | List assets by team |

---

## What This Unlocks (the Demo)

With all three sources connected, the demo story is:

> "I connected our GitHub org, our AWS account, and deployed a DNS monitor in our VPC.
> In 10 minutes, AI SBOM found:
> - 12 repos using OpenAI SDK (3 of which nobody on the security team knew about)
> - $4,200/month in Bedrock spend across 3 AWS accounts
> - 47 unique IPs making direct API calls to AI providers, including 8 from production
>
> Here's the unified inventory. Each asset shows every source it was discovered through.
> The payments-api shows up in all three: DNS logs show it calling api.openai.com, its package.json has the openai SDK, and the AWS account it runs in is spending on Bedrock.
>
> Nobody else gives you this view."

---

## Out of Scope (Phase 2+)

| Capability | Phase |
|---|---|
| SaaS/IdP integration (Okta, Azure AD) | Phase 2 |
| SaaS audit log scanning (Slack, Notion, Salesforce) | Phase 2 |
| Policy engine + compliance rules | Phase 3 |
| EU AI Act compliance reporting | Phase 3 |
| AI-BOM export (SPDX 3.0 / CycloneDX) | Phase 3 |
| Frontend dashboard (React/Next.js) | Phase 1.5 |
| Azure/GCP billing | Phase 1.5 |
| GitLab/Bitbucket support | Phase 1.5 |
| Browser extension detection | Phase 3 |
| Expense report / procurement scanning | Phase 3 |
