package proposer

import (
	"iter"
	"slices"

	"github.com/NethermindEth/juno/adapters/consensus2p2p"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

const txBatchSize = 64 // TODO: make this configurable

type dispatcher[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	adapter     ProposerAdapter[V, H, A]
	proposal    *types.Proposal[V, H, A]
	buildResult *builder.BuildResult
	streamID    string
	seqNo       uint64
}

func newProposerDispatcher[V types.Hashable[H], H types.Hash, A types.Addr](
	adapter ProposerAdapter[V, H, A],
	proposal *types.Proposal[V, H, A],
	buildResult *builder.BuildResult,
) (dispatcher[V, H, A], error) {
	streamIDStruct := &consensus.ConsensusStreamId{
		BlockNumber: uint64(proposal.Height),
		Round:       uint32(proposal.Round),
		Nonce:       0,
	}

	streamID, err := proto.Marshal(streamIDStruct)
	if err != nil {
		return dispatcher[V, H, A]{}, err // TODO: log error
	}

	return dispatcher[V, H, A]{
		adapter:     adapter,
		proposal:    proposal,
		buildResult: buildResult,
		streamID:    string(streamID),
		seqNo:       0,
	}, nil
}

func (d *dispatcher[V, H, A]) run() iter.Seq2[*consensus.StreamMessage, error] {
	return func(yield func(*consensus.StreamMessage, error) bool) {
		if !yield(d.fromProposalInit()) {
			return
		}

		if !yield(d.fromBlockInfo()) {
			return
		}

		txStream, err := d.adapter.ProposalTransactions(d.buildResult)
		if err != nil {
			yield(nil, err)
			return
		}

		for txBatch := range slices.Chunk(txStream, txBatchSize) {
			if !yield(d.fromTransactions(txBatch)) {
				return
			}
		}

		if !yield(d.fromProposalCommitment()) {
			return
		}

		if !yield(d.fromProposalFin()) {
			return
		}

		if !yield(d.fromFin(), nil) {
			return
		}
	}
}

func (d *dispatcher[V, H, A]) fromProposalInit() (*consensus.StreamMessage, error) {
	proposalInit, err := d.adapter.ProposalInit(d.proposal)
	if err != nil {
		return nil, err // TODO: log error
	}

	p2pProposalInit := consensus2p2p.AdaptProposalInit(&proposalInit)

	return d.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Init{
			Init: &p2pProposalInit,
		},
	})
}

func (d *dispatcher[V, H, A]) fromBlockInfo() (*consensus.StreamMessage, error) {
	blockInfo, err := d.adapter.ProposalBlockInfo(d.buildResult)
	if err != nil {
		return nil, err // TODO: log error
	}

	p2pBlockInfo := consensus2p2p.AdaptBlockInfo(&blockInfo)

	return d.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_BlockInfo{
			BlockInfo: &p2pBlockInfo,
		},
	})
}

func (d *dispatcher[V, H, A]) fromTransactions(txs []types.Transaction) (*consensus.StreamMessage, error) {
	p2pTxBatch, err := consensus2p2p.AdaptProposalTransaction(txs)
	if err != nil {
		return nil, err // TODO: log error
	}

	return d.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Transactions{
			Transactions: &p2pTxBatch,
		},
	})
}

func (d *dispatcher[V, H, A]) fromProposalCommitment() (*consensus.StreamMessage, error) {
	commitment, err := d.adapter.ProposalCommitment(d.buildResult)
	if err != nil {
		return nil, err // TODO: log error
	}

	p2pCommitment := consensus2p2p.AdaptProposalCommitment(&commitment)

	return d.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Commitment{
			Commitment: &p2pCommitment,
		},
	})
}

func (d *dispatcher[V, H, A]) fromProposalFin() (*consensus.StreamMessage, error) {
	fin, err := d.adapter.ProposalFin(d.proposal)
	if err != nil {
		return nil, err // TODO: log error
	}

	p2pFin := consensus2p2p.AdaptProposalFin(&fin)

	return d.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Fin{
			Fin: &p2pFin,
		},
	})
}

func (d *dispatcher[V, H, A]) sendProposalPart(proposal *consensus.ProposalPart) (*consensus.StreamMessage, error) {
	proposalBytes, err := proto.Marshal(proposal)
	if err != nil {
		return nil, err // TODO: log error
	}

	return &consensus.StreamMessage{
		Message: &consensus.StreamMessage_Content{
			Content: proposalBytes,
		},
		StreamId:       []byte(d.streamID),
		SequenceNumber: d.next(),
	}, nil
}

func (d *dispatcher[V, H, A]) fromFin() *consensus.StreamMessage {
	return &consensus.StreamMessage{
		Message: &consensus.StreamMessage_Fin{
			Fin: &common.Fin{},
		},
		StreamId:       []byte(d.streamID),
		SequenceNumber: d.next(),
	}
}

func (d *dispatcher[V, H, A]) next() uint64 {
	seqNo := d.seqNo
	d.seqNo++
	return seqNo
}
