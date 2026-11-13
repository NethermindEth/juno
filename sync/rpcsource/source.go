// Package rpcsource lives outside package sync because rpc/v10, whose types it decodes,
// imports sync.
package rpcsource

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/NethermindEth/juno/adapters/rpc2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients/rpc"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils/log"
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
	client     *rpc.Client
	feeder     sync.DataSource
	mode       Mode
	caughtUp   atomic.Bool
	remoteHead atomic.Pointer[core.Header]
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
	client *rpc.Client,
	feeder sync.DataSource,
	mode Mode,
	logger log.StructuredLogger,
	opts ...Option,
) *Source {
	source := &Source{
		blockchain: chain,
		client:     client,
		feeder:     feeder,
		mode:       mode,
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

	if err := s.checkSpecVersion(ctx); err != nil {
		return err
	}

	return s.checkNextBlockNotPruned(ctx)
}

func (s *Source) checkChainID(ctx context.Context) error {
	chainID, err := s.client.Call(ctx, rpc.ChainID, rpc.NoParams{})
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
	specVersion, err := s.client.Call(ctx, rpc.SpecVersion, rpc.NoParams{})
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

func (s *Source) checkNextBlockNotPruned(ctx context.Context) error {
	next, err := s.nextBlockNumber()
	if err != nil {
		return err
	}

	reached, err := s.remoteHeadReaches(ctx, next)
	if err != nil {
		return err
	}
	if !reached {
		return nil
	}

	served, err := s.remoteServesBlock(ctx, next)
	if err != nil {
		return err
	}
	if !served {
		return fmt.Errorf(
			"RPC sync node has pruned block %d, which this node needs next; "+
				"point --rpc-sync-url at an archive node or start from a recent database snapshot",
			next,
		)
	}

	return nil
}

func (s *Source) nextBlockNumber() (uint64, error) {
	height, err := s.blockchain.Height()
	if errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading the local chain height: %w", err)
	}

	return height + 1, nil
}

func (s *Source) remoteHeadReaches(ctx context.Context, blockNumber uint64) (bool, error) {
	head, err := s.refreshRemoteHead(ctx)
	if rpc.IsError(err, rpccore.ErrNoBlock) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("querying the RPC sync node head: %w", err)
	}

	return head.Number >= blockNumber, nil
}

func (s *Source) remoteServesBlock(ctx context.Context, blockNumber uint64) (bool, error) {
	_, err := s.client.Call(
		ctx,
		rpc.BlockWithReceipts,
		rpc.BlockWithReceiptsParams{BlockNumber: blockNumber},
	)
	if rpc.IsError(err, rpccore.ErrBlockNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("querying block %d from the RPC sync node: %w", blockNumber, err)
	}

	return true, nil
}

func (s *Source) fetchesFromFeeder(ctx context.Context) (bool, error) {
	switch s.mode {
	case Follow:
		return false, nil
	case CatchUp:
		return s.catchUpEnded(ctx)
	default:
		return false, fmt.Errorf("unknown RPC sync mode %s", s.mode)
	}
}

func (s *Source) catchUpEnded(ctx context.Context) (bool, error) {
	if s.caughtUp.Load() {
		return true, nil
	}

	next, err := s.nextBlockNumber()
	if err != nil {
		return false, err
	}

	if s.knownRemoteHeadReaches(next) {
		return false, nil
	}

	remoteHasNext, err := s.remoteHeadReaches(ctx, next)
	if err != nil {
		return false, err
	}
	if remoteHasNext {
		return false, nil
	}

	s.endCatchUp(next)
	return true, nil
}

func (s *Source) knownRemoteHeadReaches(blockNumber uint64) bool {
	head := s.remoteHead.Load()
	return head != nil && head.Number >= blockNumber
}

func (s *Source) endCatchUp(nextBlockNumber uint64) {
	if s.caughtUp.CompareAndSwap(false, true) {
		s.logger.Info(
			"Holding every block the RPC sync node has; "+
				"syncing committed blocks from the feeder gateway for the rest of the run",
			zap.Uint64("next", nextBlockNumber),
		)
	}
}

// BlockByNumber pauses before reporting an RPC error: the synchronizer refetches as soon
// as it fails, most often while polling for a block the remote doesn't have yet.
func (s *Source) BlockByNumber(
	ctx context.Context,
	blockNumber uint64,
) (sync.CommittedBlock, error) {
	fromFeeder, err := s.fetchesFromFeeder(ctx)
	if err != nil {
		s.pauseAfterError(ctx, blockNumber, err)
		return sync.CommittedBlock{}, err
	}
	if fromFeeder {
		return s.feeder.BlockByNumber(ctx, blockNumber)
	}

	return s.rpcBlockByNumber(ctx, blockNumber)
}

func (s *Source) rpcBlockByNumber(
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

	if rpc.IsError(err, rpccore.ErrBlockNotFound) {
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
		Block:       &block,
		StateUpdate: &stateUpdate,
		NewClasses:  newClasses,
		Persisted:   make(chan error, 1),
	}, nil
}

func (s *Source) block(ctx context.Context, blockNumber uint64) (core.Block, error) {
	response, err := s.client.Call(
		ctx,
		rpc.BlockWithReceipts,
		rpc.BlockWithReceiptsParams{BlockNumber: blockNumber},
	)
	if err != nil {
		return core.Block{}, err
	}

	block, err := rpc2core.AdaptBlock(response, s.blockchain.Network())
	if err != nil {
		return core.Block{}, fmt.Errorf("adapting block %d: %w", blockNumber, err)
	}

	return block, nil
}

func (s *Source) stateUpdate(
	ctx context.Context,
	blockNumber uint64,
) (core.StateUpdate, error) {
	response, err := s.client.Call(
		ctx,
		rpc.StateUpdate,
		rpc.BlockParams{BlockNumber: blockNumber},
	)
	if err != nil {
		return core.StateUpdate{}, err
	}

	stateUpdate, err := rpc2core.AdaptStateUpdate(response)
	if err != nil {
		return core.StateUpdate{}, fmt.Errorf("adapting state update %d: %w", blockNumber, err)
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
		rpc.Class,
		rpc.ClassParams{BlockNumber: blockNumber, ClassHash: classHash},
	)
	if err != nil {
		return nil, fmt.Errorf("fetching class %s: %w", classHash, err)
	}

	var casm *rpcv10.CompiledCasmResponse
	if rpc2core.IsSierraClass(class) {
		casm, err = s.client.Call(
			ctx,
			rpc.CompiledCasm,
			rpc.ClassHashParams{ClassHash: classHash},
		)
		if err != nil {
			return nil, fmt.Errorf("fetching compiled class %s: %w", classHash, err)
		}
	}

	return rpc2core.AdaptClass(class, casm)
}

func (s *Source) BlockHeaderLatest(ctx context.Context) (*core.Header, error) {
	fromFeeder, err := s.fetchesFromFeeder(ctx)
	if err != nil {
		return nil, err
	}
	if fromFeeder {
		return s.feeder.BlockHeaderLatest(ctx)
	}

	return s.refreshRemoteHead(ctx)
}

// refreshRemoteHead forgets the stored head when the RPC node has none left, so a head
// that moved backwards never keeps catch-up from ending.
func (s *Source) refreshRemoteHead(ctx context.Context) (*core.Header, error) {
	head, err := s.client.Call(ctx, rpc.BlockHashAndNumber, rpc.NoParams{})
	if rpc.IsError(err, rpccore.ErrNoBlock) {
		s.remoteHead.Store(nil)
	}
	if err != nil {
		return nil, err
	}

	header := &core.Header{Number: head.Number, Hash: head.Hash}
	s.remoteHead.Store(header)
	return header, nil
}
