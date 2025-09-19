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
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
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
	nodeAddress *starknet.Address,
	validators votecounter.Validators[starknet.Address],
	timeoutFn driver.TimeoutFn,
	bootstrapPeersFn func() []peer.AddrInfo,
) (ConsensusServices, error) {
	chainHeight, err := blockchain.Height()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return ConsensusServices{}, err
	}
	currentHeight := types.Height(chainHeight + 1)

	tendermintDB := consensusDB.NewTendermintDB[starknet.Value, starknet.Hash, starknet.Address](database)

	executor := builder.NewExecutor(blockchain, vm, logger, false, true) // TODO: We're currently skipping signature validation
	builder := builder.New(blockchain, executor)

	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	proposer := proposer.New(logger, &builder, &proposalStore, *nodeAddress, toValue)
	stateMachine := tendermint.New(logger, *nodeAddress, proposer, validators, currentHeight)

	p2p := p2p.New(host, logger, &builder, &proposalStore, currentHeight, &config.DefaultBufferSizes, bootstrapPeersFn)

	commitListener := driver.NewCommitListener(logger, &proposalStore, proposer, p2p)
	driver := driver.New(
		logger,
		tendermintDB,
		stateMachine,
		commitListener,
		p2p.Broadcasters(),
		p2p.Listeners(),
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
