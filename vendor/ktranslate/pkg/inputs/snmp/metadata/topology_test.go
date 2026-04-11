package metadata

import (
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
