package metadata

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

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
	return nil, nil
}

// pollCDPNeighbors walks the cdpCacheTable and returns any discovered
// neighbors. The ifaces map is used purely to enrich the neighbor records
// with a local interface name (ifDescr); resolving the local ifIndex is
// free for CDP because it's part of the cdpCacheEntry index.
func pollCDPNeighbors(ctx context.Context, conf *kt.SnmpDeviceConfig, server *gosnmp.GoSNMP, ifaces map[string]*kt.InterfaceData, log logger.ContextL) ([]kt.TopologyNeighbor, error) {
	pdus, err := snmp_util.WalkOID(ctx, conf, oidCDPCacheTable, server, log, "CDPCache")
	if err != nil {
		return nil, err
	}
	return parseCDPNeighbors(pdus, oidCDPCacheTable, ifaces), nil
}

// parseCDPNeighbors builds TopologyNeighbor records from a walk of
// cdpCacheEntry. It's split out from pollCDPNeighbors so it can be exercised
// by unit tests against synthetic PDUs.
//
// The OID of each walked varbind is expected to have the form
// <base>.<col>.<ifIndex>.<cdpCacheDeviceIndex>. Columns of interest:
//
//	.3  cdpCacheAddressType (INTEGER, 1=ip)
//	.4  cdpCacheAddress     (OCTET STRING, raw address bytes)
//	.5  cdpCacheVersion     (OCTET STRING)
//	.6  cdpCacheDeviceId    (OCTET STRING, remote device name / chassis id)
//	.7  cdpCacheDevicePort  (OCTET STRING, remote port name)
//	.8  cdpCachePlatform    (OCTET STRING, remote platform/model)
//	.9  cdpCacheCapabilities(OCTET STRING, bit-packed)
func parseCDPNeighbors(pdus []gosnmp.SnmpPDU, base string, ifaces map[string]*kt.InterfaceData) []kt.TopologyNeighbor {
	// Keyed by "<ifIndex>.<devIdx>" so we can accrete columns onto a single
	// record as we iterate the walk.
	byKey := map[string]*kt.TopologyNeighbor{}
	order := []string{} // preserve insertion order for deterministic output

	getOrCreate := func(key, ifIdx string) *kt.TopologyNeighbor {
		if n, ok := byKey[key]; ok {
			return n
		}
		n := &kt.TopologyNeighbor{Source: kt.NeighborSourceCDP}
		if idx64, err := strconv.ParseInt(ifIdx, 10, 64); err == nil {
			n.LocalIfIndex = idx64
		}
		if ifaces != nil {
			if id, ok := ifaces[ifIdx]; ok {
				n.LocalIfName = id.Description
			}
		}
		byKey[key] = n
		order = append(order, key)
		return n
	}

	for _, pdu := range pdus {
		col, ifIdx, devIdx, ok := splitCDPIndex(pdu.Name, base)
		if !ok {
			continue
		}
		key := ifIdx + "." + devIdx
		n := getOrCreate(key, ifIdx)

		switch col {
		case "4": // cdpCacheAddress
			if s, ok := decodeCDPAddress(pdu); ok {
				n.RemoteMgmtAddr = s
			}
		case "6": // cdpCacheDeviceId — remote chassis / sysname
			if s, ok := snmp_util.ReadOctetString(pdu, true); ok {
				n.RemoteSysName = s
				// CDP folds chassis id and sysname together; store the
				// raw value as chassis id too so joins that key on chassis
				// id still work.
				n.RemoteChassisID = s
			}
		case "7": // cdpCacheDevicePort
			if s, ok := snmp_util.ReadOctetString(pdu, true); ok {
				n.RemotePortID = s
				n.RemotePortDesc = s
			}
		case "8": // cdpCachePlatform
			if s, ok := snmp_util.ReadOctetString(pdu, true); ok {
				n.RemotePlatform = s
			}
		case "9": // cdpCacheCapabilities
			if s, ok := decodeCapabilityBits(pdu); ok {
				n.RemoteCapabilities = s
			}
		}
	}

	out := make([]kt.TopologyNeighbor, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out
}

// splitCDPIndex pulls apart an OID walked under cdpCacheEntry. Given a
// variable.Name like ".1.3.6.1.4.1.9.9.23.1.2.1.1.6.3.1" and
// base="1.3.6.1.4.1.9.9.23.1.2.1.1", returns col="6", ifIdx="3",
// devIdx="1". Anything that doesn't look like an entry row returns
// ok=false and is skipped.
func splitCDPIndex(name, base string) (col, ifIdx, devIdx string, ok bool) {
	trimmed := strings.TrimPrefix(name, ".")
	suffix := strings.TrimPrefix(trimmed, base)
	suffix = strings.TrimPrefix(suffix, ".")
	parts := strings.Split(suffix, ".")
	if len(parts) < 3 {
		return "", "", "", false
	}
	// The index is (ifIndex, cdpCacheDeviceIndex); both are single integers
	// so we only need the last two elements.
	col = parts[0]
	ifIdx = parts[len(parts)-2]
	devIdx = parts[len(parts)-1]
	return col, ifIdx, devIdx, true
}

// decodeCDPAddress parses cdpCacheAddress (raw binary address). IPv4 is the
// common case (4 bytes); IPv6 is 16. Anything else is reported as hex so it
// at least survives the round trip.
func decodeCDPAddress(pdu gosnmp.SnmpPDU) (string, bool) {
	if pdu.Type != gosnmp.OctetString {
		return "", false
	}
	b, ok := pdu.Value.([]byte)
	if !ok {
		return "", false
	}
	switch len(b) {
	case 4:
		return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3]), true
	case 16:
		parts := make([]string, 8)
		for i := 0; i < 8; i++ {
			parts[i] = fmt.Sprintf("%x", (uint16(b[2*i])<<8)|uint16(b[2*i+1]))
		}
		return strings.Join(parts, ":"), true
	default:
		if len(b) == 0 {
			return "", false
		}
		return hex.EncodeToString(b), true
	}
}

// decodeCapabilityBits renders a bit-packed capability octet-string as a
// lowercase hex token. Callers that want semantic decoding can split the
// result back out; for now we just surface the raw bits so nothing is lost.
func decodeCapabilityBits(pdu gosnmp.SnmpPDU) (string, bool) {
	if pdu.Type != gosnmp.OctetString {
		return "", false
	}
	b, ok := pdu.Value.([]byte)
	if !ok || len(b) == 0 {
		return "", false
	}
	return "0x" + hex.EncodeToString(b), true
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
