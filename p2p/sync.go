package p2p

import (
	"context"

	"github.com/NethermindEth/juno/adapters/p2p2core"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"

	"github.com/NethermindEth/juno/blockchain"

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

type syncService struct {
	height  uint64
	host    host.Host
	network utils.Network

	blockchain *blockchain.Blockchain
	log        utils.SimpleLogger
}

func newSyncService(bc *blockchain.Blockchain, host host.Host, network utils.Network,
	log utils.SimpleLogger,
) *syncService {
	return &syncService{
		height:     0,
		host:       host,
		network:    network,
		blockchain: bc,
		log:        log,
	}
}

func (s *syncService) start(ctx context.Context) {
	store := s.host.Peerstore()
	peers := store.Peers()

	peersAddrInfo := utils.Map(peers, store.PeerInfo)
	for _, peerInfo := range peersAddrInfo {
		s.requestBlockHeader(ctx, peerInfo)
	}

	// Open streams will all the peers and request block headers
}

func (s *syncService) requestBlockHeader(ctx context.Context, peerInfo peer.AddrInfo) {
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
		// processing one block after another
		h := res.GetPart()[0].GetHeader()

		var (
			block core.Block
			// todo ask for rest of data:
			blockCommitments *core.BlockCommitments
			stateUpdate      *core.StateUpdate
			newClasses       map[felt.Felt]core.Class
			receipts         []*core.TransactionReceipt
		)

		for _, part := range res.GetPart() {
			switch part.HeaderMessage.(type) {
			case *spec.BlockHeadersResponsePart_Header:
				block.Header = &core.Header{
					Hash:             nil, // how?
					ParentHash:       p2p2core.AdaptHash(h.ParentHeader),
					Number:           h.Number,
					GlobalStateRoot:  p2p2core.AdaptHash(h.State.Root),
					SequencerAddress: p2p2core.AdaptAddress(h.SequencerAddress),
					TransactionCount: uint64(h.Transactions.NLeaves),
					EventCount:       uint64(h.Events.NLeaves),
					Timestamp:        uint64(h.Time.AsTime().Second()) + 1,
					ProtocolVersion:  "",  // todo ?
					ExtraData:        nil, // todo where?
					EventsBloom:      core.EventsBloom(receipts),
					GasPrice:         nil,
					Signatures:       nil,
				}
			case *spec.BlockHeadersResponsePart_Signatures:
				// assumption that Signatures go immediately after Header struct
				// todo check blockID
				// todo block.Signatures = part.GetSignatures().Signatures ?
			case *spec.BlockHeadersResponsePart_Fin:
				// what should we do here?
			}
		}

		s.blockchain.Store(block, blockCommitments, stateUpdate, newClasses)
	}
}
