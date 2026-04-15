# ktranslate topology feature — patch series

These 9 patches apply the LLDP/CDP topology discovery feature and the
`cmd/ktranslate-topology` live viewer onto a clean
[kentik/ktranslate](https://github.com/kentik/ktranslate) tree at
upstream commit `4c7af5a61824c19d672b260b7e923cafd30c6798` (the same
commit that
[`mukeshappcel2026-cpu/ktranslate`](https://github.com/mukeshappcel2026-cpu/ktranslate)
was forked from).

They were produced by `git format-patch --relative=vendor/ktranslate`
against the `claude/fork-ktranslate-WRlwX` branch in this repo, so the
paths inside each patch are already stripped of the `vendor/ktranslate/`
prefix and will `git am` straight into a ktranslate checkout.

## What's in the series

| # | Commit | What it does |
|---|---|---|
| 0001 | ktranslate: add topology neighbor scaffolding | `kt.TopologyNeighbor` type, config flag, stub `PollTopology`, metadata poll wiring, `toFlows` extension |
| 0002 | ktranslate: implement CDP neighbor walk | Walks `cdpCacheEntry`, unit tests |
| 0003 | ktranslate: implement LLDP neighbor walk | Walks `lldpLocPortEntry` + `lldpRemEntry`, local-port resolution, chassis/port id decoding |
| 0004 | ktranslate: document topology discovery in README and config sample | README section + `config/snmp.yaml.sample` example |
| 0005 | ktranslate: surface LLDP management addresses on neighbors | `lldpRemManAddrEntry` walk |
| 0006 | ktranslate: decode topology capability bitmaps into readable names | Semantic LLDP + CDP capability decoders |
| 0007 | ktranslate: integration tests for topology walks via fake walker | End-to-end tests via `SNMPTestWalker` |
| 0008 | ktranslate-topology: live web viewer for SNMP topology data | New `cmd/ktranslate-topology` binary + embedded HTML viewer |
| 0009 | ktranslate-topology: add -demo flag with simulated enterprise topology | `-demo` flag seeding 17 devices / 19 links |

## Applying them to your ktranslate fork

```bash
# 1. Clone the fork
git clone https://github.com/mukeshappcel2026-cpu/ktranslate.git
cd ktranslate

# 2. Grab the patches from this notes-app repo (either clone notes-app
#    or curl them one by one). Assuming a local clone at ~/src/notes-app:
PATCHES=~/src/notes-app/ops/ktranslate-patches

# 3. Create a feature branch and apply the series
git checkout -b claude/topology-feature
git am "$PATCHES"/*.patch

# 4. Verify it builds and tests pass
go generate ./pkg/version/...
go build ./...
go test ./cmd/ktranslate-topology/... ./pkg/inputs/snmp/metadata/... ./pkg/kt/...

# 5. Push to the fork
git push -u origin claude/topology-feature
```

## Trying the demo

```bash
go build -o ktranslate-topology ./cmd/ktranslate-topology
./ktranslate-topology -listen :8082 -demo
```

Then open <http://localhost:8082/> for a 17-device / 19-link simulated
enterprise topology with a live force-directed graph.

## Wiring it to real SNMP data

In your snmp yaml, enable neighbor discovery per device:

```yaml
devices:
  core-sw-1:
    device_ip: 10.10.0.2
    snmp_comm: public
    discover_neighbors: true
    # Optional: restrict to one protocol; omit to walk both.
    # neighbor_protocols: [lldp]
```

Then run the main ktranslate binary with the HTTP sink pointed at the
viewer:

```bash
ktranslate \
  -format json \
  -sinks http \
  -http_url http://localhost:8082/ingest \
  -snmp /etc/ktranslate/snmp.yaml
```

Every `KSnmpTopology` record ktranslate emits becomes a node/edge in
the viewer, deduplicated across reciprocal views and aged out after the
viewer's `-ttl` window (default 2h).
