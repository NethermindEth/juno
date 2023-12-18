package p2p

import (
	"context"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
)

func (s *syncService) startPipeline(ctx context.Context) {
	s.client = starknet.NewClient(s.randomPeerStream, s.network, s.log)

	bootNodeHeight, err := s.bootNodeHeight(ctx)
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

	specBlockPartsFanIn(ctx,
		utils.PipelineFanIn(ctx,
			utils.PipelineStage(ctx, headersAndSigsCh, func(in blockHeaderAndSigs) any { return in }),
			utils.PipelineStage(ctx, blockBodiesCh, func(in specBlockBody) any { return in }),
			utils.PipelineStage(ctx, txsCh, func(in specTransactions) any { return in }),
			utils.PipelineStage(ctx, receiptsCh, func(in specReceipts) any { return in }),
			utils.PipelineStage(ctx, eventsCh, func(in specEvents) any { return in }),
		),
	)
}

func specBlockPartsFanIn(ctx context.Context, specBlockPartsCh <-chan any) {
	for part := range specBlockPartsCh {
		select {
		case <-ctx.Done():
		default:
			var partStr string

			switch p := part.(type) {
			case blockHeaderAndSigs:
				partStr = fmt.Sprintf("Block Header and Signatures for block number: %v\n", p.header.Number)
			case specBlockBody:
				partStr = fmt.Sprintf("Block Body parts            for block number: %v\n", p.id.Number)
			case specTransactions:
				partStr = fmt.Sprintf("Transactions                for block number: %v\n", p.id.Number)
			case specReceipts:
				partStr = fmt.Sprintf("Receipts                    for block number: %v\n", p.id.Number)
			case specEvents:
				partStr = fmt.Sprintf("Events                      for block number: %v\n", p.id.Number)
			}

			fmt.Print(partStr)
		}
	}
}

type blockHeaderAndSigs struct {
	header *spec.BlockHeader
	sig    *spec.Signatures
}

func (s *syncService) genHeadersAndSigs(ctx context.Context, it *spec.Iteration) (<-chan blockHeaderAndSigs, error) {
	headersIt, err := s.client.RequestBlockHeaders(ctx, &spec.BlockHeadersRequest{Iteration: it})
	if err != nil {
		return nil, err
	}

	headersAndSigCh := make(chan blockHeaderAndSigs)
	go func() {
		defer close(headersAndSigCh)
		for res, valid := headersIt(); valid; res, valid = headersIt() {
			headerAndSig := blockHeaderAndSigs{}
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

func adaptBlockHeadersAndSigs(headerAndSig blockHeaderAndSigs) core.Header {
	header := p2p2core.AdaptBlockHeader(headerAndSig.header)
	header.Signatures = utils.Map(headerAndSig.sig.GetSignatures(), p2p2core.AdaptSignature)
	return header
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
	id  *spec.BlockID
	Txs *spec.Receipts
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
	id  *spec.BlockID
	Txs *spec.Events
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
	Txs *spec.Transactions
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
