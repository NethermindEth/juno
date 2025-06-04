package proposer

import (
	"iter"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"google.golang.org/protobuf/proto"
)

const txBatchSize = 64 // TODO: make this configurable

type seqNoCounter[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	streamID string
	seqNo    uint64
}

// ToProposalStream converts a proposal to a stream of messages.
func ToProposalStream[V types.Hashable[H], H types.Hash, A types.Addr](
	adapter ProposerAdapter[V, H, A],
	proposal types.Proposal[V, H, A],
) (iter.Seq2[*consensus.StreamMessage, error], error) {
	streamID, err := buildStreamID(proposal)
	if err != nil {
		return nil, err // TODO: log error
	}

	return func(yield func(*consensus.StreamMessage, error) bool) {
		seqNoCounter := seqNoCounter[V, H, A]{
			streamID: streamID,
		}

		if !yield(seqNoCounter.fromProposalInit(adapter, &proposal)) {
			return
		}

		if !yield(seqNoCounter.fromBlockInfo(adapter, &proposal)) {
			return
		}

		txStream, err := adapter.ProposalTransactions(proposal)
		if err != nil {
			yield(nil, err)
			return
		}

		txBatch := make([]*consensus.ConsensusTransaction, 0, txBatchSize)
		for tx := range txStream {
			txBatch = append(txBatch, tx)
			if len(txBatch) == txBatchSize {
				if !yield(seqNoCounter.fromTransactions(txBatch)) {
					return
				}
				txBatch = txBatch[:0]
			}
		}

		if len(txBatch) > 0 {
			if !yield(seqNoCounter.fromTransactions(txBatch)) {
				return
			}
		}

		if !yield(seqNoCounter.fromProposalCommitment(adapter, &proposal)) {
			return
		}

		if !yield(seqNoCounter.fromProposalFin(adapter, &proposal)) {
			return
		}

		if !yield(seqNoCounter.fromFin(), nil) {
			return
		}
	}, nil
}

func buildStreamID[V types.Hashable[H], H types.Hash, A types.Addr](proposal types.Proposal[V, H, A]) (string, error) {
	streamIDStruct := &consensus.ConsensusStreamId{
		BlockNumber: uint64(proposal.Height),
		Round:       uint32(proposal.Round),
		Nonce:       0,
	}

	streamID, err := proto.Marshal(streamIDStruct)
	if err != nil {
		return "", err // TODO: log error
	}

	return string(streamID), nil
}

func (s *seqNoCounter[V, H, A]) fromProposalInit(
	adapter ProposerAdapter[V, H, A],
	proposal *types.Proposal[V, H, A],
) (*consensus.StreamMessage, error) {
	proposalInit, err := adapter.ProposalInit(*proposal)
	if err != nil {
		return nil, err // TODO: log error
	}
	return s.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Init{
			Init: proposalInit,
		},
	})
}

func (s *seqNoCounter[V, H, A]) fromBlockInfo(
	adapter ProposerAdapter[V, H, A],
	proposal *types.Proposal[V, H, A],
) (*consensus.StreamMessage, error) {
	blockInfo, err := adapter.ProposalBlockInfo(*proposal)
	if err != nil {
		return nil, err // TODO: log error
	}
	return s.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_BlockInfo{
			BlockInfo: blockInfo,
		},
	})
}

func (s *seqNoCounter[V, H, A]) fromTransactions(
	txs []*consensus.ConsensusTransaction,
) (*consensus.StreamMessage, error) {
	return s.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Transactions{
			Transactions: &consensus.TransactionBatch{
				Transactions: txs,
			},
		},
	})
}

func (s *seqNoCounter[V, H, A]) fromProposalCommitment(
	adapter ProposerAdapter[V, H, A],
	proposal *types.Proposal[V, H, A],
) (*consensus.StreamMessage, error) {
	commitment, err := adapter.ProposalCommitment(*proposal)
	if err != nil {
		return nil, err // TODO: log error
	}
	return s.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Commitment{
			Commitment: commitment,
		},
	})
}

func (s *seqNoCounter[V, H, A]) fromProposalFin(
	adapter ProposerAdapter[V, H, A],
	proposal *types.Proposal[V, H, A],
) (*consensus.StreamMessage, error) {
	fin, err := adapter.ProposalFin(*proposal)
	if err != nil {
		return nil, err // TODO: log error
	}
	return s.sendProposalPart(&consensus.ProposalPart{
		Messages: &consensus.ProposalPart_Fin{
			Fin: fin,
		},
	})
}

func (s *seqNoCounter[V, H, A]) sendProposalPart(proposal *consensus.ProposalPart) (*consensus.StreamMessage, error) {
	proposalBytes, err := proto.Marshal(proposal)
	if err != nil {
		return nil, err // TODO: log error
	}

	return &consensus.StreamMessage{
		Message: &consensus.StreamMessage_Content{
			Content: proposalBytes,
		},
		StreamId:       []byte(s.streamID),
		SequenceNumber: s.next(),
	}, nil
}

func (s *seqNoCounter[V, H, A]) fromFin() *consensus.StreamMessage {
	return &consensus.StreamMessage{
		Message: &consensus.StreamMessage_Fin{
			Fin: &common.Fin{},
		},
		StreamId:       []byte(s.streamID),
		SequenceNumber: s.next(),
	}
}

func (s *seqNoCounter[V, H, A]) next() uint64 {
	seqNo := s.seqNo
	s.seqNo++
	return seqNo
}
