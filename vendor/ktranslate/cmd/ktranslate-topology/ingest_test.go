package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"testing"
	"time"
)

// sampleRecord builds a JCHF-shaped map mirroring what ktranslate's
// json formatter emits.
func sampleRecord(local, localIf, remote, remoteIf, source string) map[string]interface{} {
	return map[string]interface{}{
		"eventType":   topologyEventType,
		"device_name": local,
		"src_addr":    "10.0.0.1",
		"timestamp":   time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC).Unix(),
		"custom_str": map[string]string{
			"neighbor_source":   source,
			"local_if_name":     localIf,
			"remote_sys_name":   remote,
			"remote_port_id":    remoteIf,
			"remote_mgmt_addr":  "10.0.0.99",
			"remote_chassis_id": "00:11:22:33:44:55",
		},
		"custom_bigint": map[string]int64{
			"local_if_index": 101,
		},
	}
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestParseAndApply_AcceptsArrayPayload(t *testing.T) {
	body := mustMarshal(t, []map[string]interface{}{
		sampleRecord("a", "Eth1/1", "b", "Eth2/2", "lldp"),
		sampleRecord("c", "Eth3/3", "d", "Eth4/4", "cdp"),
	})
	// Pin wall clock to the fixture timestamp so TTL pruning doesn't
	// drop the observation out from under us.
	fixedNow := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	g := NewGraph(time.Hour)
	g.now = fixedTime(fixedNow)
	accepted, total, err := ParseAndApply(body, g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accepted != 2 || total != 2 {
		t.Errorf("want 2 accepted/total, got %d/%d", accepted, total)
	}
	if n := g.Snapshot().Counts["devices"]; n != 4 {
		t.Errorf("want 4 devices, got %d", n)
	}
}

func TestParseAndApply_SkipsNonTopologyRecords(t *testing.T) {
	topo := sampleRecord("a", "Eth1", "b", "Eth2", "lldp")
	other := map[string]interface{}{
		"eventType":   "KSnmpInterfaceMetadata",
		"device_name": "a",
		"custom_str":  map[string]string{"SysName": "a"},
	}
	body := mustMarshal(t, []map[string]interface{}{topo, other})

	g := NewGraph(time.Hour)
	accepted, total, err := ParseAndApply(body, g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accepted != 1 {
		t.Errorf("want 1 accepted, got %d", accepted)
	}
	if total != 2 {
		t.Errorf("want 2 total, got %d", total)
	}
}

func TestParseAndApply_GzipBody(t *testing.T) {
	body := mustMarshal(t, []map[string]interface{}{
		sampleRecord("a", "Eth1", "b", "Eth2", "lldp"),
	})
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(body); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	g := NewGraph(time.Hour)
	accepted, _, err := ParseAndApply(buf.Bytes(), g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accepted != 1 {
		t.Errorf("want 1 accepted, got %d", accepted)
	}
}

func TestParseAndApply_AcceptsSingleRecord(t *testing.T) {
	body := mustMarshal(t, sampleRecord("a", "Eth1", "b", "Eth2", "lldp"))
	g := NewGraph(time.Hour)
	accepted, _, err := ParseAndApply(body, g)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accepted != 1 {
		t.Errorf("want 1 accepted for single record, got %d", accepted)
	}
}

func TestParseAndApply_EmptyBody(t *testing.T) {
	g := NewGraph(time.Hour)
	accepted, total, err := ParseAndApply(nil, g)
	if err != nil {
		t.Errorf("empty body should not error, got %v", err)
	}
	if accepted != 0 || total != 0 {
		t.Errorf("want 0/0, got %d/%d", accepted, total)
	}
}

func TestSplitSources_Combined(t *testing.T) {
	if got := splitSources("lldp+cdp"); len(got) != 2 || got[0] != "lldp" || got[1] != "cdp" {
		t.Errorf("want [lldp,cdp], got %v", got)
	}
	if got := splitSources("lldp"); len(got) != 1 || got[0] != "lldp" {
		t.Errorf("want [lldp], got %v", got)
	}
	if got := splitSources(""); got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

func TestObservationFromRaw_FillsAllFields(t *testing.T) {
	r := rawRecord{
		EventType:  topologyEventType,
		DeviceName: "sw-1",
		SrcAddr:    "10.0.0.1",
		Timestamp:  time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC).Unix(),
		CustomStr: map[string]string{
			"neighbor_source":  "lldp+cdp",
			"local_if_name":    "Eth1/1",
			"remote_sys_name":  "peer",
			"remote_port_id":   "Eth2/2",
			"remote_mgmt_addr": "10.0.0.99",
		},
		CustomBig: map[string]int64{
			"local_if_index": 101,
		},
	}
	obs, ok := observationFromRaw(r)
	if !ok {
		t.Fatal("expected observation to be built")
	}
	if obs.LocalDevice != "sw-1" || obs.RemoteDevice != "peer" {
		t.Errorf("device names wrong: %+v", obs)
	}
	if obs.LocalIfIndex != 101 {
		t.Errorf("want ifIndex 101, got %d", obs.LocalIfIndex)
	}
	if len(obs.NeighborSources) != 2 {
		t.Errorf("want 2 sources, got %v", obs.NeighborSources)
	}
	if obs.RemoteMgmtAddr != "10.0.0.99" {
		t.Errorf("mgmt addr missing")
	}
}
