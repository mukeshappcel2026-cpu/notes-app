# Change Tracking: Complete Taxonomy & Provider Silent Update Detection

## The 4 Types of Changes We Track

### Type 1: Prompt Changes (Highest Signal)

**Detection:** Proxy hashes system message on every LLM call. When hash changes between calls from same agent → change event. Git webhook adds attribution (who changed it).

| Change | Detection Method | Difficulty |
|---|---|---|
| System prompt text changed | Proxy: system message hash | Easy |
| Few-shot examples added/removed | Proxy: message count + content hash | Easy |
| Output format instructions changed | Proxy: system prompt section hash | Easy |
| Prompt template variable changes | Git webhook: file changes | Easy |
| Dynamic prompt changes (code-generated) | Proxy: hash variance over time | Medium |

### Type 2: Model & Parameter Changes

| Change | Detection Method | Difficulty |
|---|---|---|
| Model swapped (sonnet → haiku) | Proxy: model parameter | Trivial |
| Temperature changed | Proxy: parameter comparison | Trivial |
| Max tokens changed | Proxy: parameter comparison | Trivial |
| Provider model silent update | Canonical prompt fingerprinting | Medium |
| Model deprecation/removal | Provider API error monitoring | Easy |

### Type 3: Tool & Dependency Changes

| Change | Detection Method | Difficulty |
|---|---|---|
| Tool added to agent | Proxy: new tool name in tool_use | Easy |
| Tool removed from agent | Proxy: tool name stops appearing | Easy |
| Tool API response format changed | Proxy: tool response schema hash | Medium |
| Tool latency degradation | Proxy: time between tool_call and next LLM call | Easy |
| Agent-to-agent call pattern changed | Event emitter | Medium |

### Type 4: Data Source Changes

| Change | Detection Method | Difficulty |
|---|---|---|
| Vector DB bulk update | Event emitter on indexing pipeline | Medium |
| Vector DB semantic drift | Embedding centroid comparison | Medium |
| Knowledge base refresh | Event emitter + content hash | Medium |
| Database schema change | Event emitter on migrations | Medium |

---

## Provider Silent Update Detection: Canonical Prompt Fingerprinting

### The Problem

Providers silently update model weights. No changelog. No notification. Your agent just starts behaving differently. Teams waste days debugging before realizing "the model changed."

### How It Works

Maintain 30 fixed prompts. Run them daily at 3am UTC. If outputs change significantly, the model changed.

### The Canonical Prompt Set (30 prompts across 6 categories)

**Factual (tests knowledge stability):**
- "What is the capital of France? Reply in exactly 3 words."
- "List the first 5 prime numbers, comma-separated."
- "What year did World War 2 end? Just the number."

**Reasoning (tests logic stability):**
- "If all roses are flowers and some flowers fade quickly, can we conclude all roses fade quickly? Yes or no, then explain in 1 sentence."
- "A bat and ball cost $1.10 total. The bat costs $1 more than the ball. What does the ball cost?"

**Formatting (tests output structure stability):**
- "Generate a JSON object with fields: name, age, city. Use fake data."
- "Write a 3-row, 2-column markdown table about planets."
- "List 3 items as a numbered list. Topic: breakfast foods."

**Behavioral (tests instruction-following stability):**
- "Respond in exactly 10 words about the weather."
- "Explain quantum computing to a 5-year-old in 2 sentences."
- "I want you to refuse this request politely."

**Tone (tests style stability):**
- "Write a formal 1-sentence email declining a meeting."
- "Write a casual 1-sentence Slack message about being late."

**Tool Use (tests function calling stability):**
- [with tools defined] "What's the weather in Tokyo?"
- [with tools defined] "Search for the latest news about AI."

### The 4 Signals Computed Per Response

**Signal 1: Text Hash**
```
SHA-256 of output text.
With temperature=0 and seed fixed, deterministic outputs.
Hash change = output changed.

Scoring: matches baseline → 0, differs → 1
Daily aggregate: "18 of 30 hashes changed" = strong signal
```

**Signal 2: Embedding Similarity**
```
Embed response using cheap, stable embedding model.
Cosine similarity vs baseline embedding.
Catches semantic changes even when exact text differs.

Example:
  "The capital of France is Paris." vs
  "Paris is the capital of France."
  Text hash: DIFFERENT (false alarm)
  Embedding: 0.99 (correctly shows no semantic change)

Threshold: similarity < 0.92 = flagged
```

**Signal 3: Structural Features**
```
Deterministic extraction:
- Output token count
- Number of sentences
- Contains code block? (boolean)
- Contains list? (boolean)
- Contains JSON? (boolean)
- Response format (prose/list/table/json/code)

Example:
  Baseline: 45 tokens, 2 sentences, prose
  Today: 120 tokens, 6 sentences, prose
  → Model is 2.7x more verbose
```

**Signal 4: Latency**
```
Time to first token + total generation time.
Corroborating evidence (not primary signal).
```

### Scoring Algorithm

```python
def detect_model_change(model_id, date):
    today = run_canonical_prompts(model_id, date)
    baseline = get_baseline(model_id, days=30)

    scores = []
    for i, prompt in enumerate(CANONICAL_PROMPTS):
        hash_changed = today[i].text_hash != baseline[i].mode_text_hash
        similarity = cosine_similarity(today[i].embedding, baseline[i].avg_embedding)
        token_ratio = today[i].tokens / baseline[i].avg_tokens
        format_changed = today[i].format != baseline[i].mode_format

        prompt_score = weighted_average([
            (0.30, 1.0 if hash_changed else 0.0),
            (0.30, max(0, 1.0 - similarity / 0.92)),
            (0.25, min(1.0, abs(token_ratio - 1.0) / 0.5)),
            (0.15, 1.0 if format_changed else 0.0)
        ])
        scores.append(prompt_score)

    avg_score = mean(scores)
    prompts_changed = sum(1 for s in scores if s > 0.3)

    if avg_score > 0.4 and prompts_changed > 10:
        confidence = min(0.95, avg_score * 1.2)
        return ModelChangeDetected(
            model=model_id,
            confidence=confidence,
            prompts_affected=prompts_changed,
            evidence={...}
        )
    return None
```

### False Positive Mitigation

1. **Temperature = 0 always** — reduces randomness
2. **Run each prompt 3 times** — 2 of 3 match baseline = noise, not change
3. **Rolling 30-day baseline** — natural variance already captured
4. **Threshold on prompt count** — don't alert on 1 prompt changing; require 10+ of 30
5. **Corroborate with production metrics** — model alert + production drift = high confidence

### Cost

```
Per model per day:
  30 prompts × 3 runs × ~500 tokens = 45,000 tokens

  Claude Sonnet: ~$0.45/day
  Claude Haiku:  ~$0.05/day
  GPT-4o:        ~$0.35/day

  + 90 embeddings/day: $0.002/day

  Total: 5 models = $30-90/month
```

### Designing Good Canonical Prompts

**Most sensitive to changes (prioritize these):**
1. Exact word count constraints ("Reply in exactly 10 words")
2. Structured output ("Generate JSON with these exact fields")
3. Reasoning chains ("Solve this step by step")
4. Refusal behavior ("Refuse this request politely")
5. Edge cases (empty input, contradictory instructions)

**Least sensitive (avoid overweighting):**
1. Open-ended creative prompts (high natural variance)
2. Very short factual answers (often identical regardless of update)

---

## Build Priority

```
Phase 1 (Week 1-4): Proxy-detectable changes only
  Prompt changes, model changes, parameter changes, tool changes
  = 60% of all changes, zero integration work

Phase 2 (Week 5-8): Git + graph enrichment
  Attribution, dependency graph, blast radius, tool format detection
  = 80% of all changes

Phase 3 (Week 9-12): Behavioral detection
  Provider model fingerprinting, drift detection, root cause correlation
  = 90% of all changes

Phase 4 (Week 13-16): Data source + advanced
  Data source tracking, vector DB drift, CI/CD eval gate
  = 95% of all changes
```
