package sync

import (
	"context"
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/starknetdata"
)

type CommittedBlock struct {
	Block       *core.Block
	StateUpdate *core.StateUpdate
	NewClasses  map[felt.Felt]core.ClassDefinition
	Persisted   chan error // This is used to signal whether the block was persisted successfully
}

type DataSource interface {
	BlockByNumber(ctx context.Context, blockNumber uint64) (CommittedBlock, error)
	BlockHeaderLatest(ctx context.Context) (*core.Header, error)
}

type feederGatewayDataSource struct {
	blockchain   *blockchain.Blockchain
	starknetData starknetdata.StarknetData
}

func NewFeederGatewayDataSource(blockchain *blockchain.Blockchain, starknetData starknetdata.StarknetData) DataSource {
	return &feederGatewayDataSource{
		blockchain:   blockchain,
		starknetData: starknetData,
	}
}

func (f *feederGatewayDataSource) BlockByNumber(ctx context.Context, blockNumber uint64) (CommittedBlock, error) {
	stateUpdate, block, err := f.starknetData.StateUpdateWithBlock(ctx, blockNumber)
	if err != nil {
		return CommittedBlock{}, err
	}

	missingClassHashes, err := MissingClassHashes(f.blockchain, stateUpdate.StateDiff)
	if err != nil {
		return CommittedBlock{}, err
	}

	newClasses := make(map[felt.Felt]core.ClassDefinition, len(missingClassHashes))
	for _, classHash := range missingClassHashes {
		class, err := f.starknetData.Class(ctx, classHash)
		if err != nil {
			return CommittedBlock{}, err
		}

		newClasses[*classHash] = class
	}

	return CommittedBlock{
		Block:       block,
		StateUpdate: stateUpdate,
		NewClasses:  newClasses,
		Persisted:   make(chan error, 1),
	}, nil
}

func (f *feederGatewayDataSource) BlockHeaderLatest(ctx context.Context) (*core.Header, error) {
	header, err := f.starknetData.BlockHeaderLatest(ctx)
	if err != nil {
		return nil, err
	}

	return &header, nil
}

func MissingClassHashes(
	chain *blockchain.Blockchain,
	stateDiff *core.StateDiff,
) ([]*felt.Felt, error) {
	introduced := introducedClassHashes(stateDiff)

	state, closer, err := chain.HeadState()
	if errors.Is(err, db.ErrKeyNotFound) { // empty DB: nothing is known yet
		return introduced, nil
	}
	if err != nil {
		return nil, err
	}

	missing := make([]*felt.Felt, 0, len(introduced))
	for _, classHash := range introduced {
		_, err := state.Class(classHash)
		switch {
		case errors.Is(err, db.ErrKeyNotFound):
			missing = append(missing, classHash)
		case err != nil:
			return nil, errors.Join(err, closer())
		}
	}

	return missing, closer()
}

func introducedClassHashes(stateDiff *core.StateDiff) []*felt.Felt {
	seen := make(map[felt.Felt]struct{})
	hashes := make([]*felt.Felt, 0)
	add := func(classHash *felt.Felt) {
		if _, ok := seen[*classHash]; ok {
			return
		}

		seen[*classHash] = struct{}{}
		hashes = append(hashes, classHash)
	}

	for _, classHash := range stateDiff.DeployedContracts {
		add(classHash)
	}

	for _, classHash := range stateDiff.DeclaredV0Classes {
		add(classHash)
	}

	for classHash := range stateDiff.DeclaredV1Classes {
		add(&classHash)
	}

	return hashes
}
