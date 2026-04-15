package main

import (
	"strings"
	"testing"
	"time"
)

func TestSeedDemo_ProducesExpectedShape(t *testing.T) {
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	g := NewGraph(time.Hour)
	g.now = fixedTime(now)

	seedDemo(g, now)
	snap := g.Snapshot()

	if got, want := snap.Counts["devices"], len(demoDevices); got != want {
		t.Errorf("devices: want %d, got %d", want, got)
	}
	if got, want := snap.Counts["links"], len(demoLinks); got != want {
		t.Errorf("links: want %d (one per demoLink entry after dedup), got %d", want, got)
	}

	// Every device must show up as a "device" node with an IP populated.
	// If metadata propagation broke this would catch it.
	byLabel := make(map[string]SnapshotNode, len(snap.Nodes))
	for _, n := range snap.Nodes {
		if n.Kind == "device" {
			byLabel[n.Label] = n
		}
	}
	for _, d := range demoDevices {
		n, ok := byLabel[d.Name]
		if !ok {
			t.Errorf("device %q missing from snapshot", d.Name)
			continue
		}
		if n.IP != d.IP {
			t.Errorf("device %q: want IP %q, got %q", d.Name, d.IP, n.IP)
		}
		if n.SysDescr != d.SysDescr {
			t.Errorf("device %q: sysDescr missing or wrong", d.Name)
		}
	}
}

func TestSeedDemo_MergesLLDPAndCDPOnSameLink(t *testing.T) {
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	g := NewGraph(time.Hour)
	g.now = fixedTime(now)

	seedDemo(g, now)
	snap := g.Snapshot()

	// The ap-2 link is defined with {"lldp","cdp"} so both protocols
	// should show up on its single collapsed edge.
	var found bool
	for _, e := range snap.Edges {
		if strings.Contains(e.ID, "ap-2") {
			found = true
			if len(e.Sources) != 2 {
				t.Errorf("ap-2 edge: want 2 sources, got %v", e.Sources)
			}
			hasLLDP, hasCDP := false, false
			for _, s := range e.Sources {
				if s == "lldp" {
					hasLLDP = true
				}
				if s == "cdp" {
					hasCDP = true
				}
			}
			if !hasLLDP || !hasCDP {
				t.Errorf("ap-2 edge sources missing one: %v", e.Sources)
			}
		}
	}
	if !found {
		t.Error("ap-2 edge missing from snapshot")
	}
}

func TestSeedDemo_InterfacesAreAttachedToParents(t *testing.T) {
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	g := NewGraph(time.Hour)
	g.now = fixedTime(now)

	seedDemo(g, now)
	snap := g.Snapshot()

	deviceIDs := make(map[string]struct{})
	for _, n := range snap.Nodes {
		if n.Kind == "device" {
			deviceIDs[n.ID] = struct{}{}
		}
	}

	// Every interface node must reference a real device parent.
	for _, n := range snap.Nodes {
		if n.Kind != "interface" {
			continue
		}
		if n.Parent == "" {
			t.Errorf("interface %q has no parent", n.Label)
			continue
		}
		if _, ok := deviceIDs[n.Parent]; !ok {
			t.Errorf("interface %q references missing parent %q", n.Label, n.Parent)
		}
	}
}

func TestSeedDemo_DistAccessLinksAreLLDPOnly(t *testing.T) {
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	g := NewGraph(time.Hour)
	g.now = fixedTime(now)

	seedDemo(g, now)
	snap := g.Snapshot()

	// Spot check: a dist-to-access link should be tagged lldp only, to
	// exercise the single-source render path.
	for _, e := range snap.Edges {
		if strings.Contains(e.ID, "dist-sw1") && strings.Contains(e.ID, "acc-sw1") {
			if len(e.Sources) != 1 || e.Sources[0] != "lldp" {
				t.Errorf("dist-sw1<->acc-sw1: want [lldp], got %v", e.Sources)
			}
			return
		}
	}
	t.Error("expected dist-sw1<->acc-sw1 edge not found")
}
