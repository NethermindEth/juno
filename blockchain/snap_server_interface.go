package blockchain

import (
	"context"
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/service"
	"github.com/NethermindEth/juno/utils"
)

const MaxSnapshots = 128

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

func (b *Blockchain) GetDClasses(felts []*felt.Felt) ([]*core.DeclaredClass, error) {
	classes := make([]*core.DeclaredClass, len(felts))
	err := b.database.View(func(txn db.Transaction) error {
		state := core.NewState(txn)
		for i, f := range felts {
			d, err := state.Class(f)
			if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
				return err
			} else if errors.Is(err, db.ErrKeyNotFound) {
				classes[i] = nil
			} else {
				classes[i] = d
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

	stateR, srCloser, err := b.HeadState()
	if err != nil {
		return err
	}

	defer func() { _ = srCloser() }()

	state := stateR.(*core.State)
	contractsRoot, classRoot, err := state.StateAndClassRoot()
	if err != nil {
		return err
	}

	stateRoot, err := state.Root()
	if err != nil {
		return err
	}

	txn, closer, err := b.database.PersistedView()
	if err != nil {
		return err
	}

	dbsnap := snapshotRecord{
		stateRoot:     stateRoot,
		contractsRoot: contractsRoot,
		classRoot:     classRoot,
		blockHash:     headheader.Hash,
		txn:           txn,
		closer:        closer,
	}

	fmt.Printf("Snapshot %d %s %s\n", headheader.Number, headheader.GlobalStateRoot, stateRoot)

	// TODO: Reorgs
	b.snapshots = append(b.snapshots, &dbsnap)
	if len(b.snapshots) > MaxSnapshots {
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
		// ignore the errors here as it's most likely called on shutdown
		_ = snapshot.closer()
	}
}

type Blockchain_Closer struct {
	log utils.SimpleLogger
	bc  *Blockchain
}

var _ service.Service = (*Blockchain_Closer)(nil)

func NewBlockchainCloser(bc *Blockchain, log utils.SimpleLogger) *Blockchain_Closer {
	return &Blockchain_Closer{log, bc}
}

func (b *Blockchain_Closer) Run(ctx context.Context) error {
	b.log.Infow("Blockchain_Closer has started")

	<-ctx.Done()
	b.bc.Close()
	b.log.Infow("Blockchain_Closer has stopped")
	return nil
}