//nolint:gocritic //commentedOutCode:
package deprecatedmigration

// The commented code represents the original code of the `deprecatedStateBackend` type
// not used for this implementation.

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/blocktransactions/txlayout"
	"github.com/NethermindEth/juno/utils"
)

// testStateBackend is a copy of the [statebackend.deprecatedStateBackend] type
// that stores the state using the old layout.
//
// Juno now uses a new layout to store transactions and receipts on the database,
// but there are some old migrations that use the old layout. These migrations
// can not be tested using the current state code, since it uses the new layout.
// [testStateBackend] was created specifically to solve this problem.
type testStateBackend struct {
	database db.KeyValueStore
	network  *utils.Network
}

// NewTestState creates a new test state that stores the data using the old layout.
func NewTestState(
	database db.KeyValueStore,
	network *utils.Network,
) testStateBackend {
	return testStateBackend{
		database: database,
		network:  network,
	}
}

func (b *testStateBackend) Store(
	block *core.Block,
	blockCommitments *core.BlockCommitments,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	//nolint:staticcheck,nolintlint // used by old state
	err := b.database.Update(func(txn db.IndexedBatch) error {
		if err := verifyBlockSuccession(txn, block); err != nil {
			return err
		}
		err := core.NewDeprecatedState(txn).Update(block.Header, stateUpdate, newClasses, false)
		if err != nil {
			return err
		}

		return writeBlockContent(
			// b.database,
			txn,
			block,
			stateUpdate,
			blockCommitments,
			newClasses,
			// b.transactionLayout,
		)
	})
	if err != nil {
		return err
	}

	// return b.runningFilter.Insert(
	// 	block.EventsBloom,
	// 	block.Number,
	// )
	return nil
}

var ErrParentDoesNotMatchHead = errors.New("block's parent hash does not match head block hash")

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

func headsHeader(reader db.KeyValueReader) (*core.Header, error) {
	height, err := core.GetChainHeight(reader)
	if err != nil {
		return nil, err
	}
	return core.GetBlockHeaderByNumber(reader, height)
}

func writeBlockContent(
	// reader db.KeyValueReader,
	writer db.Batch,
	block *core.Block,
	stateUpdate *core.StateUpdate,
	commitments *core.BlockCommitments,
	newClasses map[felt.Felt]core.ClassDefinition,
	// txLayout core.TransactionLayout,
) error {
	if err := core.WriteBlockHeader(writer, block.Header); err != nil {
		return err
	}

	if err := txlayout.TransactionLayoutPerTx.WriteTransactionsAndReceipts(
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
		// reader,
		writer,
		block.Number,
		// block.ProtocolVersion,
		stateUpdate,
		newClasses,
	); err != nil {
		return err
	}

	return core.WriteChainHeight(writer, block.Number)
}

func storeCasmHashMetadata(
	// reader db.KeyValueReader,
	writer db.KeyValueWriter,
	blockNumber uint64,
	// protocolVersion string,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	// ver, err := core.ParseBlockVersion(protocolVersion)
	// if err != nil {
	// 	return err
	// }

	// isV2Protocol := ver.GreaterThanEqual(core.Ver0_14_1)

	// if isV2Protocol {
	// 	return storeCasmHashMetadataV2(reader, writer, blockNumber, stateUpdate)
	// }

	return storeCasmHashMetadataV1(writer, blockNumber, stateUpdate, newClasses)
}

func storeCasmHashMetadataV1(
	writer db.KeyValueWriter,
	blockNumber uint64,
	stateUpdate *core.StateUpdate,
	newClasses map[felt.Felt]core.ClassDefinition,
) error {
	for sierraClassHash, casmHash := range stateUpdate.StateDiff.DeclaredV1Classes {
		casmHashV1 := (*felt.CasmClassHash)(casmHash)

		classDef, ok := newClasses[sierraClassHash]
		if !ok {
			return fmt.Errorf("class %s not available in newClasses at block %d",
				sierraClassHash.String(),
				blockNumber,
			)
		}

		sierraClass, ok := classDef.(*core.SierraClass)
		if !ok {
			return fmt.Errorf("class %s must be a SierraClass at block %d",
				sierraClassHash.String(),
				blockNumber,
			)
		}

		v2Hash := sierraClass.Compiled.Hash(core.HashVersionV2)
		casmHashV2 := felt.CasmClassHash(v2Hash)

		metadata := core.NewCasmHashMetadataDeclaredV1(blockNumber, casmHashV1, &casmHashV2)
		err := core.WriteClassCasmHashMetadata(
			writer,
			(*felt.SierraClassHash)(&sierraClassHash),
			&metadata,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
