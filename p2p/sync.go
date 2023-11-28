package p2p

import (
	"context"
	"fmt"
	"math/rand"
	"time"

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

// represents range of blocks to request [start; end)
type BlockRange struct {
	Start, End uint64
}

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
	bootNodeHeight, err := s.bootNodeHeight(ctx)
	if err != nil {
		s.log.Errorw("Failed to get boot node height", "err", err)
		return
	}
	s.log.Infow("Boot node height", "height", bootNodeHeight)

	/*


		sync reads store with more checks <- blocks in order <---- adapt p2p to core <---- Sanitycheck block <---- peer1
															 <---- adapt p2p to core <---- Sanitycheck block <---- peer2
															 <---- adapt p2p to core <---- Sanitycheck block <---- peer3
		Retry mechanism if peer fails to provide a block
		Iterator object options:
			1. Only it to fetch one block at a time. We don't have to do any error handling if there are missing blocks.
			2. Use it to fetch multiple blocks from a peer at the same time.
				If peer doesn't have all the block requested we need to ask other peers

	*/

	fetchBlocks := func(ranges <-chan BlockRange) <-chan core.Block {
		coreBlocks := make(chan core.Block)

		go func() {
			defer close(coreBlocks)

			for r := range ranges {
				go s.requestBlocks(ctx, r, coreBlocks)
			}
		}()

		return coreBlocks
	}

	santiyChecks := func(blocks <-chan core.Block, fetchBlocks chan<- BlockRange) <-chan core.Block {
		// check structural integrity of the block and signatures

		checkedBlocks := make(chan core.Block)
		go func() {
			defer close(checkedBlocks)

			for block := range blocks {
				// do santify check
				var checkFailed bool

				checkedBlocks <- block
				if checkFailed {
					fetchBlocks <- BlockRange{
						Start: block.Number,
						End:   block.Number + 1,
					}
				}
			}
		}()

		return checkedBlocks
	}

	orderCheckedBlocks := func(checkedBlocks <-chan core.Block) <-chan core.Block {
		outOfOrderBlocks := make(map[uint64]core.Block)

		orderBlocks := make(chan core.Block)
		go func() {
			defer close(orderBlocks)
			for block := range checkedBlocks {
				curH, err := s.blockchain.Height()
				if block.Number == curH+1 {
					orderBlocks <- block
				} else {
					outOfOrderBlocks[block.Number] = block

					// check if there is a block already in the map
					if b, ok := outOfOrderBlocks[curH+1]; ok {
						orderBlocks <- b

						delete(outOfOrderBlocks, curH+1)
					}
				}
			}
		}()
		return orderBlocks
	}
	storeBlocks := func(blocks <-chan core.Block) <-chan struct{} {
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)

			for block := range blocks {
				_ = block
				// todo store block
			}
		}()

		return stopped
	}

	blockRangeStream := make(chan BlockRange, 1)
	blockRangeStream <- BlockRange{
		Start: 0,
		End:   bootNodeHeight,
	}
	fetchedBlocks := fetchBlocks(blockRangeStream)
	checkedBlocks := santiyChecks(fetchedBlocks, blockRangeStream)
	orderedBlocks := orderCheckedBlocks(checkedBlocks)
	stopped := storeBlocks(orderedBlocks)

	<-stopped

	//fmt.Println("header's start", s.height, "header's stop", bootNodeHeight)
	//headers, err := s.requestBlockHeaders(ctx, s.height, bootNodeHeight, randomPeer)
	//if err != nil {
	//	s.log.Errorw("Failed to get headers", "err", err)
	//	return
	//}
	//
	//fmt.Println("body's start", s.height, "body's stop", bootNodeHeight)
	//blockBodies, err := s.requestBlockBodies(ctx, s.height, bootNodeHeight, randomPeer)
	//if err != nil {
	//	s.log.Errorw("Failed to get block bodies", "err", err)
	//	return
	//}
	//
	//s.log.Debugw("Merging block bodies with headers", "bodiesLen", len(blockBodies), "headerLen", len(headers))
	//for i, body := range blockBodies {
	//	body.block.Header = &headers[i]
	//	// fmt.Printf("Received block body %d %+v\n", i, body)
	//}
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

// blockBody is used to mange all the different parts of the blocks require to store the block in the blockchain.Store()
type blockBody struct {
	block       *core.Block
	stateUpdate *core.StateUpdate
	newClasses  map[felt.Felt]core.Class
}

func newBlockBody() blockBody {
	return blockBody{
		block:       new(core.Block),
		stateUpdate: new(core.StateUpdate),
		newClasses:  make(map[felt.Felt]core.Class),
	}
}

func (s *syncService) requestBlockBodies(ctx context.Context, it *spec.Iteration, id peer.ID) ([]blockBody, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	blockIt, err := c.RequestBlockBodies(ctx, &spec.BlockBodiesRequest{
		Iteration: it,
	})
	if err != nil {
		s.log.Errorw("request block bodies from peer", "id", id, "err", err)
	}

	blockBodies := make(map[uint64]blockBody)
	updateBlockBody := func(blockNumber uint64, f func(*blockBody)) {
		b, ok := blockBodies[blockNumber]
		if !ok {
			b = newBlockBody()
		}

		f(&b)

		blockBodies[blockNumber] = b
	}

	var count int
	prevTime := time.Now()
	for res, valid := blockIt(); valid; res, valid = blockIt() {
		if count%6 == 0 {
			fmt.Printf("fetching header body=%d, timeSpent=%v\n", count, time.Since(prevTime))
		}
		prevTime = time.Now()
		count++

		switch res.BodyMessage.(type) {
		case *spec.BlockBodiesResponse_Classes:
			updateBlockBody(res.GetId().Number, func(b *blockBody) {
				classes := res.GetClasses().GetClasses()

				b.newClasses = utils.ToMap(classes, func(cls *spec.Class) (felt.Felt, core.Class) {
					coreCls := p2p2core.AdaptClass(cls)

					var hash *felt.Felt
					switch v := coreCls.(type) {
					case *core.Cairo0Class:
						hash = p2p2core.AdaptHash(cls.GetCairo0().Hash)
					case *core.Cairo1Class:
						hash = v.Hash()
					}

					return *hash, coreCls
				})
			})
		case *spec.BlockBodiesResponse_Diff:
			// res.GetDiff()
		case *spec.BlockBodiesResponse_Transactions:
			updateBlockBody(res.GetId().Number, func(b *blockBody) {
				for _, item := range res.GetTransactions().GetItems() {
					b.block.Transactions = append(b.block.Transactions, p2p2core.AdaptTransaction(item, s.network))
				}
			})
		case *spec.BlockBodiesResponse_Receipts:
			updateBlockBody(res.GetId().Number, func(b *blockBody) {
				b.block.Receipts = utils.Map(res.GetReceipts().GetItems(), p2p2core.AdaptReceipt)
			})
		case *spec.BlockBodiesResponse_Fin:
			// do nothing
		case *spec.BlockBodiesResponse_Proof:
			// do nothing
		default:
			fmt.Printf("Unknown BlockBody type %T\n", res.BodyMessage)
		}
	}

	return utils.MapValues(blockBodies), nil
}

// todo rename method
func (s *syncService) requestBlockHeaders(ctx context.Context, it *spec.Iteration, id peer.ID) ([]core.Header, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	headersIt, err := c.RequestBlockHeaders(ctx, &spec.BlockHeadersRequest{Iteration: it})
	if err != nil {
		s.log.Errorw("request block header from peer", "id", id, "err", err)
	}

	var headers []core.Header
	var count int32
	prevTime := time.Now()
	for res, valid := headersIt(); valid; res, valid = headersIt() {
		fmt.Printf("fetching header number=%d, timeSpent=%v\n", count, time.Since(prevTime))
		prevTime = time.Now()
		count++

		var (
			header     core.Header
			signatures [][]*felt.Felt
		)
		parts := res.GetPart()
		if len(parts) == 1 {
			// assumption that parts contain only Fin element
			continue
		}

		for i, part := range parts {
			switch part.HeaderMessage.(type) {
			case *spec.BlockHeadersResponsePart_Header:
				h := part.GetHeader()

				receipts, err := s.requestReceipts(ctx, id, &spec.Iteration{
					Start: &spec.Iteration_BlockNumber{
						BlockNumber: h.Number,
					},
					Direction: spec.Iteration_Forward,
					Limit:     1,
					Step:      1,
				})
				if err != nil {
					return nil, err
				}
				// s.log.Infow("time spend in receipts", "timeSpend", delta)

				header = core.Header{
					Hash:             nil, // todo SPEC
					ParentHash:       p2p2core.AdaptHash(h.ParentHash),
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
				if i != 2 {
					return nil, fmt.Errorf("fin message received as %d part (header,signatures are missing?)", i)
				}
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
				transactions = append(transactions, p2p2core.AdaptTransaction(item, s.network))
			}
		case *spec.TransactionsResponse_Fin:
			// todo what should we do here?
		}
	}

	return transactions, nil
}

func (s *syncService) requestEvents(ctx context.Context, id peer.ID, it *spec.Iteration) ([]*core.Event, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	eventsIt, err := c.RequestEvents(ctx, &spec.EventsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	var events []*core.Event
	count := 0
	for res, valid := eventsIt(); valid; res, valid = eventsIt() {
		switch res.Responses.(type) {
		case *spec.EventsResponse_Events:
			items := res.GetEvents().GetItems()
			for _, item := range items {
				events = append(events, p2p2core.AdaptEvent(item))
			}
		case *spec.EventsResponse_Fin:
			if count < 1 {
				return nil, fmt.Errorf("fin received before events: %d", count)
			}
		}
		count++
	}

	return events, nil
}

func (s *syncService) requestBlocks(ctx context.Context, r BlockRange, blocks chan<- core.Block) {
	const limit = 10

	for i := r.Start; i < r.End; i += limit {
		it := s.createIterator(BlockRange{
			Start: i,
			End:   i + limit,
		})

		id := s.randomPeer()
		headersFromPeer, err := s.requestBlockHeaders(ctx, it, id)
		if err != nil {
		}

		blocksFromPeer, err := s.requestBlockBodies(ctx, it, id)
		if err != nil {
		}
	}
}

func (s *syncService) randomPeer() peer.ID {
	peers := s.host.Peerstore().Peers()

	// todo do not request same block from all peers
	peers = utils.Filter(peers, func(peerID peer.ID) bool {
		return peerID != s.host.ID()
	})
	if len(peers) == 0 {
		panic("No peers available")
	}

	idx := rand.Intn(len(peers))
	return peers[idx]
}

func (s *syncService) createIterator(r BlockRange) *spec.Iteration {
	return &spec.Iteration{
		Start:     &spec.Iteration_BlockNumber{r.Start},
		Direction: spec.Iteration_Forward,
		Limit:     r.End,
		Step:      1,
	}
}
