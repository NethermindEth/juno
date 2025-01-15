package sync

import (
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/protocol"
)

const Prefix = "/starknet"

func HeadersPID(network *utils.Network) protocol.ID {
	return protocol.ID(Prefix + "/" + network.L2ChainID + "/sync/headers/0.1.0-rc.0")
}

func EventsPID(network *utils.Network) protocol.ID {
	return protocol.ID(Prefix + "/" + network.L2ChainID + "/sync/events/0.1.0-rc.0")
}

func TransactionsPID(network *utils.Network) protocol.ID {
	return protocol.ID(Prefix + "/" + network.L2ChainID + "/sync/transactions/0.1.0-rc.0")
}

func ClassesPID(network *utils.Network) protocol.ID {
	return protocol.ID(Prefix + "/" + network.L2ChainID + "/sync/classes/0.1.0-rc.0")
}

func StateDiffPID(network *utils.Network) protocol.ID {
	return protocol.ID(Prefix + "/" + network.L2ChainID + "/sync/state_diffs/0.1.0-rc.0")
}

func DHTPrefixPID(network *utils.Network) protocol.ID {
	return protocol.ID(Prefix + "/" + network.L2ChainID + "/sync")
}
