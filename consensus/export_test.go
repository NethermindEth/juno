package consensus

import (
	"github.com/NethermindEth/juno/blockchain"
	consensusDB "github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/votecounter"
	kvdb "github.com/NethermindEth/juno/db"
	p2psync "github.com/NethermindEth/juno/p2p/sync"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/NethermindEth/juno/vm"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

type InitOptionsForTest struct {
	WrapWALStore func(
		consensusDB.TendermintWALStore[starknet.Value, starknet.Hash, starknet.Address],
	) consensusDB.TendermintWALStore[starknet.Value, starknet.Hash, starknet.Address]
	WrapBroadcasters func(
		p2p.Broadcasters[starknet.Value, starknet.Hash, starknet.Address],
	) p2p.Broadcasters[starknet.Value, starknet.Hash, starknet.Address]
}

func InitWithOptionsForTest(
	host host.Host,
	logger *log.ZapLogger,
	database kvdb.KeyValueStore,
	blockchain *blockchain.Blockchain,
	vm vm.VM,
	blockFetcher *p2psync.BlockFetcher,
	nodeAddress *starknet.Address,
	validators votecounter.Validators[starknet.Address],
	timeoutFn driver.TimeoutFn,
	bootstrapPeersFn func() []peer.AddrInfo,
	compiler compiler.Compiler,
	options InitOptionsForTest,
) (ConsensusServices, error) {
	return initWithOptions(
		host,
		logger,
		database,
		blockchain,
		vm,
		blockFetcher,
		nodeAddress,
		validators,
		timeoutFn,
		bootstrapPeersFn,
		compiler,
		initOptions{
			wrapWALStore:     options.WrapWALStore,
			wrapBroadcasters: options.WrapBroadcasters,
		},
	)
}
