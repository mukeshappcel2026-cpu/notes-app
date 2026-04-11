package metadata

import (
	"fmt"
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/assert"

	"github.com/kentik/ktranslate/pkg/kt"
)

// cdpPDU builds a synthetic PDU for cdpCacheEntry testing.
func cdpPDU(col, ifIdx, devIdx string, t gosnmp.Asn1BER, v interface{}) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{
		Name:  "." + oidCDPCacheTable + "." + col + "." + ifIdx + "." + devIdx,
		Type:  t,
		Value: v,
	}
}

func TestSplitCDPIndex(t *testing.T) {
	col, ifIdx, devIdx, ok := splitCDPIndex("."+oidCDPCacheTable+".6.3.1", oidCDPCacheTable)
	assert.True(t, ok)
	assert.Equal(t, "6", col)
	assert.Equal(t, "3", ifIdx)
	assert.Equal(t, "1", devIdx)

	_, _, _, ok = splitCDPIndex("."+oidCDPCacheTable+".6", oidCDPCacheTable)
	assert.False(t, ok, "truncated OID should be rejected")
}

func TestDecodeCDPAddress(t *testing.T) {
	ipv4, ok := decodeCDPAddress(gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: []byte{10, 0, 0, 1}})
	assert.True(t, ok)
	assert.Equal(t, "10.0.0.1", ipv4)

	_, ok = decodeCDPAddress(gosnmp.SnmpPDU{Type: gosnmp.OctetString, Value: []byte{}})
	assert.False(t, ok)

	_, ok = decodeCDPAddress(gosnmp.SnmpPDU{Type: gosnmp.Integer, Value: 42})
	assert.False(t, ok)
}

func TestParseCDPNeighbors_TwoNeighbors(t *testing.T) {
	ifaces := map[string]*kt.InterfaceData{
		"3":  {Index: "3", Description: "GigabitEthernet1/0/3"},
		"10": {Index: "10", Description: "GigabitEthernet1/0/10"},
	}
	pdus := []gosnmp.SnmpPDU{
		// Neighbor on ifIndex=3, devIdx=1 — a remote switch.
		cdpPDU("4", "3", "1", gosnmp.OctetString, []byte{10, 0, 0, 2}),
		cdpPDU("6", "3", "1", gosnmp.OctetString, []byte("switch-b.example.net")),
		cdpPDU("7", "3", "1", gosnmp.OctetString, []byte("GigabitEthernet0/24")),
		cdpPDU("8", "3", "1", gosnmp.OctetString, []byte("cisco WS-C3750")),
		cdpPDU("9", "3", "1", gosnmp.OctetString, []byte{0x00, 0x00, 0x00, 0x28}),

		// Neighbor on ifIndex=10, devIdx=1 — a phone.
		cdpPDU("4", "10", "1", gosnmp.OctetString, []byte{10, 0, 0, 3}),
		cdpPDU("6", "10", "1", gosnmp.OctetString, []byte("SEP001122334455")),
		cdpPDU("7", "10", "1", gosnmp.OctetString, []byte("Port 1")),
		cdpPDU("8", "10", "1", gosnmp.OctetString, []byte("Cisco IP Phone 7965")),
	}

	got := parseCDPNeighbors(pdus, oidCDPCacheTable, ifaces)
	if assert.Len(t, got, 2) {
		// Order preserved by first-seen key (ifIdx.devIdx).
		n0 := got[0]
		assert.Equal(t, kt.NeighborSourceCDP, n0.Source)
		assert.EqualValues(t, 3, n0.LocalIfIndex)
		assert.Equal(t, "GigabitEthernet1/0/3", n0.LocalIfName)
		assert.Equal(t, "10.0.0.2", n0.RemoteMgmtAddr)
		assert.Equal(t, "switch-b.example.net", n0.RemoteSysName)
		assert.Equal(t, "switch-b.example.net", n0.RemoteChassisID)
		assert.Equal(t, "GigabitEthernet0/24", n0.RemotePortID)
		assert.Equal(t, "cisco WS-C3750", n0.RemotePlatform)
		assert.Equal(t, "0x00000028", n0.RemoteCapabilities)

		n1 := got[1]
		assert.EqualValues(t, 10, n1.LocalIfIndex)
		assert.Equal(t, "GigabitEthernet1/0/10", n1.LocalIfName)
		assert.Equal(t, "10.0.0.3", n1.RemoteMgmtAddr)
		assert.Equal(t, "SEP001122334455", n1.RemoteSysName)
		assert.Equal(t, "Cisco IP Phone 7965", n1.RemotePlatform)
	}
}

func TestParseCDPNeighbors_NilIfaces(t *testing.T) {
	pdus := []gosnmp.SnmpPDU{
		cdpPDU("6", "5", "1", gosnmp.OctetString, []byte("peer")),
		cdpPDU("7", "5", "1", gosnmp.OctetString, []byte("Fa0/1")),
	}
	got := parseCDPNeighbors(pdus, oidCDPCacheTable, nil)
	if assert.Len(t, got, 1) {
		assert.EqualValues(t, 5, got[0].LocalIfIndex)
		assert.Equal(t, "", got[0].LocalIfName, "no ifaces map means no local name")
		assert.Equal(t, "peer", got[0].RemoteSysName)
	}
}

// lldpLocPDU builds a synthetic lldpLocPortEntry PDU.
func lldpLocPDU(col, portNum string, t gosnmp.Asn1BER, v interface{}) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{
		Name:  "." + oidLLDPLocPortTable + "." + col + "." + portNum,
		Type:  t,
		Value: v,
	}
}

// lldpRemPDU builds a synthetic lldpRemEntry PDU.
func lldpRemPDU(col, timeMark, locPort, remIdx string, t gosnmp.Asn1BER, v interface{}) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{
		Name:  "." + oidLLDPRemTable + "." + col + "." + timeMark + "." + locPort + "." + remIdx,
		Type:  t,
		Value: v,
	}
}

func TestFormatMAC(t *testing.T) {
	assert.Equal(t, "aa:bb:cc:dd:ee:ff", formatMAC([]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}))
	// Non-6-byte input falls back to hex so nothing is silently dropped.
	assert.Equal(t, "aabb", formatMAC([]byte{0xaa, 0xbb}))
}

func TestDecodeLLDPChassisID(t *testing.T) {
	// Subtype 4 = macAddress
	assert.Equal(t, "00:11:22:33:44:55",
		decodeLLDPChassisID(4, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}))
	// Subtype 5 = networkAddress, IPv4 (family=1)
	assert.Equal(t, "10.0.0.9",
		decodeLLDPChassisID(5, []byte{0x01, 10, 0, 0, 9}))
	// Subtype 7 = local, printable string passes through
	assert.Equal(t, "switch-a",
		decodeLLDPChassisID(7, []byte("switch-a")))
	// Non-printable falls back to hex
	assert.Equal(t, "0011",
		decodeLLDPChassisID(7, []byte{0x00, 0x11}))
}

func TestParseLLDPLocPorts(t *testing.T) {
	pdus := []gosnmp.SnmpPDU{
		lldpLocPDU("2", "1", gosnmp.Integer, 5),                          // subtype=interfaceName
		lldpLocPDU("3", "1", gosnmp.OctetString, []byte("Ethernet1/1")),  // id
		lldpLocPDU("4", "1", gosnmp.OctetString, []byte("Ethernet1/1")),  // desc

		lldpLocPDU("2", "2", gosnmp.Integer, 3),                          // subtype=macAddress
		lldpLocPDU("3", "2", gosnmp.OctetString, []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}),
		lldpLocPDU("4", "2", gosnmp.OctetString, []byte("Ethernet1/2")),
	}
	out := parseLLDPLocPorts(pdus, oidLLDPLocPortTable)
	if assert.Contains(t, out, "1") {
		assert.EqualValues(t, lldpSubtypeInterfaceName, out["1"].subtype)
		assert.Equal(t, []byte("Ethernet1/1"), out["1"].id)
		assert.Equal(t, "Ethernet1/1", out["1"].desc)
	}
	if assert.Contains(t, out, "2") {
		assert.EqualValues(t, lldpSubtypeMacAddress, out["2"].subtype)
		assert.Equal(t, "Ethernet1/2", out["2"].desc)
	}
}

func TestParseLLDPNeighbors_InterfaceNameResolution(t *testing.T) {
	// Two neighbors: one on local port 1 (mapped via interfaceName), one
	// on local port 2 (mapped via macAddress).
	ifaces := map[string]*kt.InterfaceData{
		"101": {
			Index:       "101",
			Description: "Ethernet1/1",
			ExtraInfo:   map[string]string{},
		},
		"102": {
			Index:       "102",
			Description: "Ethernet1/2",
			ExtraInfo: map[string]string{
				SNMP_ifPhysAddress: "aa:bb:cc:dd:ee:ff",
			},
		},
	}
	locPorts := map[string]*lldpLocPort{
		"1": {
			subtype: lldpSubtypeInterfaceName,
			id:      []byte("Ethernet1/1"),
			desc:    "Ethernet1/1",
		},
		"2": {
			subtype: lldpSubtypeMacAddress,
			id:      []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			desc:    "Ethernet1/2",
		},
	}
	pdus := []gosnmp.SnmpPDU{
		// local port 1, timeMark 5, rem index 1 — a neighbor switch
		lldpRemPDU("4", "5", "1", "1", gosnmp.Integer, 4), // chassisSubtype = macAddress
		lldpRemPDU("5", "5", "1", "1", gosnmp.OctetString, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}),
		lldpRemPDU("6", "5", "1", "1", gosnmp.Integer, 5), // portSubtype = interfaceName
		lldpRemPDU("7", "5", "1", "1", gosnmp.OctetString, []byte("GigabitEthernet0/24")),
		lldpRemPDU("8", "5", "1", "1", gosnmp.OctetString, []byte("uplink to core")),
		lldpRemPDU("9", "5", "1", "1", gosnmp.OctetString, []byte("core-a.example.net")),
		lldpRemPDU("10", "5", "1", "1", gosnmp.OctetString, []byte("Cisco IOS core-a")),
		lldpRemPDU("12", "5", "1", "1", gosnmp.OctetString, []byte{0x00, 0x14}),

		// local port 2, timeMark 5, rem index 1 — a server
		lldpRemPDU("4", "5", "2", "1", gosnmp.Integer, 4),
		lldpRemPDU("5", "5", "2", "1", gosnmp.OctetString, []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x01}),
		lldpRemPDU("6", "5", "2", "1", gosnmp.Integer, 3), // portSubtype = macAddress
		lldpRemPDU("7", "5", "2", "1", gosnmp.OctetString, []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x02}),
		lldpRemPDU("9", "5", "2", "1", gosnmp.OctetString, []byte("server-1")),
	}

	got := parseLLDPNeighbors(pdus, oidLLDPRemTable, locPorts, ifaces, nil)
	if assert.Len(t, got, 2) {
		n0 := got[0]
		assert.Equal(t, kt.NeighborSourceLLDP, n0.Source)
		assert.EqualValues(t, 101, n0.LocalIfIndex)
		assert.Equal(t, "Ethernet1/1", n0.LocalIfName)
		assert.Equal(t, "00:11:22:33:44:55", n0.RemoteChassisID)
		assert.Equal(t, "core-a.example.net", n0.RemoteSysName)
		assert.Equal(t, "Cisco IOS core-a", n0.RemoteSysDesc)
		assert.Equal(t, "GigabitEthernet0/24", n0.RemotePortID)
		assert.Equal(t, "uplink to core", n0.RemotePortDesc)
		assert.Equal(t, "0x0014", n0.RemoteCapabilities)

		n1 := got[1]
		assert.EqualValues(t, 102, n1.LocalIfIndex)
		assert.Equal(t, "Ethernet1/2", n1.LocalIfName)
		assert.Equal(t, "deadbeef:0001", "deadbeef:0001") // sanity
		assert.Equal(t, "de:ad:be:ef:00:01", n1.RemoteChassisID)
		assert.Equal(t, "de:ad:be:ef:00:02", n1.RemotePortID, "mac port id should be formatted")
		assert.Equal(t, "server-1", n1.RemoteSysName)
	}
}

// lldpManAddrPDU builds a synthetic lldpRemManAddrEntry row PDU. The OID
// suffix after the base column is
// <timeMark>.<locPort>.<remIdx>.<addrSubtype>.<addrLen>.<b1>...<bN>.
func lldpManAddrPDU(timeMark, locPort, remIdx string, addrSubtype int, addr []byte) gosnmp.SnmpPDU {
	oid := "." + oidLLDPRemManAddrIfStype + "." + timeMark + "." + locPort + "." + remIdx + "." +
		itoa(addrSubtype) + "." + itoa(len(addr))
	for _, b := range addr {
		oid += "." + itoa(int(b))
	}
	return gosnmp.SnmpPDU{Name: oid, Type: gosnmp.Integer, Value: 1}
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }

func TestParseLLDPManAddrs_IPv4(t *testing.T) {
	pdus := []gosnmp.SnmpPDU{
		lldpManAddrPDU("5", "1", "1", 1, []byte{10, 0, 0, 9}),
		lldpManAddrPDU("5", "1", "1", 2, []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}),
		lldpManAddrPDU("5", "2", "1", 1, []byte{172, 16, 0, 5}),
	}
	got := parseLLDPManAddrs(pdus, oidLLDPRemManAddrIfStype)
	// First-seen wins, so (5,1,1) keeps the IPv4 address.
	assert.Equal(t, "10.0.0.9", got["5.1.1"])
	assert.Equal(t, "172.16.0.5", got["5.2.1"])
}

func TestParseLLDPManAddrs_IPv6(t *testing.T) {
	pdus := []gosnmp.SnmpPDU{
		lldpManAddrPDU("5", "3", "1", 2,
			[]byte{0xfe, 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x01}),
	}
	got := parseLLDPManAddrs(pdus, oidLLDPRemManAddrIfStype)
	assert.Equal(t, "fe80:0:0:0:0:0:0:1", got["5.3.1"])
}

func TestParseLLDPManAddrs_Malformed(t *testing.T) {
	// Truncated: claims addrLen=4 but only supplies 2 bytes.
	truncated := gosnmp.SnmpPDU{
		Name:  "." + oidLLDPRemManAddrIfStype + ".5.1.1.1.4.10.0",
		Type:  gosnmp.Integer,
		Value: 1,
	}
	got := parseLLDPManAddrs([]gosnmp.SnmpPDU{truncated}, oidLLDPRemManAddrIfStype)
	assert.Empty(t, got)
}

func TestParseLLDPNeighbors_AttachesManagementAddress(t *testing.T) {
	// Minimum PDUs to produce one neighbor on (timeMark=5, locPort=1, remIdx=1).
	pdus := []gosnmp.SnmpPDU{
		lldpRemPDU("4", "5", "1", "1", gosnmp.Integer, 4),
		lldpRemPDU("5", "5", "1", "1", gosnmp.OctetString, []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}),
		lldpRemPDU("9", "5", "1", "1", gosnmp.OctetString, []byte("peer")),
	}
	manAddrs := map[string]string{"5.1.1": "192.0.2.10"}
	got := parseLLDPNeighbors(pdus, oidLLDPRemTable, nil, nil, manAddrs)
	if assert.Len(t, got, 1) {
		assert.Equal(t, "192.0.2.10", got[0].RemoteMgmtAddr)
		assert.Equal(t, "peer", got[0].RemoteSysName)
	}
}

func TestParseLLDPNeighbors_DescFallback(t *testing.T) {
	// locPort subtype 7 (local) — resolve via desc match against ifDescr.
	ifaces := map[string]*kt.InterfaceData{
		"7": {Index: "7", Description: "Fa0/7", ExtraInfo: map[string]string{}},
	}
	locPorts := map[string]*lldpLocPort{
		"7": {subtype: lldpSubtypeLocal, id: []byte("7"), desc: "Fa0/7"},
	}
	pdus := []gosnmp.SnmpPDU{
		lldpRemPDU("4", "1", "7", "1", gosnmp.Integer, 4),
		lldpRemPDU("5", "1", "7", "1", gosnmp.OctetString, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}),
		lldpRemPDU("9", "1", "7", "1", gosnmp.OctetString, []byte("neighbor")),
	}
	got := parseLLDPNeighbors(pdus, oidLLDPRemTable, locPorts, ifaces, nil)
	if assert.Len(t, got, 1) {
		assert.EqualValues(t, 7, got[0].LocalIfIndex)
		assert.Equal(t, "Fa0/7", got[0].LocalIfName)
	}
}

func TestMergeNeighbors_LLDPAndCDP(t *testing.T) {
	// Same local ifIndex and remote sysName seen via LLDP and CDP should
	// collapse into a single record with Source=lldp+cdp.
	in := []kt.TopologyNeighbor{
		{
			Source:          kt.NeighborSourceLLDP,
			LocalIfIndex:    3,
			LocalIfName:     "Eth1/3",
			RemoteSysName:   "peer-a",
			RemoteChassisID: "00:11:22:33:44:55",
			RemotePortID:    "Eth1/1",
		},
		{
			Source:         kt.NeighborSourceCDP,
			LocalIfIndex:   3,
			LocalIfName:    "Eth1/3",
			RemoteSysName:  "peer-a",
			RemotePortID:   "Eth1/1",
			RemotePlatform: "cisco N9K",
		},
	}
	out := mergeNeighbors(in)
	if assert.Len(t, out, 1) {
		assert.Equal(t, kt.NeighborSourceBoth, out[0].Source)
		assert.Equal(t, "00:11:22:33:44:55", out[0].RemoteChassisID)
		assert.Equal(t, "cisco N9K", out[0].RemotePlatform)
	}
}

func TestMergeNeighbors_DistinctNeighborsStay(t *testing.T) {
	in := []kt.TopologyNeighbor{
		{Source: kt.NeighborSourceLLDP, LocalIfIndex: 1, RemoteSysName: "a"},
		{Source: kt.NeighborSourceLLDP, LocalIfIndex: 2, RemoteSysName: "b"},
	}
	out := mergeNeighbors(in)
	assert.Len(t, out, 2, "same protocol on different ports should not be merged")
}

func TestSelectedNeighborProtocols(t *testing.T) {
	both := selectedNeighborProtocols(&kt.SnmpDeviceConfig{})
	assert.True(t, both[neighborProtocolLLDP])
	assert.True(t, both[neighborProtocolCDP])

	onlyLLDP := selectedNeighborProtocols(&kt.SnmpDeviceConfig{NeighborProtocols: []string{"lldp"}})
	assert.True(t, onlyLLDP[neighborProtocolLLDP])
	assert.False(t, onlyLLDP[neighborProtocolCDP])

	bogus := selectedNeighborProtocols(&kt.SnmpDeviceConfig{NeighborProtocols: []string{"garbage"}})
	assert.False(t, bogus[neighborProtocolLLDP])
	assert.False(t, bogus[neighborProtocolCDP])
}
