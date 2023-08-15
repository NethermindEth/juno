package core2p2p_test

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestAdaptHeader(t *testing.T) {
	block := core.Block{
		Header: &core.Header{
			Hash:             utils.HexToFelt(t, "0x1"),
			ParentHash:       utils.HexToFelt(t, "0x2"),
			Number:           3,
			GlobalStateRoot:  utils.HexToFelt(t, "0x4"),
			SequencerAddress: utils.HexToFelt(t, "0x5"),
			TransactionCount: 6,
			EventCount:       7,
			Timestamp:        8,
			ProtocolVersion:  "v1.0.0",
		},
	}
	commitments := core.BlockCommitments{
		TransactionCommitment: utils.HexToFelt(t, "0x10"),
		EventCommitment:       utils.HexToFelt(t, "0x11"),
	}
	p2pHeader := core2p2p.AdaptHeader(&block, &commitments, utils.MAINNET.ChainID())
	assert.EqualValues(t, utils.MAINNET.ChainID().Marshal(), p2pHeader.ChainId.Id)
	assert.EqualValues(t, block.EventCount, p2pHeader.Events.NLeaves)
	assert.EqualValues(t, commitments.EventCommitment.Marshal(), p2pHeader.Events.Root.Elements)
	assert.EqualValues(t, block.ParentHash.Marshal(), p2pHeader.ParentBlock.Hash.Elements)
	assert.Nil(t, p2pHeader.ProofFact)
	assert.EqualValues(t, 0, p2pHeader.ProtocolVersion)
	assert.Nil(t, p2pHeader.Receipts)
	assert.EqualValues(t, block.SequencerAddress.Marshal(), p2pHeader.SequencerAddress.Elements)
	assert.EqualValues(t, block.GlobalStateRoot.Marshal(), p2pHeader.State.Elements)
	assert.Nil(t, p2pHeader.StateDiffs)
	assert.EqualValues(t, block.Timestamp, p2pHeader.Time.GetSeconds())
}
