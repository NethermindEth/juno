package consensus

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	consensusDB "github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/p2p/config"
	"github.com/NethermindEth/juno/consensus/proposal"
	"github.com/NethermindEth/juno/consensus/proposer"
	"github.com/NethermindEth/juno/consensus/starknet"
	consensusSync "github.com/NethermindEth/juno/consensus/sync"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/p2p/sync"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type ConsensusServices struct {
	Proposer       proposer.Proposer[starknet.Value, starknet.Hash]
	P2P            p2p.P2P[starknet.Value, starknet.Hash, starknet.Address]
	Driver         *driver.Driver[starknet.Value, starknet.Hash, starknet.Address]
	CommitListener driver.CommitListener[starknet.Value, starknet.Hash]
}

func Init(
	host host.Host,
	logger *utils.ZapLogger,
	database db.KeyValueStore,
	blockchain *blockchain.Blockchain,
	vm vm.VM,
	blockFetcher *sync.BlockFetcher,
	nodeAddress *starknet.Address,
	validators votecounter.Validators[starknet.Address],
	timeoutFn driver.TimeoutFn,
	bootstrapPeersFn func() []peer.AddrInfo,
	compiler compiler.Compiler,
) (ConsensusServices, error) {
	chainHeight, err := blockchain.Height()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return ConsensusServices{}, err
	}
	currentHeight := types.Height(chainHeight + 1)

	tendermintDB := consensusDB.NewTendermintDB[starknet.Value, starknet.Hash, starknet.Address](database)

	executor := builder.NewExecutor(blockchain, vm, logger, false, false)
	builder := builder.New(blockchain, executor)

	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	proposer := proposer.New(logger, &builder, &proposalStore, *nodeAddress, toValue)
	stateMachine := tendermint.New(logger, *nodeAddress, proposer, validators, currentHeight)

	p2p := p2p.New(
		host,
		logger,
		&builder,
		&proposalStore,
		currentHeight,
		&config.DefaultBufferSizes,
		bootstrapPeersFn,
		compiler,
	)

	commitListener := driver.NewCommitListener(logger, &proposalStore, proposer, p2p)

	messageExtractor := consensusSync.New(validators, toValue, &proposalStore)

	driver := driver.New(
		logger,
		tendermintDB,
		stateMachine,
		commitListener,
		p2p.Broadcasters(),
		p2p.Listeners(),
		blockFetcher,
		&messageExtractor,
		timeoutFn,
	)

	return ConsensusServices{
		Proposer:       proposer,
		P2P:            p2p,
		Driver:         &driver,
		CommitListener: commitListener,
	}, nil
}

func toValue(value *felt.Felt) starknet.Value {
	return starknet.Value(*value)
}
