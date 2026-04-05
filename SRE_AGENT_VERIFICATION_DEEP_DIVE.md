# SRE Agent Outcome Verification — Deep Dive on 10 Core Problems

## The 10 SRE Agent Actions That Matter

For each: what breaks, what the agent does, how it fails silently, and exactly how we verify.

---

## 1. Pod Crash Loop Recovery

**What breaks:** Pod enters CrashLoopBackOff. Kubernetes restarts with exponential backoff. Service degraded or down.

**What agent does:** Bumps memory/CPU limits, rolls back to last known good image, deletes corrupted PVC, or restarts dependent services.

**How it goes wrong silently:**
- Agent bumps memory limits. Pod stops crashing. But root cause is a memory leak — pod will OOM again in 4 hours instead of 10 minutes. Agent declared "fixed."
- Agent rolls back image. Pod stabilizes. But 3 other services depend on the new API contract. They start failing 20 minutes later.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| Crash elimination | `kubectl get events --field-selector reason=BackOff` | 30min | Zero CrashLoopBackOff events |
| Durability | Pod restart count | 4hr | 0 restarts = pass; returns = agent masked problem |
| Resource trajectory | `container_memory_working_set_bytes` | 4hr | Stable = pass; linear climb = leak still present, agent just bought time |
| Downstream health | Error rates/latency of dependent services (service mesh) | 15min | No degradation in dependents |
| Causality | Was pod approaching natural backoff retry? | Immediate | Recovery faster than backoff schedule = causal; aligned with retry = spurious |

---

## 2. Latency Spike Remediation

**What breaks:** p99 latency jumps from 200ms to 5s.

**What agent does:** Scales up replicas, restarts pods, adjusts connection pools, triggers GC, rolls back deploy, or reroutes traffic away from degraded AZ.

**How it goes wrong silently:**
- Agent scales 3→10 replicas. Latency drops. But cause was a slow DB query. Now 10 replicas hammer the DB. DB CPU hits 100% in 20 minutes.
- Agent restarts pod, clearing in-memory cache. Latency recovers. But cache is cold — thundering herd on backend as cache repopulates. Brief recovery, then worse spike.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| Target metric recovery | `histogram_quantile(0.99, http_request_duration_seconds)` | 15min sustained | p99 < threshold, sustained |
| Recovery durability | Same metric | 2hr | Flat at recovered level vs gradual creep back |
| Resource cost | Pod count x instance cost, pre vs post | Immediate | Was scaling proportionate? (3→10 when 3→5 sufficed) |
| Upstream/downstream propagation | Latency of callers and dependencies | 15min | No new latency issues elsewhere |
| Database impact | DB CPU, connection count, query latency | 30min | More replicas = more DB connections. DB health stable? |
| Causality | Traffic volume during spike window | Immediate | Traffic flat + latency dropped after action = causal; traffic spike ended simultaneously = spurious |

---

## 3. Failed Deployment Rollback

**What breaks:** New deployment causes error rate spike, health check failures, or canary degradation.

**What agent does:** `kubectl rollout undo` or Argo Rollout abort.

**How it goes wrong silently:**
- Deploy included a DB migration (added column, changed type). Rollback reverts code but NOT schema. Old code runs against new schema. Silent data corruption.
- Error spike was actually caused by a separate config change, not the deploy. Agent rolls back a perfectly good deploy for no reason.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| Error rate recovery | `rate(http_requests_total{status=~"5.."}[5m])` | 10min | Returned to pre-deploy baseline |
| Deploy was the cause? | Correlate deploy timestamp with error onset | Immediate | Errors within 2min of deploy = correlated; errors started before = wrong call |
| Schema/migration consistency | Check migration history table (Flyway/Alembic) | Immediate | No unapplied migrations = pass; migration present = code/schema mismatch alert |
| Inter-service contract | Error logs of callers for serialization/contract errors | 30min | No contract-related errors |
| Confounding changes | Concurrent config changes, feature flags, infra events within ±10min | Immediate | No confounders = causal; concurrent changes = unclear |

---

## 4. Database Connection Exhaustion Recovery

**What breaks:** Connection pool maxed out. Requests queue, timeout, 5xx.

**What agent does:** Kills idle/long-running queries, bumps pool size, restarts app, or scales down replicas.

**How it goes wrong silently:**
- Agent kills a long-running query that was a critical batch job halfway through 100K records. 50K processed, 50K abandoned with no clean resume.
- Agent bumps pool from 20→50. Works now. But DB has max_connections=100 across 3 replicas. Next scale-up to 4 replicas exhausts DB connections at server level.
- Agent restarts all pods simultaneously. Brief total outage instead of degraded service.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| Connection availability | DB active connections vs max, pool metrics (active, idle, waiting) | 10min | Waiting connections = 0, pool healthy |
| Query collateral | If queries killed — were any batch/ETL jobs? Check pg_stat_activity snapshot | Immediate | Only killed idle/leaked = pass; killed batch jobs = alert |
| Headroom check | (max_connections - active) / total_replicas | Immediate | >20% headroom = pass; <10% = fragile state |
| Rolling restart safety | Ready replica count over time during restart | 5min | Always ≥1 ready replica; 0 ready = brief total outage |
| Error pattern shift | 5xx rate and error types post-action | 15min | Baseline errors = pass; new error types = fix introduced different problem |

---

## 5. Certificate Expiration / TLS Failure Recovery

**What breaks:** TLS cert expires or misconfigured. All HTTPS traffic fails.

**What agent does:** Triggers cert renewal, rotates to backup cert, updates ingress cert reference, restarts ingress controller.

**How it goes wrong silently:**
- Agent renews cert but for wrong domain (wildcard vs specific subdomain). Most traffic works. One critical subdomain broken.
- Agent updates cert in ingress but not in service mesh sidecar. External traffic works. Internal mTLS between services breaks silently.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| TLS handshake | Active probe — attempt TLS connection, verify cert chain, check expiry | 5min (propagation) | Valid cert, correct domain, >30d to expiry |
| Domain coverage | Extract SANs from new cert, compare against all routing domains | Immediate | All domains covered; missing domains = partial fix |
| Internal mTLS | Istio/Linkerd proxy logs for TLS handshake failures | 10min | No mTLS errors between services |
| Connection disruption | Client connection resets / 503s during ingress restart | 5min | <1% error rate increase |
| Full chain validation | Intermediate certs, root CA trust, OCSP stapling | Immediate | Full chain valid; incomplete chain = some clients will reject |

---

## 6. Disk Pressure / Storage Full Recovery

**What breaks:** Node or PVC hits storage limit. Pods evicted (DiskPressure taint). App can't write.

**What agent does:** Clears old logs/temp files/container images, expands PVC, evicts pods to different nodes, triggers log rotation.

**How it goes wrong silently:**
- Agent deletes "old logs" needed for active investigation or compliance retention.
- Agent clears container image cache. Next deploy takes 10x longer because every image pulled fresh.
- Agent expands PVC from 100→200Gi. But disk is filling at 5Gi/day from a log-spamming bug. In 20 days, same crisis.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| Disk pressure resolved | `node_filesystem_avail_bytes / node_filesystem_size_bytes` | Immediate | >20% free = pass; 10-20% = bought time; <10% = minimal impact |
| Fill rate trajectory | Disk usage linear regression | 24hr | Normal fill rate = pass; unchanged rate = cause still active, calculate time to next crisis |
| Pod eviction collateral | `kubectl get events --field-selector reason=Evicted` | 30min | No unintended evictions |
| Data retention compliance | Cross-reference deleted files with retention policies | Immediate | All deletions within policy = pass; violated retention = alert |
| Durability | Disk usage at 24h, 48h, 7d | 7d | Stable = pass; returned to critical within 48h = underlying cause unaddressed |

---

## 7. Runaway Autoscaler / Resource Quota Exhaustion

**What breaks:** HPA scales to max. Cluster quota hit. New pods pending. Or autoscaler creates nodes uncontrollably, cloud bill explodes.

**What agent does:** Adjusts HPA max, kills runaway scaling trigger, patches resource requests/limits, manually scales down.

**How it goes wrong silently:**
- Agent lowers HPA max from 50→10. Scaling stops. But there was genuine traffic demand — service now under-provisioned. Latency rises but stays below alert threshold.
- Agent kills "runaway" queue consumer. Scaling stops. But queue was legitimately backed up. Events never processed. Data loss.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| Scaling stabilized | HPA current replicas, node count over time | 30min | Stable at reasonable level; still oscillating = fail |
| Service quality preserved | p50/p99 latency, error rate, throughput | 1hr | All SLOs met; degraded = over-corrected |
| Queue/event health | Queue depth, consumer lag, message age | 1hr | Queue draining normally; growing = killed legitimate consumer |
| Cost impact | Node count x instance cost, pre vs post | Immediate | $/hr saved. Was runaway real or metric blip? |
| OOM risk | `container_memory_working_set_bytes / limits` | Next traffic peak | Peak <80% of new limit = pass; >90% = will OOM under load |

---

## 8. Service Mesh / Network Policy Failure Recovery

**What breaks:** Service-to-service communication fails. Istio sidecar misconfigured, network policy too restrictive, DNS resolution failure.

**What agent does:** Restarts Istio sidecar, patches network policy, restarts CoreDNS, rolls back network policy change.

**How it goes wrong silently:**
- Agent loosens network policy. Traffic flows. But now a service that shouldn't be reachable from public internet is exposed. Security hole.
- Agent restarts CoreDNS. DNS works. But brief window with cold caches — thundering herd causes second, worse outage.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| Connectivity restored | Synthetic probes — test requests between affected services | 5min | All inter-service calls succeeding |
| Security posture | Diff network policies before vs after. Any WIDENED access? | Immediate | No new permissive rules; opened new paths = security review alert |
| DNS stability | CoreDNS request rate, NXDOMAIN rate, resolution latency | 15min | Baseline metrics; elevated NXDOMAIN = partial fix |
| Blast radius | Error rates across ALL services | 10min | No collateral degradation |
| Root cause match | Recent deploys, policy changes, cert rotations before incident | Immediate | Agent addressed actual cause = causal; patched around it = symptomatic, will recur |

---

## 9. Memory Leak / OOMKill Recovery

**What breaks:** Container memory usage climbs linearly until OOMKilled. Sawtooth pattern on restart.

**What agent does:** Bumps memory limits, rolling restart, rolls back to previous image, enables GC tuning.

**How it goes wrong silently:**
- Agent bumps memory 1→4Gi. Pod stops OOMKilling. But memory still leaking — takes 8hr instead of 2hr. At 3am it crashes, and 4x memory footprint causes node-level pressure, evicting other pods.
- Agent rolls back to previous version. Leak gone. But new version contained a critical security patch. Now running known-vulnerable code.
- Agent sets up periodic restarts as workaround. Each restart drops in-flight requests. Users see periodic failures nobody investigates.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| OOMKill eliminated | `kubectl get events --field-selector reason=OOMKilling` | 8hr (longer than leak cycle) | Zero OOMKills = pass; recurred = only delayed |
| Memory trajectory | `container_memory_working_set_bytes` linear regression | 8hr | Flat = fixed; climbing = leak present, calculate new time-to-OOM |
| Version security check | Compare rolled-back image against vulnerability DB (Trivy/Grype) | Immediate | No critical CVEs = pass; known vulns = security alert |
| Request continuity | Dropped requests / 5xx spikes correlated with restart times | Across 3 cycles | Zero drops = pass; drops on each restart = ongoing user impact |
| Node memory headroom | `node_memory_MemAvailable_bytes` | Immediate | >30% available = pass; approaching pressure = agent's fix will cause node problems |

---

## 10. Cascading Failure / Circuit Breaker Intervention

**What breaks:** Service A fails → B retries → B's thread pool exhausts → B fails → C fails. Classic cascade.

**What agent does:** Opens circuit breakers, sheds load, isolates failing service, scales intermediate services, restarts root cause.

**How it goes wrong silently:**
- Agent opens circuit breaker B→A. Cascade stops. But breaker stays open permanently (no half-open probe configured). Users of B permanently lose features backed by A — even after A recovers.
- Agent rate-limits inbound traffic. Cascade stops. But limit too aggressive — 40% of legitimate traffic dropped. Users see intermittent failures. No alert because errors below threshold.
- Agent restarts A. A comes back. But all retrying services simultaneously succeed — thundering herd takes A back down in 30 seconds.

**Outcome probes:**

| Probe | Query | Window | Pass/Fail |
|---|---|---|---|
| Cascade terminated | Error rates and latency for EACH service in dependency chain | 15min | All at baseline; some still recovering but trending down = partial |
| Circuit breaker lifecycle | Breaker state for affected service pairs | 30min, 2hr, 8hr | Closed (recovered) = pass; stuck open = feature degraded, needs human |
| Traffic completeness | Inbound request rate pre-incident vs current | 1hr | Within 5% of pre-incident; significant drop = shedding legitimate traffic |
| Thundering herd check | Root cause service metrics after restart/recovery | 5min post-recovery | Gradual ramp = pass; >3x spike = thundering herd risk |
| End-to-end user impact | Synthetic transaction success rate (full user journey) | 30min | >99% succeeding; services healthy individually but journey broken = fixed symptoms not path |

---

## The Universal Failure Taxonomy

Every SRE agent failure follows one of 7 patterns. This taxonomy IS the product:

| Failure Mode | Meaning | Detection Method |
|---|---|---|
| **False resolution** | Agent declared fixed, metric didn't actually recover | Probe the actual trigger metric with sustained window |
| **Delayed recurrence** | Fixed for now, breaks again in hours/days | Extended durability probes (4h, 24h, 7d) |
| **Symptom masking** | Agent patched over the real problem | Trajectory analysis — is underlying cause still progressing? |
| **Collateral damage** | Fix broke something else | Blast radius probes on dependent/related services |
| **Spurious attribution** | Agent took credit for natural recovery | Causal analysis (slope before vs after, confounding events) |
| **Security regression** | Fix opened attack surface or reverted security patch | Security posture diff (network policies, CVE check, access audit) |
| **Disproportionate response** | Agent over-corrected (10x scale when 2x sufficed) | Cost and resource analysis post-action |

We don't need 50 custom probes per action type. We need 7 probe categories that apply universally, with domain-specific query implementations.
