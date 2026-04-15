package main

import (
	"sort"
	"sync"
	"time"
)

// Graph is an in-memory, thread-safe model of the network topology built up
// from KSnmpTopology records. Devices hold a map of their interfaces; Links
// are canonicalised so that reciprocal views (A sees B, B sees A) collapse
// into a single edge.
type Graph struct {
	mu      sync.RWMutex
	devices map[string]*Device // key: device name
	links   map[string]*Link   // key: canonical link id
	ttl     time.Duration
	now     func() time.Time // injectable for tests
}

// Device is a node in the graph. Interfaces are modelled as child nodes so
// that edges can connect specific ports rather than whole devices.
type Device struct {
	Name       string                `json:"name"`
	IP         string                `json:"ip,omitempty"`
	SysDescr   string                `json:"sys_descr,omitempty"`
	Interfaces map[string]*Interface `json:"-"` // serialised separately
	LastSeen   time.Time             `json:"last_seen"`
}

// Interface is a named port on a Device.
type Interface struct {
	Name     string    `json:"name"`
	IfIndex  int64     `json:"if_index,omitempty"`
	LastSeen time.Time `json:"last_seen"`
}

// Link is an edge between two interface endpoints, annotated with the
// protocol(s) that discovered it.
type Link struct {
	ID       string    `json:"id"`
	A        Endpoint  `json:"a"`
	B        Endpoint  `json:"b"`
	Sources  []string  `json:"sources"` // sorted, unique
	LastSeen time.Time `json:"last_seen"`
}

// Endpoint identifies one end of a link.
type Endpoint struct {
	Device    string `json:"device"`
	Interface string `json:"interface"`
}

// NewGraph returns an empty graph with the given link TTL. Links not seen
// within ttl are pruned lazily on Snapshot calls.
func NewGraph(ttl time.Duration) *Graph {
	return &Graph{
		devices: make(map[string]*Device),
		links:   make(map[string]*Link),
		ttl:     ttl,
		now:     time.Now,
	}
}

// Observation carries the denormalised fields extracted from a single
// KSnmpTopology record. The graph package deliberately doesn't depend on
// pkg/kt; ingest.go is the only thing that knows how to pull these fields
// out of a JCHF JSON payload.
type Observation struct {
	LocalDevice     string
	LocalDeviceIP   string
	LocalSysDescr   string
	LocalIfName     string
	LocalIfIndex    int64
	RemoteDevice    string
	RemoteIfName    string
	RemoteMgmtAddr  string
	NeighborSources []string // e.g. ["lldp"] or ["lldp","cdp"]
	Timestamp       time.Time
}

// Apply merges a single observation into the graph. Observations are
// idempotent: seeing the same link twice just refreshes LastSeen.
func (g *Graph) Apply(obs Observation) {
	if obs.LocalDevice == "" || obs.LocalIfName == "" {
		return
	}
	if obs.RemoteDevice == "" {
		// A remote without any identifier isn't worth a node on its own;
		// drop the observation rather than building an anonymous island.
		return
	}
	ts := obs.Timestamp
	if ts.IsZero() {
		ts = g.now()
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.touchDevice(obs.LocalDevice, obs.LocalDeviceIP, obs.LocalSysDescr, ts)
	g.touchInterface(obs.LocalDevice, obs.LocalIfName, obs.LocalIfIndex, ts)

	// The remote side is seen through a neighbor record, so we know its
	// name but not its IP (unless the advertised management address is
	// set, which we use).
	g.touchDevice(obs.RemoteDevice, obs.RemoteMgmtAddr, "", ts)
	if obs.RemoteIfName != "" {
		g.touchInterface(obs.RemoteDevice, obs.RemoteIfName, 0, ts)
	}

	a := Endpoint{Device: obs.LocalDevice, Interface: obs.LocalIfName}
	b := Endpoint{Device: obs.RemoteDevice, Interface: obs.RemoteIfName}
	id, a, b := canonicalLinkID(a, b)

	link, ok := g.links[id]
	if !ok {
		link = &Link{ID: id, A: a, B: b}
		g.links[id] = link
	}
	link.LastSeen = ts
	link.Sources = mergeSources(link.Sources, obs.NeighborSources)
}

// touchDevice inserts or refreshes a device node. Call with g.mu held.
func (g *Graph) touchDevice(name, ip, sysDescr string, ts time.Time) {
	d, ok := g.devices[name]
	if !ok {
		d = &Device{
			Name:       name,
			Interfaces: make(map[string]*Interface),
		}
		g.devices[name] = d
	}
	if ip != "" {
		d.IP = ip
	}
	if sysDescr != "" {
		d.SysDescr = sysDescr
	}
	d.LastSeen = ts
}

// touchInterface inserts or refreshes an interface node on a device. Call
// with g.mu held. The device must already exist (touchDevice guarantees
// this when both are called in order).
func (g *Graph) touchInterface(device, ifName string, ifIndex int64, ts time.Time) {
	d, ok := g.devices[device]
	if !ok {
		return
	}
	i, ok := d.Interfaces[ifName]
	if !ok {
		i = &Interface{Name: ifName}
		d.Interfaces[ifName] = i
	}
	if ifIndex != 0 {
		i.IfIndex = ifIndex
	}
	i.LastSeen = ts
}

// canonicalLinkID returns a stable identifier for a link regardless of
// which side was reporting. The endpoints are returned in the order they
// appear in the canonical id, so callers can store them in the same order.
func canonicalLinkID(a, b Endpoint) (string, Endpoint, Endpoint) {
	ka := a.Device + "|" + a.Interface
	kb := b.Device + "|" + b.Interface
	if ka <= kb {
		return ka + "<->" + kb, a, b
	}
	return kb + "<->" + ka, b, a
}

// mergeSources returns the union of two source slices as a sorted,
// deduped slice. Nil inputs are treated as empty.
func mergeSources(cur, add []string) []string {
	set := make(map[string]struct{}, len(cur)+len(add))
	for _, s := range cur {
		if s != "" {
			set[s] = struct{}{}
		}
	}
	for _, s := range add {
		if s != "" {
			set[s] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Snapshot is the wire format handed to the browser. It's a plain
// nodes+edges document that the embedded renderer understands.
type Snapshot struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Nodes       []SnapshotNode  `json:"nodes"`
	Edges       []SnapshotEdge  `json:"edges"`
	Counts      map[string]int  `json:"counts"`
}

// SnapshotNode is either a device or an interface. Interface nodes have
// Parent set to the owning device id so the renderer can group them.
type SnapshotNode struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // "device" or "interface"
	Parent   string `json:"parent,omitempty"`
	SysDescr string `json:"sys_descr,omitempty"`
	IP       string `json:"ip,omitempty"`
	IfIndex  int64  `json:"if_index,omitempty"`
}

// SnapshotEdge connects two node ids (always interface ids).
type SnapshotEdge struct {
	ID      string   `json:"id"`
	Source  string   `json:"source"` // node id of the A-side interface
	Target  string   `json:"target"` // node id of the B-side interface
	Sources []string `json:"sources"`
}

// Snapshot produces a read-only view of the current graph suitable for
// JSON serialisation. Stale links (older than g.ttl) are dropped along
// with interfaces and devices that become orphaned as a result.
func (g *Graph) Snapshot() Snapshot {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := g.now()
	g.pruneLocked(now)

	// Build the node set from whatever devices/interfaces are still
	// referenced either by a fresh link or by a fresh device record. We
	// include devices without any links too so freshly-polled-but-
	// unconnected devices still show up.
	nodes := make([]SnapshotNode, 0, len(g.devices)*2)
	deviceIDs := map[string]string{}
	ifaceIDs := map[string]string{}

	// Stable ordering keeps the renderer happy across refreshes.
	deviceNames := make([]string, 0, len(g.devices))
	for name := range g.devices {
		deviceNames = append(deviceNames, name)
	}
	sort.Strings(deviceNames)

	for _, name := range deviceNames {
		d := g.devices[name]
		did := "dev:" + name
		deviceIDs[name] = did
		nodes = append(nodes, SnapshotNode{
			ID:       did,
			Label:    name,
			Kind:     "device",
			IP:       d.IP,
			SysDescr: d.SysDescr,
		})
		ifaceNames := make([]string, 0, len(d.Interfaces))
		for n := range d.Interfaces {
			ifaceNames = append(ifaceNames, n)
		}
		sort.Strings(ifaceNames)
		for _, ifn := range ifaceNames {
			iface := d.Interfaces[ifn]
			iid := "if:" + name + "|" + ifn
			ifaceIDs[name+"|"+ifn] = iid
			nodes = append(nodes, SnapshotNode{
				ID:      iid,
				Label:   ifn,
				Kind:    "interface",
				Parent:  did,
				IfIndex: iface.IfIndex,
			})
		}
	}

	// Edges over the freshly-pruned link set.
	linkIDs := make([]string, 0, len(g.links))
	for id := range g.links {
		linkIDs = append(linkIDs, id)
	}
	sort.Strings(linkIDs)

	edges := make([]SnapshotEdge, 0, len(linkIDs))
	for _, id := range linkIDs {
		link := g.links[id]
		src, ok1 := ifaceIDs[link.A.Device+"|"+link.A.Interface]
		dst, ok2 := ifaceIDs[link.B.Device+"|"+link.B.Interface]
		if !ok1 || !ok2 {
			// An endpoint's interface was pruned. Skip; it'll come back
			// on the next fresh poll.
			continue
		}
		edges = append(edges, SnapshotEdge{
			ID:      id,
			Source:  src,
			Target:  dst,
			Sources: append([]string(nil), link.Sources...),
		})
	}

	return Snapshot{
		GeneratedAt: now,
		Nodes:       nodes,
		Edges:       edges,
		Counts: map[string]int{
			"devices": len(deviceNames),
			"links":   len(edges),
		},
	}
}

// pruneLocked drops stale links and empties out any interfaces/devices
// that no longer have fresh observations. Caller must hold g.mu.
func (g *Graph) pruneLocked(now time.Time) {
	if g.ttl <= 0 {
		return
	}
	cutoff := now.Add(-g.ttl)

	for id, link := range g.links {
		if link.LastSeen.Before(cutoff) {
			delete(g.links, id)
		}
	}
	for name, d := range g.devices {
		for ifName, iface := range d.Interfaces {
			if iface.LastSeen.Before(cutoff) {
				delete(d.Interfaces, ifName)
			}
		}
		if d.LastSeen.Before(cutoff) && len(d.Interfaces) == 0 {
			delete(g.devices, name)
		}
	}
}
