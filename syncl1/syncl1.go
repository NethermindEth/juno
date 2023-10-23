package syncl1

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/l1data"
	"github.com/NethermindEth/juno/utils"
)

type Config struct {
	coreDeployment    uint64
	noConstructorArgs uint64
	v011DiffFormat    uint64
}

// MainnetConfig specifies a partial history of Starknet's breaking changes on mainnet.
// TODO this should probably be returned from utils.Network.L1Config or something.
var MainnetConfig = &Config{
	coreDeployment:    13620297,
	noConstructorArgs: 4873,
	v011DiffFormat:    28566,
}

// Synchronizer assumes that all logs are finalised and does not implement reorg handling.
// It should not sync past the finalised L1 head.
type Synchronizer struct {
	iter      *StateUpdateIterator
	db        db.DB
	log       utils.SimpleLogger
	config    *Config
	maxHeight uint64
}

func New(database db.DB, logFetcher l1data.StateUpdateLogFetcher, diffFetcher l1data.StateDiffFetcher,
	config *Config, end uint64, log utils.SimpleLogger,
) (*Synchronizer, error) {
	return &Synchronizer{
		iter: NewIterator(logFetcher, diffFetcher, writer{
			db:     database,
			config: config,
			log:    log,
		}.UpdateState),
		db:        database,
		config:    config,
		maxHeight: end,
		log:       log,
	}, nil
}

func (s *Synchronizer) Run(ctx context.Context) error {
	for {
		ethHeight, startHeight, err := s.init()
		if err != nil {
			return err
		}
		if err := s.iter.Run(ctx, startHeight, s.maxHeight, ethHeight); err == nil {
			break
		} else {
			s.log.Warnw("Failure in sync loop, retrying...", "err", err)
		}
	}
	return nil
}

// init returns the ethHeight and nextStarknetBlockNumber.
func (s *Synchronizer) init() (uint64, uint64, error) {
	var ethHeight uint64
	var nextStarknetBlockNumber uint64
	if err := s.db.View(func(txn db.Transaction) error {
		if err := txn.Get(db.EthereumHeight.Key(), func(ethHeightBytes []byte) error {
			// We do not add one to the number below since there may be logs in the same
			// block that we have not synced yet.
			ethHeight = binary.BigEndian.Uint64(ethHeightBytes)
			return nil
		}); err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return fmt.Errorf("get eth height: %v", err)
			}
			ethHeight = s.config.coreDeployment
		}

		var l1Head *core.L1Head
		if err := txn.Get(db.L1Height.Key(), func(l1HeadBytes []byte) error {
			return encoder.Unmarshal(l1HeadBytes, &l1Head)
		}); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return fmt.Errorf("get l1 height: %v", err)
		}
		if l1Head != nil {
			nextStarknetBlockNumber = l1Head.BlockNumber + 1
		}
		return nil
	}); err != nil {
		return 0, 0, fmt.Errorf("get synchronizer state from db: %v", err)
	}
	return ethHeight, nextStarknetBlockNumber, nil
}

type deployedState struct {
	state *core.State
}

func (s deployedState) ContractIsDeployed(address *felt.Felt) (bool, error) {
	if _, err := s.state.ContractClassHash(address); errors.Is(err, core.ErrContractNotDeployed) {
		return false, nil
	} else if err == nil {
		return true, nil
	} else {
		return false, err
	}
}

type writer struct {
	db     db.DB
	config *Config
	log    utils.SimpleLogger
}

func (w writer) UpdateState(encodedDiff []*big.Int, currentLog *l1data.LogStateUpdate) error {
	ethHeight := currentLog.Raw.BlockNumber
	var blockHash *felt.Felt
	if currentLog.BlockHash != nil {
		blockHash = new(felt.Felt).SetBigInt(currentLog.BlockHash)
	}
	l1Head := &core.L1Head{
		BlockNumber: currentLog.BlockNumber.Uint64(),
		BlockHash:   blockHash,
		StateRoot:   new(felt.Felt).SetBigInt(currentLog.GlobalRoot),
	}
	if err := w.db.Update(func(txn db.Transaction) error {
		state := core.NewState(txn)
		var curRoot *felt.Felt
		curRoot, err := state.Root()
		if err != nil {
			return fmt.Errorf("get root before block %d: %v", l1Head.BlockNumber, err)
		}
		if curRoot == nil {
			curRoot = &felt.Zero
		}

		var stateDiff *core.StateDiff
		if l1Head.BlockNumber < w.config.v011DiffFormat {
			stateDiff = l1data.DecodePre011Diff(encodedDiff, l1Head.BlockNumber < w.config.noConstructorArgs)
		} else {
			stateDiff, err = l1data.Decode011Diff(encodedDiff, deployedState{state: state})
			if err != nil {
				return fmt.Errorf("decode v0.11 diff: %v", err)
			}
		}

		newClasses := make(map[felt.Felt]core.Class)
		for classHash := range stateDiff.DeclaredV1Classes {
			if _, err = state.Class(&classHash); errors.Is(err, db.ErrKeyNotFound) {
				newClasses[classHash] = &core.Cairo1Class{}
			} else if err != nil {
				return fmt.Errorf("get class %s: %v", classHash.Text(felt.Base16), err)
			}
		}
		update := &core.StateUpdate{
			NewRoot:   l1Head.StateRoot,
			OldRoot:   curRoot,
			StateDiff: stateDiff,
			BlockHash: l1Head.BlockHash,
		}
		if err = state.Update(l1Head.BlockNumber, update, newClasses); err != nil {
			return fmt.Errorf("update state at block %d: %v", l1Head.BlockNumber, err)
		}

		var starknetL1HeadBytes []byte
		starknetL1HeadBytes, err = encoder.Marshal(l1Head)
		if err != nil {
			return err
		}
		if err = txn.Set(db.L1Height.Key(), starknetL1HeadBytes); err != nil {
			return fmt.Errorf("set l1 head: %v", err)
		}
		if err = txn.Set(db.EthereumHeight.Key(), core.MarshalBlockNumber(ethHeight)); err != nil {
			return fmt.Errorf("set ethereum height: %v", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("update state: %v", err)
	}
	w.log.Infow("Updated state", "blockNumber", l1Head.BlockNumber)
	return nil
}
