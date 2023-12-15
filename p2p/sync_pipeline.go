package p2p

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/starknet"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/davecgh/go-spew/spew"
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

	headersAndSigsCh, err := s.genBlockHeadersAndSigs(ctx, s.createIterator(BlockRange{nextHeight, bootNodeHeight}))
	if err != nil {
		s.log.Errorw("Failed to get block headers parts", "err", err)
	}

	for h := range adaptBlockHeadersAndSigs(ctx, headersAndSigsCh) {
		spew.Dump(h)
	}
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

func adaptBlockHeadersAndSigs(ctx context.Context, headersAndSigsCh <-chan blockHeaderAndSigs) <-chan core.Header {
	headersCh := make(chan core.Header)
	go func() {
		defer close(headersCh)
		for headerAndSig := range headersAndSigsCh {
			select {
			case <-ctx.Done():
				return
			default:
				var header core.Header
				header = p2p2core.AdaptBlockHeader(headerAndSig.header)
				header.Signatures = utils.Map(headerAndSig.sig.GetSignatures(), p2p2core.AdaptSignature)
				headersCh <- header
			}
		}
	}()
	return headersCh
}
