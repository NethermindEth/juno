// Package rpcsource lives outside package sync because rpc/v10, whose types it decodes,
// imports sync.
package rpcsource

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/adapters/rpc2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/starknetrpc"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
	gethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/sourcegraph/conc/pool"
	"go.uber.org/zap"
)

const (
	specMajor = 0
	specMinor = 10

	maxConcurrentClassFetches = 4
	defaultErrorPause         = time.Second
)

var _ sync.DataSource = (*Source)(nil)

type Source struct {
	blockchain *blockchain.Blockchain
	client     *starknetrpc.Client
	logger     log.StructuredLogger
	errorPause time.Duration
}

type Option func(*Source)

// WithErrorPause sets how long BlockByNumber waits before returning an error.
func WithErrorPause(d time.Duration) Option {
	return func(s *Source) { s.errorPause = d }
}

func New(
	chain *blockchain.Blockchain,
	client *starknetrpc.Client,
	logger log.StructuredLogger,
	opts ...Option,
) *Source {
	source := &Source{
		blockchain: chain,
		client:     client,
		logger:     logger,
		errorPause: defaultErrorPause,
	}
	for _, opt := range opts {
		opt(source)
	}

	return source
}

func (s *Source) CheckRemote(ctx context.Context) error {
	if err := s.checkChainID(ctx); err != nil {
		return err
	}

	return s.checkSpecVersion(ctx)
}

func (s *Source) checkChainID(ctx context.Context) error {
	chainID, err := s.client.Call(ctx, starknetrpc.ChainID, starknetrpc.NoParams{})
	if err != nil {
		return fmt.Errorf("querying the RPC sync node chain id: %w", err)
	}

	network := s.blockchain.Network()
	if !chainID.Equal(network.L2ChainIDFelt()) {
		return fmt.Errorf(
			"RPC sync node serves chain id %s, want %s (%s) for network %s",
			chainID,
			network.L2ChainIDFelt(),
			network.L2ChainID,
			network.Name,
		)
	}

	return nil
}

func (s *Source) checkSpecVersion(ctx context.Context) error {
	specVersion, err := s.client.Call(ctx, starknetrpc.SpecVersion, starknetrpc.NoParams{})
	if err != nil {
		return fmt.Errorf("querying the RPC sync node spec version: %w", err)
	}

	version, err := semver.NewVersion(*specVersion)
	if err != nil {
		return fmt.Errorf("parsing the RPC sync node spec version %q: %w", *specVersion, err)
	}

	if version.Major() != specMajor || version.Minor() != specMinor {
		return fmt.Errorf(
			"RPC sync node serves spec version %s, want %d.%d; "+
				"point --rpc-sync-url at the node's v0_10 path",
			*specVersion,
			specMajor,
			specMinor,
		)
	}

	return nil
}

// BlockByNumber pauses before reporting an error: the synchronizer refetches as soon as
// it fails, most often while polling for a block the remote doesn't have yet.
func (s *Source) BlockByNumber(
	ctx context.Context,
	blockNumber uint64,
) (sync.CommittedBlock, error) {
	block, err := s.fetchBlock(ctx, blockNumber)
	if err != nil {
		s.pauseAfterError(ctx, blockNumber, err)
	}

	return block, err
}

func (s *Source) pauseAfterError(ctx context.Context, blockNumber uint64, err error) {
	if ctx.Err() != nil {
		return
	}

	if isBlockNotFound(err) {
		s.logger.Debug("Block not available on the RPC sync node yet",
			zap.Uint64("number", blockNumber))
	} else {
		s.logger.Warn("Failed fetching block from the RPC sync node",
			zap.Uint64("number", blockNumber), zap.Error(err))
	}

	select {
	case <-ctx.Done():
	case <-time.After(s.errorPause):
	}
}

func isBlockNotFound(err error) bool {
	var rpcErr gethrpc.Error
	return errors.As(err, &rpcErr) && rpcErr.ErrorCode() == rpccore.ErrBlockNotFound.Code
}

func (s *Source) fetchBlock(
	ctx context.Context,
	blockNumber uint64,
) (sync.CommittedBlock, error) {
	block, err := s.block(ctx, blockNumber)
	if err != nil {
		return sync.CommittedBlock{}, err
	}

	stateUpdate, err := s.stateUpdate(ctx, blockNumber)
	if err != nil {
		return sync.CommittedBlock{}, err
	}

	missingClassHashes, err := sync.MissingClassHashes(s.blockchain, stateUpdate.StateDiff)
	if err != nil {
		return sync.CommittedBlock{}, err
	}

	newClasses, err := s.classes(ctx, blockNumber, missingClassHashes)
	if err != nil {
		return sync.CommittedBlock{}, fmt.Errorf("block %d: %w", blockNumber, err)
	}

	return sync.CommittedBlock{
		Block:       block,
		StateUpdate: stateUpdate,
		NewClasses:  newClasses,
		Persisted:   make(chan error, 1),
	}, nil
}

func (s *Source) block(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	response, err := s.client.Call(
		ctx,
		starknetrpc.BlockWithReceipts,
		starknetrpc.BlockWithReceiptsParams{BlockNumber: blockNumber},
	)
	if err != nil {
		return nil, err
	}

	block, err := rpc2core.AdaptBlock(response, s.blockchain.Network())
	if err != nil {
		return nil, fmt.Errorf("adapting block %d: %w", blockNumber, err)
	}

	return block, nil
}

func (s *Source) stateUpdate(
	ctx context.Context,
	blockNumber uint64,
) (*core.StateUpdate, error) {
	response, err := s.client.Call(
		ctx,
		starknetrpc.StateUpdate,
		starknetrpc.BlockParams{BlockNumber: blockNumber},
	)
	if err != nil {
		return nil, err
	}

	stateUpdate, err := rpc2core.AdaptStateUpdate(response)
	if err != nil {
		return nil, fmt.Errorf("adapting state update %d: %w", blockNumber, err)
	}

	return stateUpdate, nil
}

func (s *Source) classes(
	ctx context.Context,
	blockNumber uint64,
	classHashes []*felt.Felt,
) (map[felt.Felt]core.ClassDefinition, error) {
	p := pool.NewWithResults[core.ClassDefinition]().
		WithContext(ctx).
		WithMaxGoroutines(maxConcurrentClassFetches).
		WithFailFast()
	for _, classHash := range classHashes {
		p.Go(func(ctx context.Context) (core.ClassDefinition, error) {
			return s.class(ctx, blockNumber, classHash)
		})
	}

	definitions, err := p.Wait()
	if err != nil {
		return nil, err
	}

	classes := make(map[felt.Felt]core.ClassDefinition, len(classHashes))
	for i, classHash := range classHashes {
		classes[*classHash] = definitions[i]
	}

	return classes, nil
}

func (s *Source) class(
	ctx context.Context,
	blockNumber uint64,
	classHash *felt.Felt,
) (core.ClassDefinition, error) {
	class, err := s.client.Call(
		ctx,
		starknetrpc.Class,
		starknetrpc.ClassParams{BlockNumber: blockNumber, ClassHash: classHash},
	)
	if err != nil {
		return nil, fmt.Errorf("fetching class %s: %w", classHash, err)
	}

	var casm *rpcv10.CompiledCasmResponse
	if rpc2core.IsSierraClass(class) {
		casm, err = s.client.Call(
			ctx,
			starknetrpc.CompiledCasm,
			starknetrpc.ClassHashParams{ClassHash: classHash},
		)
		if err != nil {
			return nil, fmt.Errorf("fetching compiled class %s: %w", classHash, err)
		}
	}

	return rpc2core.AdaptClass(class, casm)
}

func (s *Source) BlockHeaderLatest(ctx context.Context) (*core.Header, error) {
	head, err := s.client.Call(ctx, starknetrpc.BlockHashAndNumber, starknetrpc.NoParams{})
	if err != nil {
		return nil, err
	}

	return &core.Header{Number: head.Number, Hash: head.Hash}, nil
}
