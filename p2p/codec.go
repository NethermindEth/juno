package p2p

import (
	"fmt"

	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/multiformats/go-multiaddr"
)

// EncodeAddrs encodes a slice of multiaddrs into a byte slice
func EncodeAddrs(addrs []multiaddr.Multiaddr) ([]byte, error) {
	multiAddrBytes := make([][]byte, len(addrs))
	for i, addr := range addrs {
		multiAddrBytes[i] = addr.Bytes()
	}

	encoded, err := cbor.Marshal(multiAddrBytes)
	if err != nil {
		return nil, fmt.Errorf("encode addresses: %w", err)
	}

	return encoded, nil
}

// decodeAddrs decodes a byte slice into a slice of multiaddrs
func decodeAddrs(b []byte) ([]multiaddr.Multiaddr, error) {
	var multiAddrBytes [][]byte
	if err := cbor.Unmarshal(b, &multiAddrBytes); err != nil {
		return nil, fmt.Errorf("decode addresses: %w", err)
	}

	addrs := make([]multiaddr.Multiaddr, 0, len(multiAddrBytes))
	for _, addrBytes := range multiAddrBytes {
		addr, err := multiaddr.NewMultiaddrBytes(addrBytes)
		if err != nil {
			return nil, fmt.Errorf("parse multiaddr: %w", err)
		}
		addrs = append(addrs, addr)
	}

	return addrs, nil
}
