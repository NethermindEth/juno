package history

import (
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
	"github.com/NethermindEth/juno/migration/state/newstate/internal/common"
)

type classHashIngestor struct {
	common.BaseIngestor
	scratches []historyScratch
}

var _ pipeline.State[felt.Address, common.Task] = (*classHashIngestor)(nil)

func newClassHashIngestor(
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) *classHashIngestor {
	return &classHashIngestor{
		BaseIngestor: common.NewBaseIngestor(sem, database),
		scratches:    make([]historyScratch, common.IngestorCount),
	}
}

// Run migrates the class-hash history of a single contract.
//
// Legend: Bₙ = block at which the n-th class-hash *replacement* happened.
// Vₙ = the class hash active *after* Bₙ; V₀ is the deploy-time hash. The
// deprecated layout writes nothing at deploy: each entry is written only
// on a *Replace*, and the value stored is the hash that was active before
// that replace. So deprecated[B₁] = V₀ even though no replace happened at
// deploy_h itself. The new layout adds an explicit deploy entry and shifts
// everything else by one slot:
//
//	block    │ deprecated     │ new
//	─────────┼────────────────┼──────
//	deploy_h │  —             │ V₀     ← inserted from first deprecated entry
//	  B₁     │  V₀            │ V₁
//	  B₂     │  V₁            │ V₂
//	  B₃     │  V₂            │ V₃
//	─────────┼────────────────┼──────
//	  > B₃   │  contract      │ V₃ (last entry — self-contained)
//	            .ClassHash      ← deprecated must reach into the Contract
//	                              record for any block past the last replace
//
// If the deprecated history is empty (no replaces ever), the single deploy
// entry is written with contract.ClassHash directly. Deprecated rows are
// deleted at the end of the run. Resume-safe: empty-deprecated + existing
// deploy entry → no-op.
func (i *classHashIngestor) Run(index int, addr felt.Address, outputs chan<- common.Task) error {
	addrFelt := (*felt.Felt)(&addr)
	task := &i.Tasks[index]
	scratch := &i.scratches[index]

	contractKey := fillAddressKey(scratch.contractKey[:], db.Contract, &addr)
	headClassHash, deployHeight, err := readHeadClassHash(i.Database, contractKey)
	if err != nil {
		return fmt.Errorf("reading contract record of %s: %w", addrFelt.String(), err)
	}

	prefix := fillAddressKey(scratch.deprecatedPrefix[:], db.DeprecatedContractClassHashHistory, &addr)
	depIt, err := i.Database.NewIterator(prefix, true)
	if err != nil {
		return fmt.Errorf("opening deprecated class hash history of %s: %w", addrFelt.String(), err)
	}
	defer depIt.Close()

	// key trails one deprecated row behind the iterator. It starts at the
	// deploy height: unlike nonce and storage, the first deprecated value is
	// the deploy-time class hash and must be kept.
	key := fillBlockHistoryKey(
		scratch.key[:blockKeyLen], db.ContractClassHashHistory, &addr, deployHeight[:],
	)

	if !depIt.First() {
		return i.writeDeployOnly(task, outputs, key, headClassHash[:])
	}

	for valid := true; valid; valid = depIt.Next() {
		// This row's stored pre-value is the class hash in effect after the
		// block key names, and needs no decoding to move.
		value, valueErr := depIt.UncopiedValue()
		if valueErr != nil {
			return fmt.Errorf("reading deprecated class hash of %s: %w", addrFelt.String(), valueErr)
		}
		if err := task.Batch.Put(key, value); err != nil {
			return err
		}

		if err := fillHistoryKeyFrom(key, db.ContractClassHashHistory, depIt.UncopiedKey()); err != nil {
			return fmt.Errorf("%s: %w", addrFelt.String(), err)
		}
		task.EntryCount++
		if err := i.Flush(task, outputs); err != nil {
			return err
		}
	}

	// The last entry's value is the head class hash, which the history never stored.
	if err := task.Batch.Put(key, headClassHash[:]); err != nil {
		return err
	}
	task.EntryCount++
	if err := i.Flush(task, outputs); err != nil {
		return err
	}

	task.CompletedAddrs++
	return nil
}

// writeDeployOnly handles a contract that was never re-declared: the single
// deploy entry carries the head class hash. key must already be set to the
// deploy height. Re-run safe, since a previous run may have written it.
func (i *classHashIngestor) writeDeployOnly(
	task *common.Task,
	outputs chan<- common.Task,
	key []byte,
	headClassHash []byte,
) error {
	exists, err := i.Database.Has(key)
	if err != nil {
		return fmt.Errorf("checking deploy class hash entry: %w", err)
	}
	if exists {
		return nil
	}
	if err := task.Batch.Put(key, headClassHash); err != nil {
		return err
	}
	task.CompletedAddrs++
	task.EntryCount++
	return i.Flush(task, outputs)
}
