package metadata

import (
	"context"

	"github.com/gosnmp/gosnmp"

	"github.com/kentik/ktranslate/pkg/eggs/logger"
	snmp_util "github.com/kentik/ktranslate/pkg/inputs/snmp/util"
	"github.com/kentik/ktranslate/pkg/kt"
)

// Topology OIDs.
//
// LLDP-MIB (IEEE 802.1AB):
//   lldpLocPortEntry    .1.0.8802.1.1.2.1.3.7.1   (columns 2 subtype, 3 id, 4 desc)
//   lldpRemEntry        .1.0.8802.1.1.2.1.4.1.1   (columns 4 chassisSubtype, 5 chassisId,
//                                                  6 portSubtype, 7 portId, 8 portDesc,
//                                                  9 sysName, 10 sysDesc, 12 capEnabled)
//   lldpRemManAddrEntry .1.0.8802.1.1.2.1.4.2.1   (column 3 ifSubtype, 5 ifId — not walked here)
//
// CISCO-CDP-MIB:
//   cdpCacheEntry       .1.3.6.1.4.1.9.9.23.1.2.1.1   indexed by (ifIndex, cdpCacheDeviceIndex)
//                                                     (columns 3 addrType, 4 address, 5 version,
//                                                      6 deviceId, 7 devicePort, 8 platform,
//                                                      9 capabilities)
const (
	oidLLDPLocPortTable = "1.0.8802.1.1.2.1.3.7.1"
	oidLLDPRemTable     = "1.0.8802.1.1.2.1.4.1.1"
	oidCDPCacheTable    = "1.3.6.1.4.1.9.9.23.1.2.1.1"

	neighborProtocolLLDP = "lldp"
	neighborProtocolCDP  = "cdp"
)

// PollTopology walks LLDP and/or CDP tables on the device and returns the
// list of discovered neighbors. The ifaces map (ifIndex-string -> InterfaceData)
// is used to resolve LLDP local-port identifiers to concrete ifIndex values
// and interface names; it may be nil or empty, in which case LLDP neighbors
// fall back to reporting whatever local identifier the device exposed.
//
// Returns (nil, nil) when neighbor discovery is disabled for the device.
func PollTopology(ctx context.Context, conf *kt.SnmpDeviceConfig, server *gosnmp.GoSNMP, ifaces map[string]*kt.InterfaceData, log logger.ContextL) ([]kt.TopologyNeighbor, error) {
	if conf == nil || !conf.DiscoverNeighbors {
		return nil, nil
	}

	protos := selectedNeighborProtocols(conf)
	var neighbors []kt.TopologyNeighbor

	if protos[neighborProtocolLLDP] {
		lldp, err := pollLLDPNeighbors(ctx, conf, server, ifaces, log)
		if err != nil {
			log.Warnf("LLDP neighbor walk failed: %v", err)
		} else {
			neighbors = append(neighbors, lldp...)
		}
	}

	if protos[neighborProtocolCDP] {
		cdp, err := pollCDPNeighbors(ctx, conf, server, ifaces, log)
		if err != nil {
			log.Warnf("CDP neighbor walk failed: %v", err)
		} else {
			neighbors = append(neighbors, cdp...)
		}
	}

	return mergeNeighbors(neighbors), nil
}

// selectedNeighborProtocols returns a set of enabled protocols. An empty
// NeighborProtocols list means "both LLDP and CDP".
func selectedNeighborProtocols(conf *kt.SnmpDeviceConfig) map[string]bool {
	out := map[string]bool{}
	if len(conf.NeighborProtocols) == 0 {
		out[neighborProtocolLLDP] = true
		out[neighborProtocolCDP] = true
		return out
	}
	for _, p := range conf.NeighborProtocols {
		switch p {
		case neighborProtocolLLDP, neighborProtocolCDP:
			out[p] = true
		}
	}
	return out
}

// pollLLDPNeighbors is a stub until Stage 3. It intentionally performs no
// walks so the Stage 1 wiring can land in isolation.
func pollLLDPNeighbors(ctx context.Context, conf *kt.SnmpDeviceConfig, server *gosnmp.GoSNMP, ifaces map[string]*kt.InterfaceData, log logger.ContextL) ([]kt.TopologyNeighbor, error) {
	_ = ctx
	_ = conf
	_ = server
	_ = ifaces
	_ = log
	_ = snmp_util.WalkOID // keep the import hooked up for Stage 3
	return nil, nil
}

// pollCDPNeighbors is a stub until Stage 2.
func pollCDPNeighbors(ctx context.Context, conf *kt.SnmpDeviceConfig, server *gosnmp.GoSNMP, ifaces map[string]*kt.InterfaceData, log logger.ContextL) ([]kt.TopologyNeighbor, error) {
	_ = ctx
	_ = conf
	_ = server
	_ = ifaces
	_ = log
	return nil, nil
}

// mergeNeighbors collapses adjacent records that refer to the same link but
// were reported by different protocols (LLDP+CDP). Two neighbors match when
// they share local ifIndex (or local ifName when ifIndex is unknown) plus
// remote sysName (when known) or remote chassis id / remote port id.
func mergeNeighbors(in []kt.TopologyNeighbor) []kt.TopologyNeighbor {
	if len(in) < 2 {
		return in
	}
	out := make([]kt.TopologyNeighbor, 0, len(in))
	used := make([]bool, len(in))
	for i := range in {
		if used[i] {
			continue
		}
		cur := in[i]
		for j := i + 1; j < len(in); j++ {
			if used[j] {
				continue
			}
			if neighborsMatch(cur, in[j]) {
				cur = combineNeighbors(cur, in[j])
				used[j] = true
			}
		}
		out = append(out, cur)
	}
	return out
}

func neighborsMatch(a, b kt.TopologyNeighbor) bool {
	if a.Source == b.Source {
		return false // same-protocol duplicates are kept as-is
	}
	// Require the same local port.
	if a.LocalIfIndex != 0 && b.LocalIfIndex != 0 {
		if a.LocalIfIndex != b.LocalIfIndex {
			return false
		}
	} else if a.LocalIfName != "" && b.LocalIfName != "" {
		if a.LocalIfName != b.LocalIfName {
			return false
		}
	} else {
		return false
	}
	// And the same remote end (at least one positive signal).
	switch {
	case a.RemoteSysName != "" && b.RemoteSysName != "":
		return a.RemoteSysName == b.RemoteSysName
	case a.RemoteChassisID != "" && b.RemoteChassisID != "":
		return a.RemoteChassisID == b.RemoteChassisID
	case a.RemotePortID != "" && b.RemotePortID != "":
		return a.RemotePortID == b.RemotePortID
	}
	return false
}

func combineNeighbors(a, b kt.TopologyNeighbor) kt.TopologyNeighbor {
	merged := a
	merged.Source = kt.NeighborSourceBoth
	if merged.LocalIfIndex == 0 {
		merged.LocalIfIndex = b.LocalIfIndex
	}
	merged.LocalIfName = pickNonEmpty(merged.LocalIfName, b.LocalIfName)
	merged.RemoteChassisID = pickNonEmpty(merged.RemoteChassisID, b.RemoteChassisID)
	merged.RemoteSysName = pickNonEmpty(merged.RemoteSysName, b.RemoteSysName)
	merged.RemoteSysDesc = pickNonEmpty(merged.RemoteSysDesc, b.RemoteSysDesc)
	merged.RemotePortID = pickNonEmpty(merged.RemotePortID, b.RemotePortID)
	merged.RemotePortDesc = pickNonEmpty(merged.RemotePortDesc, b.RemotePortDesc)
	merged.RemoteMgmtAddr = pickNonEmpty(merged.RemoteMgmtAddr, b.RemoteMgmtAddr)
	merged.RemoteCapabilities = pickNonEmpty(merged.RemoteCapabilities, b.RemoteCapabilities)
	merged.RemotePlatform = pickNonEmpty(merged.RemotePlatform, b.RemotePlatform)
	return merged
}

func pickNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
