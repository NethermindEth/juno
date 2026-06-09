package consensus_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/consensus"
	consensusDB "github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/driver"
	consensusP2P "github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	consensuswal "github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	kvdb "github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/p2p/pubsub/testutils"
	p2psync "github.com/NethermindEth/juno/p2p/sync"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/sourcegraph/conc"
	"github.com/stretchr/testify/require"
)

type runningConsensusNode struct {
	stop func()
	wait func()
}

type restartNodeStores struct {
	consensusStore  kvdb.KeyValueStore
	blockchainStore kvdb.KeyValueStore
	blockchain      *blockchain.Blockchain
}

type consensusNodeOptions struct {
	requiredWALEntriesWritten chan<- struct{}
	ackCommits                bool
	sentPrecommits            chan<- *starknet.Precommit
}

type observingWALStore struct {
	consensusDB.TendermintWALStore[starknet.Value, starknet.Hash, starknet.Address]
	requiredWALEntriesWritten chan<- struct{}
}

func (s *observingWALStore) Flush() error {
	if err := s.TendermintWALStore.Flush(); err != nil {
		return err
	}
	if !walContainsRequiredReplayEntries(s.TendermintWALStore) {
		return nil
	}
	select {
	case s.requiredWALEntriesWritten <- struct{}{}:
	default:
	}
	return nil
}

type observingPrecommitBroadcaster struct {
	consensusP2P.Broadcaster[*starknet.Precommit]
	sentPrecommits chan<- *starknet.Precommit
}

func (b observingPrecommitBroadcaster) Broadcast(
	ctx context.Context,
	precommit *starknet.Precommit,
) {
	b.Broadcaster.Broadcast(ctx, precommit)
	select {
	case b.sentPrecommits <- precommit:
	default:
	}
}

const (
	restartTestNodeCount         = 4
	restartTestRequiredPrevotes  = 2 * restartTestNodeCount / 3
	restartTestValidatorHighSeed = uint64(0)
	restartTestValidatorLowSeed  = uint64(0)
	restartTestHeight            = types.Height(1)
	restartTestRound             = types.Round(0)
)

func TestConsensusRestartReplaysPersistentWALAndPrecommits(t *testing.T) {
	logger := log.NewNopZapLogger()
	genesisDiff, genesisClasses := loadGenesis(t, logger)

	p2pNodes := testutils.BuildNetworks(t, testutils.LineNetworkConfig(restartTestNodeCount))
	// Use a non-proposer so replay must restore proposal/vote state received from peers.
	restartNodeIndex := chooseNonProposerNodeIndex(restartTestNodeCount)
	restartNodeConsensusAddress := consensus.InitMockServices(
		restartTestValidatorHighSeed,
		restartTestValidatorLowSeed,
		restartNodeIndex,
		restartTestNodeCount,
	).NodeAddress

	consensusDBPath := filepath.Join(t.TempDir(), "consensus")
	blockchainDBPath := filepath.Join(t.TempDir(), "blockchain")
	restartStores := openRestartNodeStores(
		t,
		consensusDBPath,
		blockchainDBPath,
		genesisDiff,
		genesisClasses,
	)

	requiredWALEntriesWritten := make(chan struct{}, 1)
	restartNode, peerNodes := startNetworkBeforeRestart(
		t,
		t.Context(),
		p2pNodes,
		restartNodeIndex,
		logger,
		restartStores,
		genesisDiff,
		genesisClasses,
		requiredWALEntriesWritten,
	)

	// Stop once the restart node has the WAL entries required for replay.
	waitForRequiredWALEntriesAndStop(t, requiredWALEntriesWritten, restartNode)
	// Stop peers too, so restart cannot use fresh network messages.
	stopAndWait(peerNodes)

	// Confirm no block was committed, so recovery must come from WAL replay.
	committedHeight, err := restartStores.blockchain.Height()
	require.NoError(t, err)
	require.Equal(t, uint64(0), committedHeight)

	restartStores.close(t)

	// Reopen stores to simulate a new process, then restart only the selected node.
	restartStores = openRestartNodeStores(
		t,
		consensusDBPath,
		blockchainDBPath,
		genesisDiff,
		genesisClasses,
	)
	restartedNodePrecommits := make(chan *starknet.Precommit, 1)
	restartedNode := startConsensusNode(
		t,
		t.Context(),
		restartNodeIndex,
		&p2pNodes[restartNodeIndex],
		logger.Named("restart-node"),
		restartStores.consensusStore,
		restartStores.blockchain,
		consensusNodeOptions{sentPrecommits: restartedNodePrecommits},
	)
	t.Cleanup(func() {
		stopAndWait([]runningConsensusNode{restartedNode})
		restartStores.close(t)
	})

	// With peers stopped and no committed block, this precommit must come from WAL replay.
	waitForPrecommitFromNode(t, restartedNodePrecommits, restartNodeConsensusAddress)
}

func chooseNonProposerNodeIndex(nodeCount int) int {
	const validatorSetNodeIndex = 0
	validators := consensus.InitMockServices(
		restartTestValidatorHighSeed,
		restartTestValidatorLowSeed,
		validatorSetNodeIndex,
		nodeCount,
	).Validators
	for i := range nodeCount {
		nodeAddress := consensus.InitMockServices(
			restartTestValidatorHighSeed,
			restartTestValidatorLowSeed,
			i,
			nodeCount,
		).NodeAddress
		if validators.Proposer(restartTestHeight, restartTestRound) != nodeAddress {
			return i
		}
	}
	panic("expected to find non-proposer node")
}

func openRestartNodeStores(
	t *testing.T,
	consensusDBPath string,
	blockchainDBPath string,
	genesisDiff core.StateDiff,
	genesisClasses map[felt.Felt]core.ClassDefinition,
) restartNodeStores {
	t.Helper()

	consensusStore, err := pebblev2.New(consensusDBPath)
	require.NoError(t, err)

	blockchainStore, err := pebblev2.New(blockchainDBPath)
	require.NoError(t, err)

	chain := blockchain.New(
		blockchainStore,
		&networks.Mainnet,
		blockchain.WithNewState(statetestutils.UseNewState()),
	)

	_, err = chain.Height()
	switch {
	case errors.Is(err, kvdb.ErrKeyNotFound):
		require.NoError(t, chain.StoreGenesis(&genesisDiff, genesisClasses))
	case err == nil:
	default:
		require.NoError(t, err)
	}

	return restartNodeStores{
		consensusStore:  consensusStore,
		blockchainStore: blockchainStore,
		blockchain:      chain,
	}
}

func (s restartNodeStores) close(t *testing.T) {
	t.Helper()

	require.NoError(t, s.consensusStore.Close())
	require.NoError(t, s.blockchainStore.Close())
}

func startNetworkBeforeRestart(
	t *testing.T,
	parent context.Context,
	p2pNodes []testutils.Node,
	restartNodeIndex int,
	logger *log.ZapLogger,
	restartStores restartNodeStores,
	genesisDiff core.StateDiff,
	genesisClasses map[felt.Felt]core.ClassDefinition,
	requiredWALEntriesWritten chan<- struct{},
) (runningConsensusNode, []runningConsensusNode) {
	t.Helper()

	var restartNode runningConsensusNode
	peerNodes := make([]runningConsensusNode, 0, len(p2pNodes)-1)
	for i := range p2pNodes {
		if i == restartNodeIndex {
			restartNode = startConsensusNode(
				t,
				parent,
				i,
				&p2pNodes[i],
				logger.Named("before-restart"),
				restartStores.consensusStore,
				restartStores.blockchain,
				consensusNodeOptions{requiredWALEntriesWritten: requiredWALEntriesWritten},
			)
			continue
		}

		peerNodes = append(peerNodes, startPeerNode(
			t,
			parent,
			i,
			&p2pNodes[i],
			logger.Named("peer"),
			genesisDiff,
			genesisClasses,
		))
	}

	return restartNode, peerNodes
}

func startPeerNode(
	t *testing.T,
	parent context.Context,
	nodeIndex int,
	p2pNode *testutils.Node,
	logger *log.ZapLogger,
	genesisDiff core.StateDiff,
	genesisClasses map[felt.Felt]core.ClassDefinition,
) runningConsensusNode {
	t.Helper()

	return startConsensusNode(
		t,
		parent,
		nodeIndex,
		p2pNode,
		logger,
		memoryDB(t),
		getBlockchain(t, genesisDiff, genesisClasses),
		// Peers must drain and ack commits, otherwise their drivers block at commit
		// and stop gossiping the proposal/prevotes the restart node needs in its WAL.
		consensusNodeOptions{ackCommits: true},
	)
}

func startConsensusNode(
	t *testing.T,
	parent context.Context,
	nodeIndex int,
	p2pNode *testutils.Node,
	logger *log.ZapLogger,
	consensusStore kvdb.KeyValueStore,
	chain *blockchain.Blockchain,
	options consensusNodeOptions,
) runningConsensusNode {
	t.Helper()

	mockServices := consensus.InitMockServices(
		restartTestValidatorHighSeed,
		restartTestValidatorLowSeed,
		nodeIndex,
		restartTestNodeCount,
	)

	network := &networks.Mainnet
	vm := vm.New(&vm.ChainInfo{
		ChainID:           network.L2ChainID,
		FeeTokenAddresses: networks.DefaultFeeTokenAddresses,
	}, false, logger)
	blockFetcher := p2psync.NewBlockFetcher(
		chain,
		compiler.NewUnsafe(),
		p2pNode.Host,
		network,
		logger,
	)

	initOpts := consensus.InitOptionsForTest{}
	if options.sentPrecommits != nil {
		initOpts.WrapBroadcasters = func(
			broadcasters consensusP2P.Broadcasters[starknet.Value, starknet.Hash, starknet.Address],
		) consensusP2P.Broadcasters[starknet.Value, starknet.Hash, starknet.Address] {
			broadcasters.PrecommitBroadcaster = observingPrecommitBroadcaster{
				Broadcaster:    broadcasters.PrecommitBroadcaster,
				sentPrecommits: options.sentPrecommits,
			}
			return broadcasters
		}
	}
	if options.requiredWALEntriesWritten != nil {
		initOpts.WrapWALStore = func(
			walStore consensusDB.TendermintWALStore[starknet.Value, starknet.Hash, starknet.Address],
		) consensusDB.TendermintWALStore[starknet.Value, starknet.Hash, starknet.Address] {
			return &observingWALStore{
				TendermintWALStore:        walStore,
				requiredWALEntriesWritten: options.requiredWALEntriesWritten,
			}
		}
	}

	services, err := consensus.InitWithOptionsForTest(
		p2pNode.Host,
		logger,
		consensusStore,
		chain,
		vm,
		&blockFetcher,
		&mockServices.NodeAddress,
		mockServices.Validators,
		mockServices.TimeoutFn,
		p2pNode.GetBootstrapPeers,
		compiler.NewUnsafe(),
		initOpts,
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(parent)
	errs := make(chan error, 4)
	wg := conc.NewWaitGroup()

	run := func(name string, fn func(context.Context) error) {
		wg.Go(func() {
			if err := fn(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errs <- errors.New(name + ": " + err.Error())
			}
		})
	}

	run("proposer", services.Proposer.Run)
	run("driver", services.Driver.Run)
	run("p2p", services.P2P.Run)

	if options.ackCommits {
		run("persist", func(ctx context.Context) error {
			return persistCommittedBlocks(ctx, chain, services.CommitListener)
		})
	}

	return runningConsensusNode{
		stop: cancel,
		wait: func() {
			wg.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}
		},
	}
}

func waitForRequiredWALEntriesAndStop(
	t *testing.T,
	requiredWALEntriesWritten <-chan struct{},
	restartNode runningConsensusNode,
) {
	t.Helper()

	select {
	case <-requiredWALEntriesWritten:
	case <-time.After(30 * time.Second):
		restartNode.stop()
		restartNode.wait()
		require.FailNow(t, "timed out waiting for required WAL entries")
	}

	restartNode.stop()
	restartNode.wait()
}

func stopAndWait(nodes []runningConsensusNode) {
	for _, node := range nodes {
		node.stop()
	}
	for _, node := range nodes {
		node.wait()
	}
}

func waitForPrecommitFromNode(
	t *testing.T,
	precommits <-chan *starknet.Precommit,
	nodeAddress starknet.Address,
) {
	t.Helper()

	timeout := time.After(30 * time.Second)
	for {
		select {
		case precommit := <-precommits:
			if precommit.Sender == nodeAddress && precommit.Height == restartTestHeight {
				return
			}
		case <-timeout:
			require.FailNow(t, "timed out waiting for restarted node precommit")
		}
	}
}

func walContainsRequiredReplayEntries(
	walStore consensusDB.TendermintWALStore[starknet.Value, starknet.Hash, starknet.Address],
) bool {
	hasStartForHeight := false
	hasProposalForRound := false
	prevoteSenders := make(map[starknet.Address]struct{}, restartTestRequiredPrevotes)

	for entry, err := range walStore.LoadAllEntries() {
		if err != nil {
			return false
		}
		switch entry := entry.(type) {
		case *consensuswal.Start:
			hasStartForHeight = hasStartForHeight || entry.GetHeight() == restartTestHeight
		case *consensuswal.Proposal[starknet.Value, starknet.Hash, starknet.Address]:
			hasProposalForRound = hasProposalForRound || isRestartRound(entry.Height, entry.Round)
		case *consensuswal.Prevote[starknet.Hash, starknet.Address]:
			if isRestartRound(entry.Height, entry.Round) {
				prevoteSenders[entry.Sender] = struct{}{}
			}
		}
	}
	return hasStartForHeight &&
		hasProposalForRound &&
		len(prevoteSenders) >= restartTestRequiredPrevotes
}

func isRestartRound(height types.Height, round types.Round) bool {
	return height == restartTestHeight && round == restartTestRound
}

func memoryDB(t *testing.T) kvdb.KeyValueStore {
	t.Helper()
	db := memory.New()
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})
	return db
}

func persistCommittedBlocks(
	ctx context.Context,
	chain *blockchain.Blockchain,
	commitListener driver.CommitListener[starknet.Value, starknet.Hash],
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case committedBlock := <-commitListener.Listen():
			commitments, err := chain.SanityCheckNewHeight(
				committedBlock.Block,
				committedBlock.StateUpdate,
				committedBlock.NewClasses,
			)
			if err != nil {
				return err
			}
			if err := chain.Store(
				committedBlock.Block,
				commitments,
				committedBlock.StateUpdate,
				committedBlock.NewClasses,
			); err != nil {
				return err
			}

			committedBlock.Persisted <- nil
		}
	}
}
