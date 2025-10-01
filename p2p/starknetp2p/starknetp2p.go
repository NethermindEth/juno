package starknetp2p

import (
	"fmt"

	"github.com/NethermindEth/juno/utils"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/protocol"
)

type Protocol protocol.ID

type SyncSubProtocol protocol.ID

const (
	protocolPrefix = "starknet"

	ConsensusProtocolID Protocol = "consensus"
	MempoolProtocolID   Protocol = "mempool"
	SyncProtocolID      Protocol = "sync"

	HeadersSyncSubProtocol      SyncSubProtocol = "headers"
	StateDiffSyncSubProtocol    SyncSubProtocol = "state_diffs"
	ClassesSyncSubProtocol      SyncSubProtocol = "classes"
	TransactionsSyncSubProtocol SyncSubProtocol = "transactions"
	EventsSyncSubProtocol       SyncSubProtocol = "events"

	syncProtocolVersion = "0.1.0-rc.0"
)

func Sync(network *utils.Network, subProtocol SyncSubProtocol) protocol.ID {
	return protocol.ID(
		fmt.Sprintf(
			"/%s/%s/%s/%s/%s",
			protocolPrefix,
			protocol.ID(network.L2ChainID),
			SyncProtocolID,
			subProtocol,
			syncProtocolVersion,
		),
	)
}

func DHT(network *utils.Network, starknetProtocol Protocol) []dht.Option {
	return []dht.Option{
		dht.ProtocolPrefix("/" + protocolPrefix),
		dht.ProtocolExtension("/" + protocol.ID(network.L2ChainID)),
		dht.ProtocolExtension("/" + protocol.ID(starknetProtocol)),
	}
}
