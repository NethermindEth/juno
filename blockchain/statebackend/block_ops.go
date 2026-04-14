package statebackend

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

var ErrParentDoesNotMatchHead = errors.New("block's parent hash does not match head block hash")

func headsHeader(reader db.KeyValueReader) (*core.Header, error) {
	height, err := core.GetChainHeight(reader)
	if err != nil {
		return nil, err
	}
	return core.GetBlockHeaderByNumber(reader, height)
}

// verifyBlockSuccession checks that the block follows the current chain head.
func verifyBlockSuccession(reader db.KeyValueReader, block *core.Block) error {
	if err := core.CheckBlockVersion(block.ProtocolVersion); err != nil {
		return err
	}

	expectedBlockNumber := uint64(0)
	expectedParentHash := &felt.Zero

	h, err := headsHeader(reader)
	if err == nil {
		expectedBlockNumber = h.Number + 1
		expectedParentHash = h.Hash
	} else if !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	if expectedBlockNumber != block.Number {
		return fmt.Errorf("expected block #%d, got block #%d", expectedBlockNumber, block.Number)
	}
	if !block.ParentHash.Equal(expectedParentHash) {
		return ErrParentDoesNotMatchHead
	}

	return nil
}

// updateBlockHash computes block hash and commitments, mutates block and stateUpdate in place.
func updateBlockHash(
	block *core.Block,
	stateUpdate *core.StateUpdate,
	network *utils.Network,
) (*core.BlockCommitments, error) {
	blockHash, commitments, err := core.BlockHash(
		block,
		stateUpdate.StateDiff,
		network,
		block.SequencerAddress,
	)
	if err != nil {
		return nil, err
	}
	block.Hash = &blockHash
	stateUpdate.BlockHash = &blockHash
	return commitments, nil
}

// signBlock applies the signature to the block if a signing function is provided.
func signBlock(block *core.Block, stateUpdate *core.StateUpdate, sign utils.BlockSignFunc) error {
	commitment := stateUpdate.StateDiff.Commitment()
	sig, err := sign(block.Hash, &commitment)
	if err != nil {
		return err
	}

	block.Signatures = [][]*felt.Felt{sig}

	return nil
}

// updateStateRoots applies the state update and computes the new state root.
func updateStateRoots(
	state core.State,
	block *core.Block,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	oldRoot, err := state.Commitment(block.ProtocolVersion)
	if err != nil {
		return err
	}
	stateUpdate.OldRoot = &oldRoot

	if err := state.Update(block.Header, stateUpdate, newClasses, true); err != nil {
		return err
	}

	newRoot, err := state.Commitment(block.ProtocolVersion)
	if err != nil {
		return err
	}
	block.GlobalStateRoot = &newRoot
	stateUpdate.NewRoot = block.GlobalStateRoot
	return nil
}

// writeBlockContent writes block data to storage: header, transactions, state, commitments.
func writeBlockContent(
	reader db.KeyValueReader,
	writer db.Batch,
	block *core.Block,
	stateUpdate *core.StateUpdate,
	commitments *core.BlockCommitments,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	if err := core.WriteBlockHeader(writer, block.Header); err != nil {
		return err
	}

	if err := core.WriteTransactionsAndReceipts(
		writer,
		block.Number,
		block.Transactions,
		block.Receipts,
	); err != nil {
		return err
	}

	if err := core.WriteStateUpdateByBlockNum(writer, block.Number, stateUpdate); err != nil {
		return err
	}

	if err := core.WriteBlockCommitment(writer, block.Number, commitments); err != nil {
		return err
	}

	if err := core.WriteL1HandlerMsgHashes(writer, block.Transactions); err != nil {
		return err
	}

	if err := storeCasmHashMetadata(
		reader, writer, block.Number, block.ProtocolVersion, stateUpdate, newClasses,
	); err != nil {
		return err
	}

	return core.WriteChainHeight(writer, block.Number)
}

// deleteBlockContent removes all data for a block:
// headers, transactions, state updates, chain height, and CASM hash metadata.
func deleteBlockContent(
	reader db.KeyValueReader,
	writer db.Batch,
	stateUpdate *core.StateUpdate,
	blockNumber uint64,
) error {
	header, err := core.GetBlockHeaderByNumber(reader, blockNumber)
	if err != nil {
		return err
	}

	if err = revertCasmHashMetadata(reader, writer, stateUpdate); err != nil {
		return err
	}

	genesisBlock := blockNumber == 0

	for _, key := range [][]byte{
		db.BlockHeaderByNumberKey(header.Number),
		db.BlockHeaderNumbersByHashKey(header.Hash),
		db.BlockCommitmentsKey(header.Number),
	} {
		if err = writer.Delete(key); err != nil {
			return err
		}
	}

	err = core.DeleteTransactionsAndReceipts(reader, writer, blockNumber)
	if err != nil {
		return err
	}

	if err := core.DeleteStateUpdateByBlockNum(writer, blockNumber); err != nil {
		return err
	}

	if genesisBlock {
		return core.DeleteChainHeight(writer)
	}

	return core.WriteChainHeight(writer, blockNumber-1)
}
