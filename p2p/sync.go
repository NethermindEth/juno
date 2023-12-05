package p2p

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"

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

	var curHeight int32 = -1

	logBlocks := func(prefix string, input <-chan blockBody) <-chan blockBody {
		return utils.Pipeline(input, func(item blockBody) blockBody {
			fmt.Println(prefix, item.block.Number)
			return item
		})
	}

	fetchBlocks := func(ranges <-chan BlockRange) <-chan blockBody {
		coreBlocks := make(chan blockBody)

		go func() {
			defer close(coreBlocks)

			for {
				select {
				case r := <-ranges:
					go s.requestBlocks(ctx, r, coreBlocks)
				case <-ctx.Done():
					s.log.Infow("Context done in p2p sync. Exiting", "err", ctx.Err())
					return
				}
			}
		}()

		return coreBlocks
	}

	sanityChecks := func(blockBodies <-chan blockBody, fetchBlocks chan<- BlockRange) <-chan blockBody {
		// check structural integrity of the block and signatures

		checkedBlocks := make(chan blockBody)
		go func() {
			defer close(checkedBlocks)

			for blockBody := range blockBodies {
				// do santify check
				var checkFailed bool

				checkedBlocks <- blockBody
				if checkFailed {
					s.log.Debugw("Check failed, refetching block range", "blockBody", blockBody)
					fetchBlocks <- BlockRange{
						Start: blockBody.block.Number,
						End:   blockBody.block.Number + 1,
					}
				}
			}
		}()

		return checkedBlocks
	}

	unorderStream := func(items <-chan blockBody) <-chan blockBody {
		out := make(chan blockBody)

		go func() {
			defer close(out)

			receivedItems := make(map[uint64]blockBody, 10)
			for item := range items {
				receivedItems[item.block.Number] = item

				if len(receivedItems) == 10 {
					for key, value := range receivedItems {
						out <- value
						delete(receivedItems, key)
					}
				}
			}
		}()

		return out
	}

	orderCheckedBlocks := func(checkedBlockBodies <-chan blockBody) <-chan blockBody {
		orderBlocks := make(chan blockBody)

		outOfOrderBlocks := make(map[uint64]blockBody)
		go func() {
			defer close(orderBlocks)
			for checkedBlockBody := range checkedBlockBodies {
				h := uint64(atomic.LoadInt32(&curHeight))
				if checkedBlockBody.block.Number == h+1 {
					orderBlocks <- checkedBlockBody
				} else {
					outOfOrderBlocks[checkedBlockBody.block.Number] = checkedBlockBody

					// check if there is a blockBody already in the map
					if b, ok := outOfOrderBlocks[h+1]; ok {
						orderBlocks <- b

						delete(outOfOrderBlocks, h+1)
					}
				}
			}
		}()

		return orderBlocks
	}

	storeBlock := func(blockBody blockBody) {
		fmt.Println("Storing block Number", blockBody.block.Number)
		atomic.AddInt32(&curHeight, 1)

		if blockBody.block.Number != uint64(curHeight) {
			s.log.Warnw("Current height != block.Number", "curHeight", curHeight, "blockNumber", blockBody.block.Number)
		}

		return // skip storing for now
		blockCommitments, err := s.blockchain.SanityCheckNewHeight(blockBody.block, blockBody.stateUpdate, blockBody.newClasses)
		if err != nil {
			s.log.Errorw("Failed to sanity check new height", "err", err)
			panic(err)
			// todo pass back to pipeline
		}

		err = s.blockchain.Store(blockBody.block, blockCommitments, blockBody.stateUpdate, blockBody.newClasses)
		if err != nil {
			s.log.Errorw("Failed to store block", "err", err)
			panic(err)
			// todo pass back to pipeline
		}
	}

	blockRangeStream := make(chan BlockRange, 1)
	blockRangeStream <- BlockRange{
		Start: 0,
		End:   bootNodeHeight,
	}
	fetchedBlocks := fetchBlocks(blockRangeStream)
	fetchedBlocks = logBlocks("Fetched", fetchedBlocks)

	checkedBlocks := sanityChecks(fetchedBlocks, blockRangeStream)
	checkedBlocks = logBlocks("SanityChecked", checkedBlocks)

	unorderedBlocks := unorderStream(checkedBlocks)
	unorderedBlocks = logBlocks("Unordered", unorderedBlocks)

	orderedBlocks := orderCheckedBlocks(unorderedBlocks)
	orderedBlocks = logBlocks("Ordered", orderedBlocks)

	stopped := utils.PipelineEnd(orderedBlocks, storeBlock)
	<-stopped
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

type specBlockBody struct {
	id        *spec.BlockID
	proof     *spec.BlockProof
	classes   *spec.Classes
	stateDiff *spec.StateDiff
}

func (s *syncService) requestBlockBodies(ctx context.Context, it *spec.Iteration, id peer.ID) (map[uint64]specBlockBody, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	blockIt, err := c.RequestBlockBodies(ctx, &spec.BlockBodiesRequest{
		Iteration: it,
	})
	if err != nil {
		s.log.Errorw("request block bodies from peer", "id", id, "err", err)
	}

	blockBodies := make(map[uint64]specBlockBody)
	updateBlockBody := func(id *spec.BlockID, f func(body *specBlockBody)) {
		b, ok := blockBodies[id.Number]
		if !ok {
			b = specBlockBody{id: id}
		}

		f(&b)

		blockBodies[id.Number] = b
	}

	curBlockBody := new(specBlockBody)
	for res, valid := blockIt(); valid; res, valid = blockIt() {
		switch res.BodyMessage.(type) {
		case *spec.BlockBodiesResponse_Classes:
			if curBlockBody.id == nil {
				curBlockBody.id = res.GetId()
			}
			updateBlockBody(res.GetId(), func(b *specBlockBody) { b.classes = res.GetClasses() })
		case *spec.BlockBodiesResponse_Diff:
			updateBlockBody(res.GetId(), func(b *specBlockBody) { b.stateDiff = res.GetDiff() })
		case *spec.BlockBodiesResponse_Fin:
			// do nothing
		case *spec.BlockBodiesResponse_Proof:
			updateBlockBody(res.GetId(), func(b *specBlockBody) { b.proof = res.GetProof() })
		default:
			fmt.Printf("Unknown BlockBody type %T\n", res.BodyMessage)
		}
	}

	return blockBodies, nil
}

// todo
func (s *syncService) requestBlockHeaders(ctx context.Context, it *spec.Iteration, id peer.ID) ([]core.Header, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	headersIt, err := c.RequestBlockHeaders(ctx, &spec.BlockHeadersRequest{Iteration: it})
	if err != nil {
		s.log.Errorw("request block header from peer", "id", id, "err", err)
	}

	var headers []core.Header

iteratorLoop:
	for res, valid := headersIt(); valid; res, valid = headersIt() {
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

				header = core.Header{
					Hash:             nil, // todo: add this when building the block
					ParentHash:       p2p2core.AdaptHash(h.ParentHash),
					Number:           h.Number,
					GlobalStateRoot:  p2p2core.AdaptHash(h.State.Root),
					SequencerAddress: p2p2core.AdaptAddress(h.SequencerAddress),
					TransactionCount: uint64(h.Transactions.NLeaves),
					EventCount:       uint64(h.Events.NLeaves),
					Timestamp:        uint64(h.Time.AsTime().Second()) + 1,
					ProtocolVersion:  h.ProtocolVersion,
					EventsBloom:      nil, // Todo: add this in when building the block
					GasPrice:         p2p2core.AdaptFelt(h.GasPrice),
					Signatures:
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

	//blockNumbers := utils.Map(headers, func(h core.Header) string {
	//	return strconv.Itoa(int(h.Number))
	//})
	//fmt.Printf("Fetched block numbers: %v\n", strings.Join(blockNumbers, ","))

	return headers, nil
}

func (s *syncService) requestReceipts(ctx context.Context, it *spec.Iteration, id peer.ID) (map[uint64][]*core.TransactionReceipt, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	receiptsIt, err := c.RequestReceipts(ctx, &spec.ReceiptsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	receipts := make(map[uint64][]*core.TransactionReceipt, 0)
	count := 0
	for res, valid := receiptsIt(); valid; res, valid = receiptsIt() {
		switch res.Responses.(type) {
		case *spec.ReceiptsResponse_Receipts:
			blockNumber := res.GetId().Number
			items := res.GetReceipts().GetItems()
			for _, item := range items {
				receipts[blockNumber] = append(receipts[blockNumber], p2p2core.AdaptReceipt(item))
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

func (s *syncService) requestTransactions(ctx context.Context, it *spec.Iteration, id peer.ID) (map[uint64][]core.Transaction, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	transactionsIt, err := c.RequestTransactions(ctx, &spec.TransactionsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	transactions := make(map[uint64][]core.Transaction, 0)
	for res, valid := transactionsIt(); valid; res, valid = transactionsIt() {
		switch res.Responses.(type) {
		case *spec.TransactionsResponse_Transactions:
			blockNumber := res.GetId().Number
			items := res.GetTransactions().Items
			for _, item := range items {
				transactions[blockNumber] = append(transactions[blockNumber], p2p2core.AdaptTransaction(item, s.network))
			}
		case *spec.TransactionsResponse_Fin:
			// todo what should we do here?
		}
	}

	return transactions, nil
}

func (s *syncService) requestEvents(ctx context.Context, it *spec.Iteration, id peer.ID) (map[uint64]map[felt.Felt][]*core.Event, error) {
	c := starknet.NewClient(func(ctx context.Context, pids ...protocol.ID) (network.Stream, error) {
		return s.host.NewStream(ctx, id, pids...)
	}, s.network, s.log)

	eventsIt, err := c.RequestEvents(ctx, &spec.EventsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	events := make(map[uint64]map[felt.Felt][]*core.Event)
	count := 0
	for res, valid := eventsIt(); valid; res, valid = eventsIt() {
		switch res.Responses.(type) {
		case *spec.EventsResponse_Events:
			blockNumber := res.GetId().Number
			txHashM, ok := events[blockNumber]
			if !ok {
				events[blockNumber] = make(map[felt.Felt][]*core.Event)
				txHashM = events[blockNumber]
			}

			items := res.GetEvents().GetItems()
			for _, item := range items {
				txH := p2p2core.AdaptHash(item.TransactionHash)
				txHashM[*txH] = append(txHashM[*txH], p2p2core.AdaptEvent(item))
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

func (s *syncService) requestBlocks(ctx context.Context, r BlockRange, blocks chan<- blockBody) {
	fmt.Println("requestBlocks called for range", r)
	const limit = 10

	for i := r.Start; i < r.End; i += limit {
		it := s.createIterator(BlockRange{
			Start: i,
			End:   i + limit,
		})
		fmt.Println("Iterator ", it.Start, it.Limit)

		id := s.randomPeer()

		// todo: Ensure the header matches the body. Matching on the index is not enough as headers and bodies will come in different
		// order. Therefore, requestBlockHeaders should return *spec.Blockheader and *spec.BlockBody and match on BlockID
		headersFromPeer, err := s.requestBlockHeaders(ctx, it, id)
		if err != nil {
			s.log.Errorw("Failed to request blockHeaders", "err", err)
			return
		}

		blocksFromPeer, err := s.requestBlockBodies(ctx, it, id)
		if err != nil {
			s.log.Errorw("Failed to request blockBodies", "err", err)
			return
		}

		transactions, err := s.requestTransactions(ctx, it, id)
		if err != nil {
			s.log.Errorw("Failed to request transactions", "err", err)
			return
		}

		receipts, err := s.requestReceipts(ctx, it, id)
		if err != nil {
			s.log.Errorw("Failed to request receipts", "err", err)
			return
		}

		events, err := s.requestEvents(ctx, it, id)
		if err != nil {
			s.log.Errorw("Failed to request events", "err", err)
			return
		}

		if len(headersFromPeer) != len(blocksFromPeer) {
			s.log.Errorw("Number of headers doesn't match number of bodies",
				"headersNum", len(headersFromPeer),
				"bodiesNum", len(blocksFromPeer))
			return
		}

		for i, blockBody := range blocksFromPeer {
			header := &headersFromPeer[i]
			blockEvents := events[header.Number]

			blockBody.block.Transactions = transactions[header.Number]
			blockReceipts := utils.Map(receipts[header.Number], func(r *core.TransactionReceipt) *core.TransactionReceipt {
				r.Events = blockEvents[*r.TransactionHash]
				return r
			})
			blockBody.block.Receipts = blockReceipts
			header.EventsBloom = core.EventsBloom(blockReceipts)
			blockBody.block.Header = header
			// todo fill hash

			blocks <- blockBody
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
	limit := r.End - r.Start
	return &spec.Iteration{
		Start:     &spec.Iteration_BlockNumber{r.Start},
		Direction: spec.Iteration_Forward,
		Limit:     limit,
		Step:      1,
	}
}
