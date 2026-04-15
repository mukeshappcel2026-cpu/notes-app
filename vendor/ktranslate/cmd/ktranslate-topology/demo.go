package main

import (
	"context"
	"time"
)

// demoDevice describes one simulated device. IP and sysDescr flow into
// the graph via Observation.LocalDeviceIP / LocalSysDescr when the device
// appears as the local side of an observation, so every device in the
// fixture must appear as the local side of at least one link.
type demoDevice struct {
	Name     string
	IP       string
	SysDescr string
}

// demoLink describes a simulated adjacency. A single entry produces two
// observations (one per endpoint) which the graph store collapses into a
// single edge via its canonical link id. Sources can be {"lldp"},
// {"cdp"}, or {"lldp","cdp"} to exercise the LLDP/CDP merge path.
type demoLink struct {
	A, B     string // device names
	AIf, BIf string // interface names
	AIdx     int64  // ifIndex on A's side
	BIdx     int64  // ifIndex on B's side
	Sources  []string
}

// demoDevices is a realistic mix of core routers, distribution and access
// switches, plus edge devices (phones, APs, a server, a workstation). IPs
// and sysDescr strings mirror what a real SNMP poll of Cisco gear would
// return so the tooltips in the viewer look authentic.
var demoDevices = []demoDevice{
	{"core-r1", "10.0.0.1", "Cisco IOS XE Software, ASR 1001-HX, Version 17.09.03"},
	{"core-r2", "10.0.0.2", "Cisco IOS XE Software, ASR 1001-HX, Version 17.09.03"},

	{"dist-sw1", "10.0.10.1", "Cisco IOS Software, Catalyst 9500, Version 17.09"},
	{"dist-sw2", "10.0.10.2", "Cisco IOS Software, Catalyst 9500, Version 17.09"},
	{"dist-sw3", "10.0.10.3", "Cisco IOS Software, Catalyst 9500, Version 17.09"},

	{"acc-sw1", "10.0.20.1", "Cisco IOS Software, Catalyst 9300, Version 17.09"},
	{"acc-sw2", "10.0.20.2", "Cisco IOS Software, Catalyst 9300, Version 17.09"},
	{"acc-sw3", "10.0.20.3", "Cisco IOS Software, Catalyst 9300, Version 17.09"},
	{"acc-sw4", "10.0.20.4", "Cisco IOS Software, Catalyst 9300, Version 17.09"},
	{"acc-sw5", "10.0.20.5", "Cisco IOS Software, Catalyst 9300, Version 17.09"},
	{"acc-sw6", "10.0.20.6", "Cisco IOS Software, Catalyst 9300, Version 17.09"},

	{"phone-1", "10.0.30.10", "Cisco IP Phone 8841"},
	{"phone-2", "10.0.30.11", "Cisco IP Phone 8841"},
	{"ap-1", "10.0.30.20", "Cisco Aironet AP2802i Version 17.09"},
	{"ap-2", "10.0.30.21", "Cisco Aironet AP2802i Version 17.09"},
	{"srv-1", "10.0.30.30", "Linux srv-1 6.1.0 x86_64 (lldpd)"},
	{"ws-1", "10.0.30.40", "Windows 11 Pro workstation"},
}

// demoLinks describes the adjacency graph. Core routers carry an HSRP
// mesh; the core-to-distribution fabric is fully meshed (each core talks
// to every dist); dist-to-access is hierarchical; access-to-edge has a
// single uplink per edge device.
var demoLinks = []demoLink{
	// Core mesh
	{A: "core-r1", AIf: "Ten0/0/0", AIdx: 1, B: "core-r2", BIf: "Ten0/0/0", BIdx: 1, Sources: []string{"lldp", "cdp"}},

	// Core -> Distribution (full mesh)
	{A: "core-r1", AIf: "Ten0/1/0", AIdx: 2, B: "dist-sw1", BIf: "Ten1/1/1", BIdx: 101, Sources: []string{"lldp", "cdp"}},
	{A: "core-r1", AIf: "Ten0/1/1", AIdx: 3, B: "dist-sw2", BIf: "Ten1/1/1", BIdx: 101, Sources: []string{"lldp", "cdp"}},
	{A: "core-r1", AIf: "Ten0/1/2", AIdx: 4, B: "dist-sw3", BIf: "Ten1/1/1", BIdx: 101, Sources: []string{"lldp", "cdp"}},
	{A: "core-r2", AIf: "Ten0/1/0", AIdx: 2, B: "dist-sw1", BIf: "Ten1/1/2", BIdx: 102, Sources: []string{"lldp", "cdp"}},
	{A: "core-r2", AIf: "Ten0/1/1", AIdx: 3, B: "dist-sw2", BIf: "Ten1/1/2", BIdx: 102, Sources: []string{"lldp", "cdp"}},
	{A: "core-r2", AIf: "Ten0/1/2", AIdx: 4, B: "dist-sw3", BIf: "Ten1/1/2", BIdx: 102, Sources: []string{"lldp", "cdp"}},

	// Distribution -> Access (LLDP-only, common in greenfield fabrics)
	{A: "dist-sw1", AIf: "Ten1/2/1", AIdx: 201, B: "acc-sw1", BIf: "Ten1/1/49", BIdx: 149, Sources: []string{"lldp"}},
	{A: "dist-sw1", AIf: "Ten1/2/2", AIdx: 202, B: "acc-sw2", BIf: "Ten1/1/49", BIdx: 149, Sources: []string{"lldp"}},
	{A: "dist-sw2", AIf: "Ten1/2/1", AIdx: 201, B: "acc-sw3", BIf: "Ten1/1/49", BIdx: 149, Sources: []string{"lldp"}},
	{A: "dist-sw2", AIf: "Ten1/2/2", AIdx: 202, B: "acc-sw4", BIf: "Ten1/1/49", BIdx: 149, Sources: []string{"lldp"}},
	{A: "dist-sw3", AIf: "Ten1/2/1", AIdx: 201, B: "acc-sw5", BIf: "Ten1/1/49", BIdx: 149, Sources: []string{"lldp"}},
	{A: "dist-sw3", AIf: "Ten1/2/2", AIdx: 202, B: "acc-sw6", BIf: "Ten1/1/49", BIdx: 149, Sources: []string{"lldp"}},

	// Access -> Edge
	{A: "acc-sw1", AIf: "Gi1/0/1", AIdx: 1, B: "phone-1", BIf: "SW-1", BIdx: 1, Sources: []string{"cdp"}},
	{A: "acc-sw2", AIf: "Gi1/0/1", AIdx: 1, B: "phone-2", BIf: "SW-1", BIdx: 1, Sources: []string{"cdp"}},
	{A: "acc-sw2", AIf: "Gi1/0/2", AIdx: 2, B: "ws-1", BIf: "eth0", BIdx: 1, Sources: []string{"lldp"}},
	{A: "acc-sw3", AIf: "Gi1/0/1", AIdx: 1, B: "srv-1", BIf: "eth0", BIdx: 1, Sources: []string{"lldp"}},
	{A: "acc-sw4", AIf: "Gi1/0/1", AIdx: 1, B: "ap-1", BIf: "GigabitEthernet0", BIdx: 1, Sources: []string{"cdp"}},
	{A: "acc-sw5", AIf: "Gi1/0/1", AIdx: 1, B: "ap-2", BIf: "GigabitEthernet0", BIdx: 1, Sources: []string{"lldp", "cdp"}},
}

// seedDemo applies every link in the fixture to the graph. Each link
// produces two observations (one per endpoint) so reciprocal-view
// deduping and metadata propagation both get exercised.
func seedDemo(g *Graph, now time.Time) {
	devByName := make(map[string]demoDevice, len(demoDevices))
	for _, d := range demoDevices {
		devByName[d.Name] = d
	}

	emit := func(local, remote demoDevice, localIf, remoteIf string, localIdx int64, sources []string) {
		g.Apply(Observation{
			LocalDevice:     local.Name,
			LocalDeviceIP:   local.IP,
			LocalSysDescr:   local.SysDescr,
			LocalIfName:     localIf,
			LocalIfIndex:    localIdx,
			RemoteDevice:    remote.Name,
			RemoteIfName:    remoteIf,
			RemoteMgmtAddr:  remote.IP,
			NeighborSources: sources,
			Timestamp:       now,
		})
	}

	for _, link := range demoLinks {
		a, okA := devByName[link.A]
		b, okB := devByName[link.B]
		if !okA || !okB {
			continue
		}
		// One observation per endpoint -- the graph store collapses them
		// into a single edge, with Sources merged.
		emit(a, b, link.AIf, link.BIf, link.AIdx, link.Sources)
		emit(b, a, link.BIf, link.AIf, link.BIdx, link.Sources)
	}
}

// runDemo seeds the graph once, then refreshes it on a timer so the
// viewer's auto-refresh shows a "live" updated_at timestamp. The
// function returns when ctx is cancelled.
func runDemo(ctx context.Context, g *Graph, interval time.Duration, now func() time.Time) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if now == nil {
		now = time.Now
	}
	seedDemo(g, now())
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			seedDemo(g, now())
		}
	}
}
