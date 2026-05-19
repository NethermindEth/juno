package statehistory

import (
	"bytes"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
)

func TransformClassHashHistory(
	database db.KeyValueReader,
	t *task,
	addr felt.Address,
	flush FlushBatchFn,
) error {
	addrFelt := felt.Felt(addr)
	deprecatedPrefix := db.DeprecatedContractClassHashHistoryKey(&addrFelt)

	contract, err := state.GetContract(database, &addrFelt)
	if err != nil {
		return fmt.Errorf("class-hash: GetContract(%s): %w", &addrFelt, err)
	}

	deployKey := db.ContractClassHashHistoryAtBlockKey(&addrFelt, contract.DeployedHeight)
	deployEntryExists, err := database.Has(deployKey)
	if err != nil {
		return fmt.Errorf("class-hash: Has(deploy entry): %w", err)
	}

	depIt, err := database.NewIterator(deprecatedPrefix, true)
	if err != nil {
		return fmt.Errorf("class-hash: open deprecated iter(%s): %w", &addrFelt, err)
	}
	defer depIt.Close()

	if !depIt.First() {
		if deployEntryExists {
			return nil
		}
		if err := state.WriteClassHashHistory(
			t.batch,
			&addrFelt,
			contract.DeployedHeight,
			&contract.ClassHash,
		); err != nil {
			return err
		}
		t.addrCount++
		t.entryCount++
		flush(t)
		return nil
	}

	rawValue, err := depIt.Value()
	if err != nil {
		return fmt.Errorf("class-hash: read first value(%s): %w", &addrFelt, err)
	}
	deployClassHash := felt.FromBytes[felt.Felt](rawValue)
	if err := state.WriteClassHashHistory(
		t.batch,
		&addrFelt,
		contract.DeployedHeight,
		&deployClassHash,
	); err != nil {
		return err
	}
	t.entryCount++
	flush(t)

	if _, err := shiftUpHistoryEntries(
		depIt,
		deprecatedPrefix,
		&addrFelt,
		contract.ClassHash,
		t,
		flush,
		state.WriteClassHashHistory,
	); err != nil {
		return fmt.Errorf("class-hash(%s): %w", &addrFelt, err)
	}

	if err := t.batch.DeleteRange(deprecatedPrefix, dbutils.UpperBound(deprecatedPrefix)); err != nil {
		return fmt.Errorf("class-hash: DeleteRange deprecated(%s): %w", &addrFelt, err)
	}
	t.addrCount++

	return nil
}

func TransformNonceHistory(
	database db.KeyValueReader,
	t *task,
	addr felt.Address,
	flush FlushBatchFn,
) error {
	addrFelt := felt.Felt(addr)
	deprecatedPrefix := db.DeprecatedContractNonceHistoryKey(&addrFelt)

	depIt, err := database.NewIterator(deprecatedPrefix, true)
	if err != nil {
		return fmt.Errorf("nonce: open deprecated iter(%s): %w", &addrFelt, err)
	}
	defer depIt.Close()
	if !depIt.First() {
		return nil
	}

	contract, err := state.GetContract(database, &addrFelt)
	if err != nil {
		return fmt.Errorf("nonce: GetContract(%s): %w", &addrFelt, err)
	}

	if _, err := shiftUpHistoryEntries(
		depIt,
		deprecatedPrefix,
		&addrFelt,
		contract.Nonce,
		t,
		flush,
		state.WriteNonceHistory,
	); err != nil {
		return fmt.Errorf("nonce(%s): %w", &addrFelt, err)
	}

	if err := t.batch.DeleteRange(deprecatedPrefix, dbutils.UpperBound(deprecatedPrefix)); err != nil {
		return fmt.Errorf("nonce: DeleteRange deprecated(%s): %w", &addrFelt, err)
	}
	t.addrCount++

	return nil
}

func TransformStorageHistory(
	database db.KeyValueReader,
	t *task,
	addr felt.Address,
	flush FlushBatchFn) error {
	addrFelt := felt.Felt(addr)
	addrBytes := addrFelt.Marshal()
	deprecatedPrefix := db.DeprecatedContractStorageHistory.Key(addrBytes)

	deprecatedHistoryIt, err := database.NewIterator(deprecatedPrefix, true)
	if err != nil {
		return fmt.Errorf("storage: open deprecated iter(%s): %w", &addrFelt, err)
	}
	defer deprecatedHistoryIt.Close()
	if !deprecatedHistoryIt.First() {
		return nil
	}

	leafPrefix := db.ContractStorage.Key(addrBytes)
	leafPrefix = append(leafPrefix, 251)

	headStorageTrieIt, err := database.NewIterator(leafPrefix, true)
	if err != nil {
		return fmt.Errorf("storage: open leaf iter(%s): %w", &addrFelt, err)
	}
	defer headStorageTrieIt.Close()
	leafValid := headStorageTrieIt.First()

	for {
		slot, block, err := parseStorageKey(deprecatedHistoryIt.Key(), deprecatedPrefix)
		if err != nil {
			return fmt.Errorf("storage: parse key(%s): %w", &addrFelt, err)
		}

		hasNext := deprecatedHistoryIt.Next()
		var successorSlot, successorValue felt.Felt
		if hasNext {
			s, _, err := parseStorageKey(deprecatedHistoryIt.Key(), deprecatedPrefix)
			if err != nil {
				return fmt.Errorf("storage: parse successor key(%s): %w", &addrFelt, err)
			}
			successorSlot = s
			rawValue, err := deprecatedHistoryIt.Value()
			if err != nil {
				return fmt.Errorf("storage: read successor value(%s, slot=%s): %w", &addrFelt, &s, err)
			}
			successorValue = felt.FromBytes[felt.Felt](rawValue)
		}

		var historyValue felt.Felt
		switch {
		case hasNext && successorSlot == slot:
			historyValue = successorValue
		case leafValid:
			leafSlot := headStorageTrieIt.Key()[len(leafPrefix):]
			if bytes.Equal(leafSlot, slot.Marshal()) {
				raw, err := headStorageTrieIt.Value()
				if err != nil {
					return fmt.Errorf("storage: leaf(%s, slot=%s): %w", &addrFelt, &slot, err)
				}
				var node trie.Node
				if err := node.UnmarshalBinary(raw); err != nil {
					return fmt.Errorf("storage: decode leaf(%s, slot=%s): %w", &addrFelt, &slot, err)
				}
				historyValue = *node.Value
				leafValid = headStorageTrieIt.Next()
			}
		}

		if err := state.WriteStorageHistory(t.batch, &addrFelt, &slot, block, &historyValue); err != nil {
			return err
		}
		t.entryCount++
		flush(t)

		if !hasNext {
			break
		}
	}

	if err := t.batch.DeleteRange(deprecatedPrefix, dbutils.UpperBound(deprecatedPrefix)); err != nil {
		return fmt.Errorf("storage: DeleteRange deprecated(%s): %w", &addrFelt, err)
	}
	t.addrCount++

	return nil
}

func shiftUpHistoryEntries(
	depIt db.Iterator,
	deprecatedPrefix []byte,
	addr *felt.Felt,
	headValue felt.Felt,
	t *task,
	flush FlushBatchFn,
	write func(db.KeyValueWriter, *felt.Felt, uint64, *felt.Felt) error,
) (int, error) {
	entryCount := 0
	for {
		block, err := parseBlockKey(depIt.Key(), deprecatedPrefix)
		if err != nil {
			return 0, err
		}
		hasNext := depIt.Next()
		historyValue := headValue
		if hasNext {
			rawValue, err := depIt.Value()
			if err != nil {
				return 0, err
			}
			historyValue = felt.FromBytes[felt.Felt](rawValue)
		}
		if err := write(t.batch, addr, block, &historyValue); err != nil {
			return 0, err
		}
		entryCount++
		t.entryCount++
		flush(t)
		if !hasNext {
			return entryCount, nil
		}
	}
}
