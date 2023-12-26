package p2p

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/utils/pipeline"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func (s *syncService) startPipeline(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.client = starknet.NewClient(s.randomPeerStream, s.network, s.log)

	var bootNodeHeight uint64
	for i := 0; ; i++ {
		s.log.Infow("Continous iteration", "i", i)

		var err error
		bootNodeHeight, err = s.bootNodeHeight(ctx)
		if err != nil {
			s.log.Errorw("Failed to get boot node height", "err", err)
			return
		}
		s.log.Infow("Boot node height", "height", bootNodeHeight)

		var nextHeight uint64
		if curHeight, err := s.blockchain.Height(); err == nil { //nolint:govet
			nextHeight = curHeight + 1
		} else if !errors.Is(db.ErrKeyNotFound, err) {
			s.log.Errorw("Failed to get current height", "err", err)
		}

		commonIt := s.createIterator(BlockRange{nextHeight, bootNodeHeight})
		headersAndSigsCh, err := s.genHeadersAndSigs(ctx, commonIt)
		if err != nil {
			s.log.Errorw("Failed to get block headers parts", "err", err)
			return
		}

		blockBodiesCh, err := s.genBlockBodies(ctx, commonIt)
		if err != nil {
			s.log.Errorw("Failed to get block bodies", "err", err)
			return
		}

		txsCh, err := s.genTransactions(ctx, commonIt)
		if err != nil {
			s.log.Errorw("Failed to get transactions", "err", err)
			return
		}

		receiptsCh, err := s.genReceipts(ctx, commonIt)
		if err != nil {
			s.log.Errorw("Failed to get receipts", "err", err)
			return
		}

		eventsCh, err := s.genEvents(ctx, commonIt)
		if err != nil {
			s.log.Errorw("Failed to get events", "err", err)
			return
		}

		// A channel of a specific type cannot be converted to a channel of another type. Therefore, we have to consume/read from the channel
		// and change the input to the desired type. The following is not allowed:
		// var ch1 chan any = make(chan any)
		// var ch2 chan someOtherType = make(chan someOtherType)
		// ch2 = (chan any)(ch2) <----- This line will give compilation error.

		for b := range pipeline.Bridge(ctx,
			s.processSpecBlockParts(ctx, nextHeight,
				pipeline.FanIn(ctx,
					pipeline.Stage(ctx, headersAndSigsCh, func(i specBlockHeaderAndSigs) specBlockParts { return i }),
					pipeline.Stage(ctx, blockBodiesCh, func(i specBlockBody) specBlockParts { return i }),
					pipeline.Stage(ctx, txsCh, func(i specTransactions) specBlockParts { return i }),
					pipeline.Stage(ctx, receiptsCh, func(i specReceipts) specBlockParts { return i }),
					pipeline.Stage(ctx, eventsCh, func(i specEvents) specBlockParts { return i }),
				))) {
			if b.err != nil {
				// cannot process any more blocks
				s.log.Errorw("Failed to process block", "err", b.err)
				return
			}
			err = s.blockchain.Store(b.block, b.commitments, b.stateUpdate, b.newClasses)
			if err != nil {
				s.log.Errorw("Failed to Store Block", "number", b.block.Number, "err", err)
			} else {
				s.log.Infow("Stored Block", "number", b.block.Number, "hash", b.block.Hash.ShortString(), "root",
					b.block.GlobalStateRoot.ShortString())
			}
		}
	}
}

func (s *syncService) processSpecBlockParts(ctx context.Context, startingBlockNum uint64,
	specBlockPartsCh <-chan specBlockParts,
) <-chan <-chan blockBody {
	orderedBlockBodiesCh := make(chan (<-chan blockBody))

	go func() {
		defer close(orderedBlockBodiesCh)

		specBlockHeadersAndSigsM := make(map[uint64]specBlockHeaderAndSigs)
		specBlockBodiesM := make(map[uint64]specBlockBody)
		specTransactionsM := make(map[uint64]specTransactions)
		specReceiptsM := make(map[uint64]specReceipts)
		specEventsM := make(map[uint64]specEvents)

		curBlockNum := startingBlockNum
		for part := range specBlockPartsCh {
			select {
			case <-ctx.Done():
			default:
				switch p := part.(type) {
				case specBlockHeaderAndSigs:
					fmt.Printf("Received Block Header and Signatures for block number: %v\n", p.header.Number)
					if _, ok := specBlockHeadersAndSigsM[part.blockNumber()]; !ok {
						specBlockHeadersAndSigsM[part.blockNumber()] = p
					}
				case specBlockBody:
					fmt.Printf("Received Block Body parts            for block number: %v\n", p.id.Number)
					if _, ok := specBlockBodiesM[part.blockNumber()]; !ok {
						specBlockBodiesM[part.blockNumber()] = p
					}
				case specTransactions:
					fmt.Printf("Received Transactions                for block number: %v\n", p.id.Number)
					if _, ok := specTransactionsM[part.blockNumber()]; !ok {
						specTransactionsM[part.blockNumber()] = p
					}
				case specReceipts:
					fmt.Printf("Received Receipts                    for block number: %v\n", p.id.Number)
					if _, ok := specReceiptsM[part.blockNumber()]; !ok {
						specReceiptsM[part.blockNumber()] = p
					}
				case specEvents:
					fmt.Printf("Received Events                      for block number: %v\n", p.id.Number)
					if _, ok := specEventsM[part.blockNumber()]; !ok {
						specEventsM[part.blockNumber()] = p
					}
				}

				headerAndSig, ok1 := specBlockHeadersAndSigsM[curBlockNum]
				body, ok2 := specBlockBodiesM[curBlockNum]
				txs, ok3 := specTransactionsM[curBlockNum]
				rs, ok4 := specReceiptsM[curBlockNum]
				es, ok5 := specEventsM[curBlockNum]
				if ok1 && ok2 && ok3 && ok4 && ok5 {
					fmt.Println("----- Received all block parts from peers for block number:", curBlockNum, "-----")

					select {
					case <-ctx.Done():
					default:
						prevBlockRoot := &felt.Zero
						if curBlockNum > 0 {
							// First check cache if the header is not present, then get it from the db.
							if oldHeader, ok := specBlockHeadersAndSigsM[curBlockNum-1]; ok {
								prevBlockRoot = p2p2core.AdaptHash(oldHeader.header.State.Root)
							} else {
								oldHeader, err := s.blockchain.BlockHeaderByNumber(curBlockNum - 1)
								if err != nil {
									s.log.Errorw("Failed to get Header", "number", curBlockNum, "err", err)
									return
								}
								prevBlockRoot = oldHeader.GlobalStateRoot
							}
						}

						orderedBlockBodiesCh <- s.adaptAndSanityCheckBlock(ctx, headerAndSig.header, headerAndSig.sig, body.stateDiff,
							body.classes, body.proof, txs.txs, rs.receipts, es.events, prevBlockRoot)
					}

					if curBlockNum > 0 {
						delete(specBlockHeadersAndSigsM, curBlockNum-1)
					}
					delete(specBlockBodiesM, curBlockNum)
					delete(specTransactionsM, curBlockNum)
					delete(specReceiptsM, curBlockNum)
					delete(specEventsM, curBlockNum)
					curBlockNum++
				}
			}
		}
	}()
	return orderedBlockBodiesCh
}

func (s *syncService) adaptAndSanityCheckBlock(ctx context.Context, header *spec.BlockHeader, sig *spec.Signatures, diff *spec.StateDiff,
	classes *spec.Classes, proof *spec.BlockProof, txs *spec.Transactions, receipts *spec.Receipts, events *spec.Events,
	prevBlockRoot *felt.Felt,
) <-chan blockBody {
	bodyCh := make(chan blockBody)
	go func() {
		defer close(bodyCh)
		select {
		case <-ctx.Done():
			bodyCh <- blockBody{err: ctx.Err()}
		default:
			coreBlock := new(core.Block)

			var coreTxs []core.Transaction
			for _, i := range txs.GetItems() {
				coreTxs = append(coreTxs, p2p2core.AdaptTransaction(i, s.network))
			}

			coreBlock.Transactions = coreTxs

			txHashEventsM := make(map[felt.Felt][]*core.Event)
			for _, item := range events.GetItems() {
				txH := p2p2core.AdaptHash(item.TransactionHash)
				txHashEventsM[*txH] = append(txHashEventsM[*txH], p2p2core.AdaptEvent(item))
			}

			coreReceipts := utils.Map(receipts.GetItems(), p2p2core.AdaptReceipt)
			coreReceipts = utils.Map(coreReceipts, func(r *core.TransactionReceipt) *core.TransactionReceipt {
				r.Events = txHashEventsM[*r.TransactionHash]
				return r
			})
			coreBlock.Receipts = coreReceipts

			var err error

			coreHeader := p2p2core.AdaptBlockHeader(header)
			coreHeader.Signatures = utils.Map(sig.GetSignatures(), p2p2core.AdaptSignature)

			coreBlock.Header = &coreHeader
			coreBlock.EventsBloom = core.EventsBloom(coreBlock.Receipts)
			coreBlock.Hash, err = core.BlockHash(coreBlock)
			if err != nil {
				bodyCh <- blockBody{err: fmt.Errorf("block hash calculation error: %v", err)}
				return
			}

			newClasses := make(map[felt.Felt]core.Class)
			for _, i := range classes.GetClasses() {
				coreC := p2p2core.AdaptClass(i)
				h, err := coreC.Hash()
				if err != nil {
					bodyCh <- blockBody{err: fmt.Errorf("class hash calculation error: %v", err)}
					return
				}
				newClasses[*h] = coreC
			}

			// Build State update
			// Note: Parts of the State Update are created from Blockchain object as the Store and SanityCheck functions require a State
			// Update but there is no such message in P2P.

			stateUpdate := &core.StateUpdate{
				BlockHash: coreBlock.Hash,
				NewRoot:   coreBlock.GlobalStateRoot,
				OldRoot:   prevBlockRoot,
				StateDiff: p2p2core.AdaptStateDiff(diff, classes.GetClasses()),
			}

			commitments, err := s.blockchain.SanityCheckNewHeight(coreBlock, stateUpdate, newClasses)
			if err != nil {
				bodyCh <- blockBody{err: fmt.Errorf("sanity check error: %v for block number: %v", err, coreBlock.Number)}
				return
			}

			select {
			case <-ctx.Done():
			case bodyCh <- blockBody{block: coreBlock, stateUpdate: stateUpdate, newClasses: newClasses, commitments: commitments}:
			}
		}
	}()

	return bodyCh
}

type specBlockParts interface {
	blockNumber() uint64
}

type specBlockHeaderAndSigs struct {
	header *spec.BlockHeader
	sig    *spec.Signatures
}

func (s specBlockHeaderAndSigs) blockNumber() uint64 {
	return s.header.Number
}

func (s *syncService) genHeadersAndSigs(ctx context.Context, it *spec.Iteration) (<-chan specBlockHeaderAndSigs, error) {
	headersIt, err := s.client.RequestBlockHeaders(ctx, &spec.BlockHeadersRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	headersAndSigCh := make(chan specBlockHeaderAndSigs)
	go func() {
		defer close(headersAndSigCh)
		for res, valid := headersIt(); valid; res, valid = headersIt() {
			headerAndSig := specBlockHeaderAndSigs{}
			for _, part := range res.GetPart() {
				switch part.HeaderMessage.(type) {
				case *spec.BlockHeadersResponsePart_Header:
					headerAndSig.header = part.GetHeader()
				case *spec.BlockHeadersResponsePart_Signatures:
					headerAndSig.sig = part.GetSignatures()
				case *spec.BlockHeadersResponsePart_Fin:
					// received all the parts of BlockHeadersResponse
					return
				}
			}

			select {
			case <-ctx.Done():
				return
			case headersAndSigCh <- headerAndSig:
			}
		}
	}()

	return headersAndSigCh, nil
}

type specBlockBody struct {
	id        *spec.BlockID
	proof     *spec.BlockProof
	classes   *spec.Classes
	stateDiff *spec.StateDiff
}

func (s specBlockBody) blockNumber() uint64 {
	return s.id.Number
}

func (s *syncService) genBlockBodies(ctx context.Context, it *spec.Iteration) (<-chan specBlockBody, error) {
	blockIt, err := s.client.RequestBlockBodies(ctx, &spec.BlockBodiesRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	specBodiesCh := make(chan specBlockBody)
	go func() {
		defer close(specBodiesCh)
		curBlockBody := new(specBlockBody)
		// Assumes that all parts of the same block will arrive before the next block parts
		// Todo: the above assumption may not be true. A peer may decide to send different parts of the block in different order
		// If the above assumption is not true we should return separate channels for each of the parts. Also, see todo above specBlockBody
		// on line 317 in p2p/sync.go
		for res, valid := blockIt(); valid; res, valid = blockIt() {
			switch res.BodyMessage.(type) {
			case *spec.BlockBodiesResponse_Classes:
				if curBlockBody.id == nil {
					curBlockBody.id = res.GetId()
				}
				curBlockBody.classes = res.GetClasses()
			case *spec.BlockBodiesResponse_Diff:
				if curBlockBody.id == nil {
					curBlockBody.id = res.GetId()
				}
				curBlockBody.stateDiff = res.GetDiff()
			case *spec.BlockBodiesResponse_Proof:
				if curBlockBody.id == nil {
					curBlockBody.id = res.GetId()
				}
				curBlockBody.proof = res.GetProof()
			case *spec.BlockBodiesResponse_Fin:
				if curBlockBody.id != nil {
					select {
					case <-ctx.Done():
					default:
						specBodiesCh <- *curBlockBody
						curBlockBody = new(specBlockBody)
					}
				}
			}
		}
	}()

	return specBodiesCh, nil
}

type specReceipts struct {
	id       *spec.BlockID
	receipts *spec.Receipts
}

func (s specReceipts) blockNumber() uint64 {
	return s.id.Number
}

func (s *syncService) genReceipts(ctx context.Context, it *spec.Iteration) (<-chan specReceipts, error) {
	receiptsIt, err := s.client.RequestReceipts(ctx, &spec.ReceiptsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	receiptsCh := make(chan specReceipts)
	go func() {
		defer close(receiptsCh)

		for res, valid := receiptsIt(); valid; res, valid = receiptsIt() {
			switch res.Responses.(type) {
			case *spec.ReceiptsResponse_Receipts:
				select {
				case <-ctx.Done():
				case receiptsCh <- specReceipts{res.GetId(), res.GetReceipts()}:
				}
			case *spec.ReceiptsResponse_Fin:
				return
			}
		}
	}()

	return receiptsCh, nil
}

type specEvents struct {
	id     *spec.BlockID
	events *spec.Events
}

func (s specEvents) blockNumber() uint64 {
	return s.id.Number
}

func (s *syncService) genEvents(ctx context.Context, it *spec.Iteration) (<-chan specEvents, error) {
	eventsIt, err := s.client.RequestEvents(ctx, &spec.EventsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	eventsCh := make(chan specEvents)
	go func() {
		defer close(eventsCh)
		for res, valid := eventsIt(); valid; res, valid = eventsIt() {
			switch res.Responses.(type) {
			case *spec.EventsResponse_Events:
				select {
				case <-ctx.Done():
				case eventsCh <- specEvents{res.GetId(), res.GetEvents()}:
				}
			case *spec.EventsResponse_Fin:
				return
			}
		}
	}()
	return eventsCh, nil
}

type specTransactions struct {
	id  *spec.BlockID
	txs *spec.Transactions
}

func (s specTransactions) blockNumber() uint64 {
	return s.id.Number
}

func (s *syncService) genTransactions(ctx context.Context, it *spec.Iteration) (<-chan specTransactions, error) {
	txsIt, err := s.client.RequestTransactions(ctx, &spec.TransactionsRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	txsCh := make(chan specTransactions)
	go func() {
		defer close(txsCh)
		for res, valid := txsIt(); valid; res, valid = txsIt() {
			switch res.Responses.(type) {
			case *spec.TransactionsResponse_Transactions:
				select {
				case <-ctx.Done():
				case txsCh <- specTransactions{res.GetId(), res.GetTransactions()}:
				}
			case *spec.TransactionsResponse_Fin:
				return
			}
		}
	}()
	return txsCh, nil
}
