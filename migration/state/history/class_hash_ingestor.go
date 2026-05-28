package history

import (
	"context"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
	"github.com/NethermindEth/juno/migration/pipeline"
	"github.com/NethermindEth/juno/migration/semaphore"
)

type classHashIngestor struct {
	baseIngestor
}

var _ pipeline.State[*felt.Felt, task] = (*classHashIngestor)(nil)

func newClassHashIngestor(
	ctx context.Context,
	sem semaphore.ResourceSemaphore[db.Batch],
	database db.KeyValueReader,
) *classHashIngestor {
	return &classHashIngestor{baseIngestor: newBaseIngestor(ctx, sem, database)}
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
func (i *classHashIngestor) Run(index int, addr *felt.Felt, outputs chan<- task) error {
	t := &i.tasks[index]

	deprecatedPrefix := db.DeprecatedContractClassHashHistoryKey(addr)
	contract, err := state.GetContract(i.database, addr)
	if err != nil {
		return fmt.Errorf("class-hash: GetContract(%s): %w", addr, err)
	}

	depIt, err := i.database.NewIterator(deprecatedPrefix, true)
	if err != nil {
		return fmt.Errorf("class-hash: open deprecated iter(%s): %w", addr, err)
	}
	defer depIt.Close()

	if !depIt.First() {
		return i.writeDeployOnly(t, outputs, addr, contract.DeployedHeight, &contract.ClassHash)
	}
	return i.writeShiftedHistory(
		t, outputs, depIt, deprecatedPrefix, addr,
		contract.DeployedHeight, &contract.ClassHash,
	)
}

// writeDeployOnly handles the "no deprecated history" branch: write the
// deploy-time entry from contract.ClassHash, unless a previous run already
// wrote it.
func (i *classHashIngestor) writeDeployOnly(
	t *task,
	outputs chan<- task,
	addr *felt.Felt,
	deployHeight uint64,
	classHash *felt.Felt,
) error {
	deployKey := db.ContractClassHashHistoryAtBlockKey(addr, deployHeight)
	deployEntryExists, err := i.database.Has(deployKey)
	if err != nil {
		return fmt.Errorf("class-hash: Has(deploy entry): %w", err)
	}
	if deployEntryExists {
		return nil
	}
	if err := state.WriteClassHashHistory(t.batch, addr, deployHeight, classHash); err != nil {
		return err
	}
	t.completedAddrs++
	t.entryCount++
	return i.flush(t, outputs)
}

// writeShiftedHistory handles the "non-empty deprecated history" branch:
// writes the deploy entry from the first deprecated value, shifts each
// deprecated entry into the new layout using the next entry's pre-value
// (or contract.ClassHash for the last), and deletes the deprecated rows.
// depIt must be positioned at the first deprecated entry.
func (i *classHashIngestor) writeShiftedHistory(
	t *task,
	outputs chan<- task,
	depIt db.Iterator,
	prefix []byte,
	addr *felt.Felt,
	deployHeight uint64,
	headClassHash *felt.Felt,
) error {
	rawValue, err := depIt.Value()
	if err != nil {
		return fmt.Errorf("class-hash: read first value(%s): %w", addr, err)
	}
	deployClassHash := felt.FromBytes[felt.Felt](rawValue)
	if err := state.WriteClassHashHistory(t.batch, addr, deployHeight, &deployClassHash); err != nil {
		return err
	}
	t.entryCount++
	if err := i.flush(t, outputs); err != nil {
		return err
	}

	for {
		block, err := parseBlockKey(depIt.Key(), prefix)
		if err != nil {
			return fmt.Errorf("class-hash(%s): %w", addr, err)
		}
		hasNext := depIt.Next()
		historyValue := *headClassHash
		if hasNext {
			rawValue, err := depIt.Value()
			if err != nil {
				return fmt.Errorf("class-hash(%s): %w", addr, err)
			}
			historyValue = felt.FromBytes[felt.Felt](rawValue)
		}
		if err := state.WriteClassHashHistory(t.batch, addr, block, &historyValue); err != nil {
			return err
		}
		t.entryCount++
		if err := i.flush(t, outputs); err != nil {
			return err
		}
		if !hasNext {
			break
		}
	}

	if err := t.batch.DeleteRange(prefix, dbutils.UpperBound(prefix)); err != nil {
		return fmt.Errorf("class-hash: DeleteRange deprecated(%s): %w", addr, err)
	}
	t.completedAddrs++
	return nil
}
