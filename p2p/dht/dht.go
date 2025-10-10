package dht

import (
	"context"
	"fmt"
	"strings"

	"github.com/NethermindEth/juno/p2p/starknetp2p"
	"github.com/NethermindEth/juno/utils"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

func New(
	ctx context.Context,
	host host.Host,
	network *utils.Network,
	starknetProtocol starknetp2p.Protocol,
	bootstrapPeersFn func() []peer.AddrInfo,
) (*dht.IpfsDHT, error) {
	return dht.New(
		ctx,
		host,
		append(
			starknetp2p.DHT(network, starknetProtocol),
			dht.BootstrapPeersFunc(bootstrapPeersFn),
			dht.Mode(dht.ModeServer),
		)...,
	)
}

func ExtractPeers(peers string) ([]peer.AddrInfo, error) {
	if peers == "" {
		return nil, nil
	}

	peerAddrs := []peer.AddrInfo{}
	for peerStr := range strings.SplitSeq(peers, ",") {
		peerAddr, err := peer.AddrInfoFromString(peerStr)
		if err != nil {
			return nil, fmt.Errorf("unable to parse peer address %q: %w", peerStr, err)
		}
		peerAddrs = append(peerAddrs, *peerAddr)
	}
	return peerAddrs, nil
}
