package p2p

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// 1. Access to db / Blockchain
// 2. Adapters to p2p2core
// 3. Determine until which height to sync by fetching boot peer's heigh
// 	a. Fetch boot peer header
// 	b. Verify signature the header.

type sync struct {
	height  uint64
	host    host.Host
	network utils.Network

	log utils.Logger
}

func newSync(host host.Host, network utils.Network, log utils.Logger) *sync {
	return &sync{
		height:  0,
		host:    host,
		network: network,
		log:     log,
	}
}

func (s *sync) start(ctx context.Context) error {
	store := s.host.Peerstore()
	peers := store.Peers()

	peersAddrInfo := utils.Map(peers, store.PeerInfo)
	for _, peerInfo := range peersAddrInfo {
		s.requestBlockHeader(ctx, peerInfo)
	}

	// Open streams will all the peers and request block headers
	return nil
}

func (s *sync) requestBlockHeader(ctx context.Context, peerInfo peer.AddrInfo) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, peerInfo.ID, pids...)
	}, s.network, s.log)

	header, err := c.RequestBlockHeaders(ctx, &spec.BlockHeadersRequest{Iteration: &spec.Iteration{
		Start:     &spec.Iteration_BlockNumber{s.height},
		Direction: spec.Iteration_Forward,
		Limit:     1,
		Step:      1,
	}})
	if err != nil {
		s.log.Errorw("request block header from peer", "id", peerInfo.ID, "err", err)
	}

	for res, valid := header(); valid; res, valid = header() {
		fmt.Println(res.GetPart()[0].GetHeader().String())
	}
}
