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
//   lldpRemManAddrEntry .1.0.8802.1.1.2.1.4.2.1   table indexed by
//                                                 (timeMark, locPortNum, remIdx,
//                                                  addrSubtype, addrLen, addrBytes...);
//                                                 walk column .3 (ifSubtype) to
//                                                 recover every row.
//
// CISCO-CDP-MIB:
//   cdpCacheEntry       .1.3.6.1.4.1.9.9.23.1.2.1.1   indexed by (ifIndex, cdpCacheDeviceIndex)
//                                                     (columns 3 addrType, 4 address, 5 version,
//                                                      6 deviceId, 7 devicePort, 8 platform,
//                                                      9 capabilities)
const (
	oidLLDPLocPortTable      = "1.0.8802.1.1.2.1.3.7.1"
	oidLLDPRemTable          = "1.0.8802.1.1.2.1.4.1.1"
	oidLLDPRemManAddrTable   = "1.0.8802.1.1.2.1.4.2.1"
	oidLLDPRemManAddrIfStype = "1.0.8802.1.1.2.1.4.2.1.3" // any column works; pick one guaranteed to exist

	oidCDPCacheTable = "1.3.6.1.4.1.9.9.23.1.2.1.1"

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

// LLDP lldpLocPortIdSubtype values from LLDP-MIB.
const (
	lldpSubtypeInterfaceAlias = 1
	lldpSubtypePortComponent  = 2
	lldpSubtypeMacAddress     = 3
	lldpSubtypeNetworkAddress = 4
	lldpSubtypeInterfaceName  = 5
	lldpSubtypeAgentCircuitID = 6
	lldpSubtypeLocal          = 7
)

// lldpLocPort captures the resolver inputs for one entry of lldpLocPortEntry.
type lldpLocPort struct {
	subtype int64
	id      []byte
	desc    string
}

// pollLLDPNeighbors walks lldpLocPortTable and lldpRemTable and returns
// discovered neighbors. The ifaces map is used to map lldpLocalPortNum
// (an opaque identifier) to a concrete local ifIndex / ifName.
func pollLLDPNeighbors(ctx context.Context, conf *kt.SnmpDeviceConfig, server *gosnmp.GoSNMP, ifaces map[string]*kt.InterfaceData, log logger.ContextL) ([]kt.TopologyNeighbor, error) {
	// lldpLocPortEntry first — we need it to resolve local port numbers.
	// Failure here isn't fatal: we can still report neighbors without a
	// local ifIndex / ifName by leaving those fields zero.
	var locPorts map[string]*lldpLocPort
	locPDUs, err := snmp_util.WalkOID(ctx, conf, oidLLDPLocPortTable, server, log, "LLDPLocPort")
	if err != nil {
		log.Infof("LLDP local-port walk failed, proceeding without local-port resolution: %v", err)
	} else {
		locPorts = parseLLDPLocPorts(locPDUs, oidLLDPLocPortTable)
	}

	remPDUs, err := snmp_util.WalkOID(ctx, conf, oidLLDPRemTable, server, log, "LLDPRem")
	if err != nil {
		return nil, err
	}

	// Management addresses live in a separate table whose rows are keyed by
	// (timeMark, locPort, remIdx, addrSubtype, addr...). Walking it is
	// best-effort: if the device doesn't support it, we just skip the
	// enrichment.
	var manAddrs map[string]string
	manAddrPDUs, err := snmp_util.WalkOID(ctx, conf, oidLLDPRemManAddrIfStype, server, log, "LLDPRemManAddr")
	if err != nil {
		log.Infof("LLDP management-address walk failed, skipping enrichment: %v", err)
	} else {
		manAddrs = parseLLDPManAddrs(manAddrPDUs, oidLLDPRemManAddrIfStype)
	}

	return parseLLDPNeighbors(remPDUs, oidLLDPRemTable, locPorts, ifaces, manAddrs), nil
}

// parseLLDPLocPorts builds a map keyed by the LLDP local-port-number (as a
// decimal string) from a walk of lldpLocPortEntry.
//
// lldpLocPortEntry OID layout: .<base>.<col>.<lldpLocalPortNum>
// Columns of interest: .2 subtype, .3 id, .4 desc.
func parseLLDPLocPorts(pdus []gosnmp.SnmpPDU, base string) map[string]*lldpLocPort {
	out := map[string]*lldpLocPort{}
	get := func(k string) *lldpLocPort {
		if p, ok := out[k]; ok {
			return p
		}
		p := &lldpLocPort{}
		out[k] = p
		return p
	}
	for _, pdu := range pdus {
		parts := trimOIDSuffix(pdu.Name, base)
		if len(parts) < 2 {
			continue
		}
		col := parts[0]
		portNum := parts[len(parts)-1]
		p := get(portNum)
		switch col {
		case "2":
			p.subtype = gosnmp.ToBigInt(pdu.Value).Int64()
		case "3":
			if b, ok := pdu.Value.([]byte); ok && pdu.Type == gosnmp.OctetString {
				p.id = b
			}
		case "4":
			if s, ok := snmp_util.ReadOctetString(pdu, true); ok {
				p.desc = s
			}
		}
	}
	return out
}

// parseLLDPManAddrs walks lldpRemManAddrEntry rows (e.g. from a walk of
// column .3 lldpRemManAddrIfSubtype) and returns a map keyed by
// "<timeMark>.<locPort>.<remIdx>" whose value is the first management
// address found for that remote, formatted as an IPv4 dotted quad, IPv6
// colon-hex, or generic hex fallback.
//
// A walked row looks like
//
//	.<base>.<timeMark>.<locPort>.<remIdx>.<addrSubtype>.<addrLen>.<b1>.<b2>...
//
// where addrSubtype is an IANA AddressFamilyNumbers value (1=ipv4, 2=ipv6)
// and addrLen is the number of bytes that follow.
func parseLLDPManAddrs(pdus []gosnmp.SnmpPDU, base string) map[string]string {
	out := map[string]string{}
	for _, pdu := range pdus {
		parts := trimOIDSuffix(pdu.Name, base)
		// Need at least: timeMark, locPort, remIdx, addrSubtype, addrLen,
		// and one address byte.
		if len(parts) < 6 {
			continue
		}
		timeMark := parts[0]
		locPort := parts[1]
		remIdx := parts[2]
		subtype, err := strconv.Atoi(parts[3])
		if err != nil {
			continue
		}
		addrLen, err := strconv.Atoi(parts[4])
		if err != nil || addrLen < 1 || len(parts) < 5+addrLen {
			continue
		}
		addrBytes := make([]byte, 0, addrLen)
		for i := 0; i < addrLen; i++ {
			v, err := strconv.Atoi(parts[5+i])
			if err != nil || v < 0 || v > 255 {
				addrBytes = nil
				break
			}
			addrBytes = append(addrBytes, byte(v))
		}
		if addrBytes == nil {
			continue
		}

		key := timeMark + "." + locPort + "." + remIdx
		if _, seen := out[key]; seen {
			// Keep the first one we observed so output is deterministic.
			continue
		}
		out[key] = formatLLDPManAddr(subtype, addrBytes)
	}
	return out
}

// formatLLDPManAddr renders the raw address bytes according to the IANA
// address-family subtype. Unknown families fall through to hex so nothing
// is lost.
func formatLLDPManAddr(subtype int, b []byte) string {
	switch subtype {
	case 1: // ipv4
		if len(b) == 4 {
			return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
		}
	case 2: // ipv6
		if len(b) == 16 {
			parts := make([]string, 8)
			for i := 0; i < 8; i++ {
				parts[i] = fmt.Sprintf("%x", (uint16(b[2*i])<<8)|uint16(b[2*i+1]))
			}
			return strings.Join(parts, ":")
		}
	}
	return hex.EncodeToString(b)
}

// parseLLDPNeighbors builds TopologyNeighbor records from a walk of
// lldpRemEntry. It's split out from pollLLDPNeighbors for testability.
//
// lldpRemEntry OID layout:
//
//	.<base>.<col>.<lldpRemTimeMark>.<lldpLocalPortNum>.<lldpRemIndex>
//
// Columns of interest:
//
//	.4  lldpRemChassisIdSubtype (INTEGER)
//	.5  lldpRemChassisId        (OCTET STRING, subtype-dependent encoding)
//	.6  lldpRemPortIdSubtype    (INTEGER)
//	.7  lldpRemPortId           (OCTET STRING, subtype-dependent encoding)
//	.8  lldpRemPortDesc         (OCTET STRING)
//	.9  lldpRemSysName          (OCTET STRING)
//	.10 lldpRemSysDesc          (OCTET STRING)
//	.12 lldpRemSysCapEnabled    (OCTET STRING, bit-packed)
//
// manAddrs (optional) is keyed by "timeMark.locPort.remIdx" and supplies a
// management address previously extracted from lldpRemManAddrEntry; pass nil
// to skip that enrichment.
func parseLLDPNeighbors(pdus []gosnmp.SnmpPDU, base string, locPorts map[string]*lldpLocPort, ifaces map[string]*kt.InterfaceData, manAddrs map[string]string) []kt.TopologyNeighbor {
	type scratch struct {
		chassisSubtype int64
		chassisRaw     []byte
		portSubtype    int64
		portRaw        []byte
		n              *kt.TopologyNeighbor
	}

	byKey := map[string]*scratch{}
	order := []string{}

	get := func(timeMark, locPort, remIdx string) *scratch {
		key := timeMark + "." + locPort + "." + remIdx
		if s, ok := byKey[key]; ok {
			return s
		}
		s := &scratch{n: &kt.TopologyNeighbor{Source: kt.NeighborSourceLLDP, LocalIfName: locPort}}
		// Record the raw local port number so callers see something even
		// if resolution fails; it gets overwritten with the real ifName
		// below.
		byKey[key] = s
		order = append(order, key)
		return s
	}

	for _, pdu := range pdus {
		parts := trimOIDSuffix(pdu.Name, base)
		if len(parts) < 4 {
			continue
		}
		col := parts[0]
		// Last three are the (timeMark, locPort, remIdx) tuple.
		timeMark := parts[len(parts)-3]
		locPort := parts[len(parts)-2]
		remIdx := parts[len(parts)-1]
		s := get(timeMark, locPort, remIdx)

		switch col {
		case "4":
			s.chassisSubtype = gosnmp.ToBigInt(pdu.Value).Int64()
		case "5":
			if b, ok := pdu.Value.([]byte); ok && pdu.Type == gosnmp.OctetString {
				s.chassisRaw = b
			}
		case "6":
			s.portSubtype = gosnmp.ToBigInt(pdu.Value).Int64()
		case "7":
			if b, ok := pdu.Value.([]byte); ok && pdu.Type == gosnmp.OctetString {
				s.portRaw = b
			}
		case "8":
			if v, ok := snmp_util.ReadOctetString(pdu, true); ok {
				s.n.RemotePortDesc = v
			}
		case "9":
			if v, ok := snmp_util.ReadOctetString(pdu, true); ok {
				s.n.RemoteSysName = v
			}
		case "10":
			if v, ok := snmp_util.ReadOctetString(pdu, true); ok {
				s.n.RemoteSysDesc = v
			}
		case "12":
			if v, ok := decodeLLDPCapabilities(pdu); ok {
				s.n.RemoteCapabilities = v
			}
		}
	}

	out := make([]kt.TopologyNeighbor, 0, len(order))
	for _, k := range order {
		s := byKey[k]
		// Finalize the chassis id / port id strings now that we have their subtypes.
		s.n.RemoteChassisID = decodeLLDPChassisID(s.chassisSubtype, s.chassisRaw)
		if pid := decodeLLDPPortID(s.portSubtype, s.portRaw); pid != "" {
			s.n.RemotePortID = pid
		}
		// Resolve local ifIndex / ifName.
		// key is "<timeMark>.<locPort>.<remIdx>"; pull the middle element.
		keyParts := strings.Split(k, ".")
		if len(keyParts) == 3 {
			resolveLLDPLocal(s.n, keyParts[1], locPorts, ifaces)
		}
		if addr, ok := manAddrs[k]; ok && addr != "" {
			s.n.RemoteMgmtAddr = addr
		}
		out = append(out, *s.n)
	}
	return out
}

// resolveLLDPLocal fills LocalIfIndex and LocalIfName on the neighbor by
// looking at the lldpLocPort entry for the given local port number and
// matching it against the interface metadata collected earlier.
func resolveLLDPLocal(n *kt.TopologyNeighbor, locPort string, locPorts map[string]*lldpLocPort, ifaces map[string]*kt.InterfaceData) {
	info, ok := locPorts[locPort]
	if !ok || info == nil {
		// No local-port info. Leave LocalIfName as the raw LLDP port
		// number so downstream consumers at least have something.
		return
	}
	if info.desc != "" {
		n.LocalIfName = info.desc
	}

	if ifaces == nil || len(ifaces) == 0 {
		return
	}

	idStr := string(info.id)
	switch info.subtype {
	case lldpSubtypeInterfaceAlias:
		for idx, d := range ifaces {
			if d.Alias != "" && d.Alias == idStr {
				setLocal(n, idx, d)
				return
			}
		}
	case lldpSubtypeInterfaceName:
		for idx, d := range ifaces {
			if d.Description != "" && d.Description == idStr {
				setLocal(n, idx, d)
				return
			}
		}
	case lldpSubtypeMacAddress:
		mac := formatMAC(info.id)
		for idx, d := range ifaces {
			if d.ExtraInfo == nil {
				continue
			}
			if phys, ok := d.ExtraInfo[SNMP_ifPhysAddress]; ok && phys != "" && strings.EqualFold(phys, mac) {
				setLocal(n, idx, d)
				return
			}
		}
	case lldpSubtypeLocal, lldpSubtypePortComponent, lldpSubtypeAgentCircuitID:
		// Fall through to the description-based fallback below.
	}

	// Generic fallback: match lldpLocPortDesc against ifDescr.
	if info.desc != "" {
		for idx, d := range ifaces {
			if d.Description == info.desc {
				setLocal(n, idx, d)
				return
			}
		}
	}
}

func setLocal(n *kt.TopologyNeighbor, idxStr string, d *kt.InterfaceData) {
	if i, err := strconv.ParseInt(idxStr, 10, 64); err == nil {
		n.LocalIfIndex = i
	}
	if d.Description != "" {
		n.LocalIfName = d.Description
	}
}

// decodeLLDPChassisID renders a chassis id according to its subtype.
// Subtype 4 (macAddress) is the most common on real gear; everything else
// either comes through as a string or as hex when the bytes don't form a
// printable string.
func decodeLLDPChassisID(subtype int64, b []byte) string {
	if len(b) == 0 {
		return ""
	}
	switch subtype {
	case 4: // macAddress
		return formatMAC(b)
	case 5: // networkAddress (first byte = address family per IANA)
		if len(b) == 5 && b[0] == 1 { // IPv4
			return fmt.Sprintf("%d.%d.%d.%d", b[1], b[2], b[3], b[4])
		}
		return hex.EncodeToString(b)
	default:
		if s, ok := printableString(b); ok {
			return s
		}
		return hex.EncodeToString(b)
	}
}

// decodeLLDPPortID renders a port id according to its subtype.
func decodeLLDPPortID(subtype int64, b []byte) string {
	if len(b) == 0 {
		return ""
	}
	switch subtype {
	case 3: // macAddress
		return formatMAC(b)
	default:
		if s, ok := printableString(b); ok {
			return s
		}
		return hex.EncodeToString(b)
	}
}

func formatMAC(b []byte) string {
	if len(b) != 6 {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
}

// printableString returns the bytes as a UTF-8 string if every byte is a
// printable ASCII character or common whitespace; otherwise ok=false.
func printableString(b []byte) (string, bool) {
	for _, c := range b {
		if c < 0x20 || c > 0x7e {
			return "", false
		}
	}
	return string(b), true
}

// trimOIDSuffix strips a leading dot and the base OID from a PDU name,
// returning the remaining index components split on ".". Returns nil if
// the name doesn't belong to the table.
func trimOIDSuffix(name, base string) []string {
	trimmed := strings.TrimPrefix(name, ".")
	if !strings.HasPrefix(trimmed, base) {
		return nil
	}
	suffix := strings.TrimPrefix(trimmed[len(base):], ".")
	if suffix == "" {
		return nil
	}
	return strings.Split(suffix, ".")
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
			if s, ok := decodeCDPCapabilities(pdu); ok {
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

// lldpCapNames maps each defined bit in lldpRemSysCapEnabled to its
// IEEE 802.1AB short name. The zero-indexed bit numbers follow the SMIv2
// BITS convention where bit 0 is the most-significant bit of the first
// octet. Order matters for decoded output: we walk the slice in bit-index
// order so the rendered list is stable.
var lldpCapNames = []string{
	"other",
	"repeater",
	"bridge",
	"wlan-ap",
	"router",
	"telephone",
	"docsis",
	"station-only",
}

// decodeLLDPCapabilities renders lldpRemSysCapEnabled (a BITS octet string)
// as a comma-joined list of capability names, e.g. "bridge,router". When
// none of the known bits are set the raw bytes are surfaced as a hex token
// so callers don't lose vendor extensions.
func decodeLLDPCapabilities(pdu gosnmp.SnmpPDU) (string, bool) {
	if pdu.Type != gosnmp.OctetString {
		return "", false
	}
	b, ok := pdu.Value.([]byte)
	if !ok || len(b) == 0 {
		return "", false
	}
	names := make([]string, 0, len(lldpCapNames))
	for bit, name := range lldpCapNames {
		byteIdx := bit / 8
		if byteIdx >= len(b) {
			break
		}
		mask := byte(0x80 >> (bit % 8))
		if b[byteIdx]&mask != 0 {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "0x" + hex.EncodeToString(b), true
	}
	return strings.Join(names, ","), true
}

// cdpCapNames maps each defined bit in cdpCacheCapabilities to its CISCO-CDP
// short name. Per the MIB, bits are numbered LSB-first in the LAST byte of
// a 4-octet value, so bit 0 = 0x01, bit 1 = 0x02, … bit 7 = 0x80, with
// bit 8 moving into the next byte down. We interpret the octet string as
// a big-endian integer to sidestep the ordering.
var cdpCapNames = []string{
	"router",
	"trans-bridge",
	"source-route-bridge",
	"switch",
	"host",
	"igmp-conditional",
	"repeater",
	"phone",
	"remotely-managed",
	"cvta",
}

// decodeCDPCapabilities renders cdpCacheCapabilities as a comma-joined list
// of capability names, falling back to hex when no known bits are set.
func decodeCDPCapabilities(pdu gosnmp.SnmpPDU) (string, bool) {
	if pdu.Type != gosnmp.OctetString {
		return "", false
	}
	b, ok := pdu.Value.([]byte)
	if !ok || len(b) == 0 {
		return "", false
	}
	// Interpret the first up-to-8 bytes as a big-endian unsigned integer.
	var v uint64
	for _, byt := range b {
		v = v<<8 | uint64(byt)
	}
	names := make([]string, 0, len(cdpCapNames))
	for bit, name := range cdpCapNames {
		if v&(1<<uint(bit)) != 0 {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "0x" + hex.EncodeToString(b), true
	}
	return strings.Join(names, ","), true
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
