package main

import (
	"testing"
	"time"
)

func fixedTime(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestCanonicalLinkID_SortsEndpoints(t *testing.T) {
	a := Endpoint{Device: "z-switch", Interface: "Eth1/1"}
	b := Endpoint{Device: "a-switch", Interface: "Eth2/2"}

	id1, first1, second1 := canonicalLinkID(a, b)
	id2, first2, second2 := canonicalLinkID(b, a)

	if id1 != id2 {
		t.Errorf("canonical id must be order-independent; got %q vs %q", id1, id2)
	}
	if first1 != first2 || second1 != second2 {
		t.Errorf("endpoint order must be canonical; got (%v,%v) vs (%v,%v)",
			first1, second1, first2, second2)
	}
	if first1.Device != "a-switch" {
		t.Errorf("a-switch should come first alphabetically, got %q", first1.Device)
	}
}

func TestMergeSources_UnionAndSort(t *testing.T) {
	out := mergeSources([]string{"lldp"}, []string{"cdp"})
	if len(out) != 2 || out[0] != "cdp" || out[1] != "lldp" {
		t.Errorf("expected [cdp,lldp], got %v", out)
	}
	// Duplicate-safe.
	out = mergeSources([]string{"lldp", "cdp"}, []string{"lldp"})
	if len(out) != 2 {
		t.Errorf("dedup failed: %v", out)
	}
	// Empty input = empty output.
	if out := mergeSources(nil, nil); len(out) != 0 {
		t.Errorf("expected empty, got %v", out)
	}
}

func TestApply_DedupesReciprocalViews(t *testing.T) {
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	g := NewGraph(time.Hour)
	g.now = fixedTime(now)

	// Device A reports seeing B on its Eth1/1.
	g.Apply(Observation{
		LocalDevice:     "switch-a",
		LocalIfName:     "Eth1/1",
		LocalIfIndex:    101,
		RemoteDevice:    "switch-b",
		RemoteIfName:    "Eth2/2",
		NeighborSources: []string{"lldp"},
		Timestamp:       now,
	})
	// Device B reports the reciprocal view.
	g.Apply(Observation{
		LocalDevice:     "switch-b",
		LocalIfName:     "Eth2/2",
		LocalIfIndex:    202,
		RemoteDevice:    "switch-a",
		RemoteIfName:    "Eth1/1",
		NeighborSources: []string{"cdp"},
		Timestamp:       now,
	})

	snap := g.Snapshot()
	if snap.Counts["devices"] != 2 {
		t.Errorf("want 2 devices, got %d", snap.Counts["devices"])
	}
	if snap.Counts["links"] != 1 {
		t.Fatalf("reciprocal views should collapse into 1 link, got %d", snap.Counts["links"])
	}
	if len(snap.Edges) != 1 {
		t.Fatalf("want 1 edge, got %d", len(snap.Edges))
	}
	edge := snap.Edges[0]
	if len(edge.Sources) != 2 || edge.Sources[0] != "cdp" || edge.Sources[1] != "lldp" {
		t.Errorf("expected edge sources [cdp,lldp], got %v", edge.Sources)
	}

	// Nodes should include both devices plus two interfaces (one per side).
	devCount, ifCount := 0, 0
	for _, n := range snap.Nodes {
		switch n.Kind {
		case "device":
			devCount++
		case "interface":
			ifCount++
		}
	}
	if devCount != 2 || ifCount != 2 {
		t.Errorf("want 2 devices + 2 interfaces, got %d + %d", devCount, ifCount)
	}
}

func TestApply_MissingFieldsDropped(t *testing.T) {
	g := NewGraph(time.Hour)
	g.now = fixedTime(time.Now())

	// No local device name.
	g.Apply(Observation{LocalIfName: "Eth1", RemoteDevice: "r", RemoteIfName: "Eth2"})
	// No remote device.
	g.Apply(Observation{LocalDevice: "a", LocalIfName: "Eth1"})
	// No interface name.
	g.Apply(Observation{LocalDevice: "a", RemoteDevice: "b", RemoteIfName: "Eth2"})

	snap := g.Snapshot()
	if snap.Counts["links"] != 0 {
		t.Errorf("want 0 links, got %d", snap.Counts["links"])
	}
	if snap.Counts["devices"] != 0 {
		t.Errorf("incomplete observations should not produce devices, got %d", snap.Counts["devices"])
	}
}

func TestSnapshot_PrunesStaleLinks(t *testing.T) {
	base := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	g := NewGraph(10 * time.Minute)
	g.now = fixedTime(base)

	g.Apply(Observation{
		LocalDevice:  "a",
		LocalIfName:  "Eth1",
		RemoteDevice: "b",
		RemoteIfName: "Eth2",
		Timestamp:    base,
	})

	// Advance wall clock past the TTL.
	g.now = fixedTime(base.Add(30 * time.Minute))
	snap := g.Snapshot()
	if snap.Counts["links"] != 0 {
		t.Errorf("stale link should be pruned, got %d", snap.Counts["links"])
	}
}

func TestSnapshot_FreshLinksKeepNodes(t *testing.T) {
	base := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	g := NewGraph(10 * time.Minute)
	g.now = fixedTime(base)

	g.Apply(Observation{
		LocalDevice:  "a",
		LocalIfName:  "Eth1",
		RemoteDevice: "b",
		RemoteIfName: "Eth2",
		Timestamp:    base,
	})

	// Advance 5m — still inside the TTL.
	g.now = fixedTime(base.Add(5 * time.Minute))
	snap := g.Snapshot()
	if snap.Counts["links"] != 1 {
		t.Errorf("fresh link should survive, got %d", snap.Counts["links"])
	}
	if snap.Counts["devices"] != 2 {
		t.Errorf("fresh devices should survive, got %d", snap.Counts["devices"])
	}
}

func TestApply_UpdatesDeviceMetadata(t *testing.T) {
	g := NewGraph(time.Hour)
	g.now = fixedTime(time.Now())

	// First observation: no IP, no sysDescr.
	g.Apply(Observation{
		LocalDevice:  "a",
		LocalIfName:  "Eth1",
		RemoteDevice: "b",
		RemoteIfName: "Eth2",
	})
	// Second observation on the same device now has the IP.
	g.Apply(Observation{
		LocalDevice:   "a",
		LocalDeviceIP: "10.0.0.1",
		LocalSysDescr: "Cisco IOS",
		LocalIfName:   "Eth2",
		RemoteDevice:  "c",
		RemoteIfName:  "Eth9",
	})

	snap := g.Snapshot()
	var seenA bool
	for _, n := range snap.Nodes {
		if n.Kind == "device" && n.Label == "a" {
			seenA = true
			if n.IP != "10.0.0.1" {
				t.Errorf("expected IP to be recorded, got %q", n.IP)
			}
			if n.SysDescr != "Cisco IOS" {
				t.Errorf("expected sysDescr, got %q", n.SysDescr)
			}
		}
	}
	if !seenA {
		t.Fatal("device a missing from snapshot")
	}
}
