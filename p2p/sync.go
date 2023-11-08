package p2p

import (
	"context"
	"fmt"
	"math/rand"

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

// Todo: how to get tha latest header of peer:
// - Add latest tag or add latest header request.

type syncService struct {
	height   uint64 // todo: remove later, instead use blockchain
	host     host.Host
	network  utils.Network
	bootNode peer.ID

	blockchain *blockchain.Blockchain
	log        utils.SimpleLogger
}

func newSyncService(bc *blockchain.Blockchain, h host.Host, bootNode peer.ID, network utils.Network,
	log utils.SimpleLogger,
) *syncService {
	return &syncService{
		height:     0,
		host:       h,
		network:    network,
		blockchain: bc,
		log:        log,
		bootNode:   bootNode,
	}
}

func (s *syncService) start(ctx context.Context) {
	store := s.host.Peerstore()
	peers := store.Peers()

	bootNodeHeight, err := s.bootNodeHeight(ctx)
	if err != nil {
		s.log.Errorw("Failed to get boot node height", "err", err)
		return
	}
	s.log.Infow("Boot node height", "height", bootNodeHeight)

	// todo do not request same block from all peers
	peers = utils.Filter(peers, func(peerID peer.ID) bool {
		return peerID != s.host.ID()
	})
	if len(peers) == 0 {
		s.log.Errorw("No peers available")
		return
	}
	idx := rand.Intn(len(peers))
	randomPeer := peers[idx]

	s.requestBlockHeaders(ctx, s.height, bootNodeHeight, randomPeer)
}

func (s *syncService) bootNodeHeight(ctx context.Context) (uint64, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, s.bootNode, pids...)
	}, s.network, s.log)

	headersIt, err := c.RequestCurrentBlockHeader(ctx, &spec.CurrentBlockHeaderRequest{})
	if err != nil {
		s.log.Errorw("request block header from peer", "id", s.bootNode, "err", err)
	}

	res, valid := headersIt()
	if !valid {
		return 0, fmt.Errorf("failed to fetch boot node height (iterator is not valid)")
	}

	header := res.GetPart()[0].GetHeader()
	return header.Number, nil
}

func (s *syncService) requestBlockHeaders(ctx context.Context, start, stop uint64, id peer.ID) {
	it := &spec.Iteration{
		Start:     &spec.Iteration_BlockNumber{start},
		Direction: spec.Iteration_Forward,
		Limit:     stop + 1,
		Step:      1,
	}

	headers, err := s.requestBlockHeader(ctx, id, it)
	if err != nil {
		s.log.Errorw("Failed to request block headers from peer", "peerID", id, "err", err)
		return
	}

	fmt.Printf("Total number of headers is %d, here is last one %+v", len(headers), headers[0])
}

// todo rename method
func (s *syncService) requestBlockHeader(ctx context.Context, id peer.ID, it *spec.Iteration) ([]core.Header, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	headersIt, err := c.RequestBlockHeaders(ctx, &spec.BlockHeadersRequest{Iteration: it})
	if err != nil {
		s.log.Errorw("request block header from peer", "id", id, "err", err)
	}

	var headers []core.Header
	for res, valid := headersIt(); valid; res, valid = headersIt() {
		var (
			header     core.Header
			signatures [][]*felt.Felt
		)
		for _, part := range res.GetPart() {
			switch part.HeaderMessage.(type) {
			case *spec.BlockHeadersResponsePart_Header:
				receipts, err := s.requestReceipts(ctx, id, it)
				if err != nil {
					return nil, err
				}

				h := part.GetHeader()
				header = core.Header{
					Hash:             nil, // todo SPEC
					ParentHash:       p2p2core.AdaptHash(h.ParentHeader),
					Number:           h.Number,
					GlobalStateRoot:  p2p2core.AdaptHash(h.State.Root),
					SequencerAddress: p2p2core.AdaptAddress(h.SequencerAddress),
					TransactionCount: uint64(h.Transactions.NLeaves),
					EventCount:       uint64(h.Events.NLeaves),
					Timestamp:        uint64(h.Time.AsTime().Second()) + 1,
					ProtocolVersion:  "",  // todo SPEC
					ExtraData:        nil, // todo SPEC
					EventsBloom:      core.EventsBloom(receipts),
					GasPrice:         nil, // todo SPEC
				}
			case *spec.BlockHeadersResponsePart_Signatures:
				// todo check blockID
				signatures = utils.Map(part.GetSignatures().Signatures, p2p2core.AdaptSignature)
			case *spec.BlockHeadersResponsePart_Fin:
				//if i != 2 {
				//	return nil, fmt.Errorf("fin message received as %d part (header,signatures are missing?)", i)
				//}
			}
		}

		header.Signatures = signatures
		headers = append(headers, header)
	}

	return headers, nil
}

func (s *syncService) requestReceipts(ctx context.Context, id peer.ID, it *spec.Iteration) ([]*core.TransactionReceipt, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	receiptsIt, err := c.RequestReceipts(ctx, &spec.ReceiptsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	var receipts []*core.TransactionReceipt
	count := 0
	for res, valid := receiptsIt(); valid; res, valid = receiptsIt() {
		switch res.Responses.(type) {
		case *spec.ReceiptsResponse_Receipts:
			items := res.GetReceipts().GetItems()
			for _, item := range items {
				receipts = append(receipts, p2p2core.AdaptReceipt(item))
			}
		case *spec.ReceiptsResponse_Fin:
			if count < 1 {
				return nil, fmt.Errorf("fin received before receipts: %d", count)
			}
		}
		count++
	}

	return receipts, nil
}

func (s *syncService) requestTransactions(ctx context.Context, id peer.ID, it *spec.Iteration) ([]core.Transaction, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	transactionsIt, err := c.RequestTransactions(ctx, &spec.TransactionsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	var transactions []core.Transaction
	for res, valid := transactionsIt(); valid; res, valid = transactionsIt() {
		switch res.Responses.(type) {
		case *spec.TransactionsResponse_Transactions:
			items := res.GetTransactions().Items
			for _, item := range items {
				transactions = append(transactions, p2p2core.AdaptTransaction(item))
			}
		case *spec.TransactionsResponse_Fin:
			// todo what should we do here?
		}
	}

	return transactions, nil
}

//func (s *syncService) requestEvents(ctx context.Context, peerInfo peer.AddrInfo, it *spec.Iteration) ([]*core.Event, error) {
//	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
//		return s.host.NewStream(ctx, peerInfo.ID, pids...)
//	}, s.network, s.log)
//
//	eventsIt, err := c.RequestEvents(ctx, &spec.EventsRequest{Iteration: it})
//	if err != nil {
//		return nil, err
//	}
//
//	var events []*core.Event
//	count := 0
//	for res, valid := eventsIt(); valid; res, valid = eventsIt() {
//		switch res.Responses.(type) {
//		case *spec.EventsResponse_Events:
//			items := res.GetEvents().GetItems()
//			for _, item := range items {
//				events = append(events, p2p2core.AdaptEvent(item))
//			}
//		case *spec.EventsResponse_Fin:
//			if count < 1 {
//				return nil, fmt.Errorf("fin received before events: %d", count)
//			}
//		}
//		count++
//	}
//
//	return events, nil
//}
