package blockchain

import (
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type snapshotRecord struct {
	stateRoot     *felt.Felt
	contractsRoot *felt.Felt
	classRoot     *felt.Felt
	blockHash     *felt.Felt
	txn           db.Transaction
	closer        func() error
}

var ErrMissingSnapshot = errors.New("missing snapshot")

func (b *Blockchain) GetStateForStateRoot(stateRoot *felt.Felt) (*core.State, error) {
	snapshot, err := b.findSnapshotMatching(func(record *snapshotRecord) bool {
		return record.stateRoot.Equal(stateRoot)
	})

	if err != nil {
		return nil, err
	}

	s := core.NewState(snapshot.txn)

	return s, nil
}

func (b *Blockchain) findSnapshotMatching(filter func(record *snapshotRecord) bool) (*snapshotRecord, error) {
	var snapshot *snapshotRecord
	for _, record := range b.snapshots {
		if filter(record) {
			snapshot = record
			break
		}
	}

	if snapshot == nil {
		return nil, ErrMissingSnapshot
	}

	return snapshot, nil
}

func (b *Blockchain) GetClasses(felts []*felt.Felt) ([]core.Class, error) {
	classes := make([]core.Class, len(felts))
	err := b.database.View(func(txn db.Transaction) error {
		state := core.NewState(txn)
		for i, f := range felts {
			d, err := state.Class(f)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			} else if errors.Is(err, db.ErrKeyNotFound) {
				classes[i] = nil
			} else {
				classes[i] = d.Class
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return classes, nil
}

func (b *Blockchain) seedSnapshot() error {
	headheader, err := b.HeadsHeader()
	if err != nil {
		return err
	}

	state, scloser, err := b.HeadState()
	if err != nil {
		return err
	}

	defer scloser()

	stateS := state.(*core.State)
	contractsRoot, theclassroot, err := stateS.StateAndClassRoot()
	if err != nil {
		return err
	}

	thestateroot, err := stateS.Root()
	if err != nil {
		return err
	}

	txn, closer, err := b.database.PersistedView()
	if err != nil {
		return err
	}

	dbsnap := snapshotRecord{
		stateRoot:     thestateroot,
		contractsRoot: contractsRoot,
		classRoot:     theclassroot,
		blockHash:     headheader.Hash,
		txn:           txn,
		closer:        closer,
	}

	fmt.Printf("Snapshot %s\n", thestateroot)

	// TODO: Reorgs
	b.snapshots = append(b.snapshots, &dbsnap)
	if len(b.snapshots) > 128 {
		toremove := b.snapshots[0]
		err = toremove.closer()
		if err != nil {
			return err
		}

		// TODO: I think internally, it keep the old array.
		// maybe the append copy it to a new array, who knows...
		b.snapshots = b.snapshots[1:]
	}

	return nil
}

func (b *Blockchain) Close() {
	for _, snapshot := range b.snapshots {
		snapshot.closer()
	}
}
