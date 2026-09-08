package common

import (
	"encoding/binary"
	"net/netip"
)

type AnnounceResponse struct {
	Interval   uint32           `bencode:"interval"`   // Minimum time before refreshing the announcement
	Incomplete uint32           `bencode:"incomplete"` // Num of peers still downloading the files (leeches)
	Complete   uint32           `bencode:"complete"`   // Num of peers with all files (seeds)
	Peers      []netip.AddrPort `bencode:"-"`          // IPv4 peers list (6 bytes per peer)
	Peers6     []netip.AddrPort `bencode:"-"`          // IPv6 peers list (18 bytes per peer)
}

func ParseSocket(addrBinary []byte, ipSize, portSize int) []netip.AddrPort {
	addrSize := ipSize + portSize

	peersCount := len(addrBinary) / addrSize
	peers := make([]netip.AddrPort, 0, peersCount)

	for i := 0; i < len(addrBinary); i += addrSize {
		addr, _ := netip.AddrFromSlice(addrBinary[i : i+ipSize])
		port := binary.BigEndian.Uint16(addrBinary[i+ipSize : i+addrSize])
		peers = append(peers, netip.AddrPortFrom(addr, port))
	}
	return peers
}
