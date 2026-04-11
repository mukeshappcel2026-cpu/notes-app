package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// topologyEventType must match pkg/kt.KENTIK_EVENT_SNMP_TOPOLOGY. It's
// hard-coded here so the viewer binary doesn't need to import all of
// pkg/kt (which pulls in gosnmp and a long transitive chain).
const topologyEventType = "KSnmpTopology"

// rawRecord mirrors just enough of the kt.JCHF JSON shape to pull the
// topology fields back out. Unknown fields are ignored so schema drift on
// unrelated fields doesn't break ingest.
type rawRecord struct {
	EventType  string            `json:"eventType"`
	DeviceName string            `json:"device_name"`
	SrcAddr    string            `json:"src_addr"`
	Timestamp  int64             `json:"timestamp"`
	CustomStr  map[string]string `json:"custom_str"`
	CustomBig  map[string]int64  `json:"custom_bigint"`
}

// ParseAndApply decodes a JSON payload produced by ktranslate's
// -format=json sink, filters it down to topology records, and applies
// each one to the graph. The payload is expected to be a JSON array of
// JCHF records (what `json.Marshal(msgs)` produces in pkg/formats/json).
// Gzip-compressed bodies are transparently decoded.
//
// Returns (accepted, total, err) where accepted counts records that
// actually contributed to the graph and total counts everything in the
// payload (including non-topology records we skipped).
func ParseAndApply(body []byte, g *Graph) (int, int, error) {
	if len(body) == 0 {
		return 0, 0, nil
	}

	// Best-effort gzip detection. The http sink gzip flag lives on
	// the main binary's side; we sniff the magic number instead of
	// requiring the user to declare it.
	if len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b {
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return 0, 0, fmt.Errorf("gzip reader: %w", err)
		}
		defer r.Close()
		decoded, err := io.ReadAll(r)
		if err != nil {
			return 0, 0, fmt.Errorf("gzip decode: %w", err)
		}
		body = decoded
	}

	var recs []rawRecord
	if err := json.Unmarshal(body, &recs); err != nil {
		// Also accept a single record (convenient for curl testing).
		var single rawRecord
		if err2 := json.Unmarshal(body, &single); err2 == nil {
			recs = []rawRecord{single}
		} else {
			return 0, 0, fmt.Errorf("json decode: %w", err)
		}
	}

	accepted := 0
	for _, r := range recs {
		if r.EventType != topologyEventType {
			continue
		}
		obs, ok := observationFromRaw(r)
		if !ok {
			continue
		}
		g.Apply(obs)
		accepted++
	}
	return accepted, len(recs), nil
}

// observationFromRaw lifts the topology-relevant fields out of a single
// raw record. It returns ok=false when mandatory fields are missing.
func observationFromRaw(r rawRecord) (Observation, bool) {
	cs := r.CustomStr
	cb := r.CustomBig

	local := firstNonEmpty(r.DeviceName, r.SrcAddr)
	if local == "" {
		return Observation{}, false
	}

	localIfName := cs["local_if_name"]
	if localIfName == "" {
		return Observation{}, false
	}

	remote := firstNonEmpty(
		cs["remote_sys_name"],
		cs["remote_chassis_id"],
		cs["remote_mgmt_addr"],
	)
	if remote == "" {
		return Observation{}, false
	}

	remoteIf := firstNonEmpty(cs["remote_port_id"], cs["remote_port_desc"])
	if remoteIf == "" {
		// No port info — synthesise a placeholder so the link still
		// terminates on a distinct interface node instead of floating.
		remoteIf = "?"
	}

	sources := splitSources(cs["neighbor_source"])
	ts := time.Unix(r.Timestamp, 0)
	if r.Timestamp == 0 {
		ts = time.Time{}
	}

	return Observation{
		LocalDevice:     local,
		LocalDeviceIP:   r.SrcAddr,
		LocalIfName:     localIfName,
		LocalIfIndex:    cb["local_if_index"],
		RemoteDevice:    remote,
		RemoteIfName:    remoteIf,
		RemoteMgmtAddr:  cs["remote_mgmt_addr"],
		NeighborSources: sources,
		Timestamp:       ts,
	}, true
}

// splitSources handles both the single-protocol ("lldp") and combined
// ("lldp+cdp") neighbor_source values that parseLLDPNeighbors / mergeNeighbors
// emit.
func splitSources(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, "+")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
