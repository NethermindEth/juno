package p2p

import (
	"bytes"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/multiformats/go-multiaddr"
)

// EncodeAddrs encodes a slice of multiaddrs into a byte slice
func EncodeAddrs(addrs []multiaddr.Multiaddr) ([]byte, error) {
	multiAddrBytes := make([][]byte, len(addrs))
	for i, addr := range addrs {
		multiAddrBytes[i] = addr.Bytes()
	}

	var buf bytes.Buffer
	if err := cbor.NewEncoder(&buf).Encode(multiAddrBytes); err != nil {
		return nil, fmt.Errorf("encode addresses: %w", err)
	}

	return buf.Bytes(), nil
}

// decodeAddrs decodes a byte slice into a slice of multiaddrs
func decodeAddrs(b []byte) ([]multiaddr.Multiaddr, error) {
	var multiAddrBytes [][]byte
	if err := cbor.NewDecoder(bytes.NewReader(b)).Decode(&multiAddrBytes); err != nil {
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
