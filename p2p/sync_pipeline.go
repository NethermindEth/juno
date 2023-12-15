package p2p

import (
	"context"
	"errors"
	"fmt"
	"sync"

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
	headersAndSigsCh, err := s.genBlockHeadersAndSigs(ctx, commonIt)
	if err != nil {
		s.log.Errorw("Failed to get block headers parts", "err", err)
	}

	blockBodiesCh, err := s.genBlockBodies(ctx, commonIt)

	var wg sync.WaitGroup
	wg.Add(2)
	//for h := range utils.PipelineStage(ctx, headersAndSigsCh, adaptBlockHeadersAndSigs) {
	//	spew.Dump(h)
	//}

	go func() {
		defer wg.Done()
		for h := range headersAndSigsCh {
			fmt.Println("Got Spec Block Header and Signatures:", "number:", h.header.Number)
		}
	}()

	go func() {
		defer wg.Done()
		for b := range blockBodiesCh {
			fmt.Println("Got Spec Block Body parts:", "number:", b.id.Number)
		}
	}()
	wg.Wait()
}

type blockHeaderAndSigs struct {
	header *spec.BlockHeader
	sig    *spec.Signatures
}

func (s *syncService) genBlockHeadersAndSigs(ctx context.Context, it *spec.Iteration) (<-chan blockHeaderAndSigs, error) {
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
					specBodiesCh <- *curBlockBody
					curBlockBody = new(specBlockBody)
				}
			}
		}
	}()

	return specBodiesCh, nil
}
