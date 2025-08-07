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
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type ConsensusServices struct {
	Host           host.Host
	Proposer       proposer.Proposer[starknet.Value, starknet.Hash]
	P2P            p2p.P2P[starknet.Value, starknet.Hash, starknet.Address]
	Driver         *driver.Driver[starknet.Value, starknet.Hash, starknet.Address]
	CommitListener driver.CommitListener[starknet.Value, starknet.Hash, starknet.Address]
}

func Init(
	logger *utils.ZapLogger,
	database db.KeyValueStore,
	blockchain *blockchain.Blockchain,
	vm vm.VM,
	nodeAddress *starknet.Address,
	validators votecounter.Validators[starknet.Address],
	timeoutFn driver.TimeoutFn,
	hostAddress string,
	hostPrivateKey crypto.PrivKey,
	bootstrapPeersFn func() []peer.AddrInfo,
) (ConsensusServices, error) {
	chainHeight, err := blockchain.Height()
	if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return ConsensusServices{}, err
	}
	currentHeight := types.Height(chainHeight + 1)

	tendermintDB := consensusDB.NewTendermintDB[starknet.Value, starknet.Hash, starknet.Address](database, currentHeight)

	executor := builder.NewExecutor(blockchain, vm, logger, false, true) // TODO: We're currently skipping signature validation
	builder := builder.New(blockchain, executor)

	proposalStore := proposal.ProposalStore[starknet.Hash]{}
	proposer := proposer.New(logger, &builder, &proposalStore, *nodeAddress, toValue)
	stateMachine := tendermint.New(tendermintDB, logger, *nodeAddress, proposer, validators, currentHeight)

	host, err := libp2p.New(
		libp2p.ListenAddrStrings(hostAddress),
		libp2p.Identity(hostPrivateKey),
		// libp2p.UserAgent(makeAgentName(version)),
		// // Use address factory to add the public address to the list of
		// // addresses that the node will advertise.
		// libp2p.AddrsFactory(addressFactory),
		// If we know the public ip, enable the relay service.
		libp2p.EnableRelayService(),
		// When listening behind NAT, enable peers to try to poke thought the
		// NAT in order to reach the node.
		libp2p.EnableHolePunching(),
		// Try to open a port in the NAT router to accept incoming connections.
		libp2p.NATPortMap(),
	)
	if err != nil {
		return ConsensusServices{}, err
	}

	p2p := p2p.New(host, logger, &builder, &proposalStore, currentHeight, &config.DefaultBufferSizes, bootstrapPeersFn)

	commitListener := driver.NewCommitListener(logger, &proposalStore, proposer, p2p)
	driver := driver.New(logger, tendermintDB, stateMachine, commitListener, p2p, timeoutFn)

	return ConsensusServices{
		Host:           host,
		Proposer:       proposer,
		P2P:            p2p,
		Driver:         &driver,
		CommitListener: commitListener,
	}, nil
}

func toValue(value *felt.Felt) starknet.Value {
	return starknet.Value(*value)
}
