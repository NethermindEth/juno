package starknet

import (
	"github.com/libp2p/go-libp2p/core/protocol"
)

const Prefix = "/starknet"

func HeadersPID() protocol.ID {
	return Prefix + "/headers/0.1.0-rc.0"
}

func EventsPID() protocol.ID {
	return Prefix + "/events/0.1.0-rc.0"
}

func TransactionsPID() protocol.ID {
	return Prefix + "/transactions/0.1.0-rc.0"
}

func ClassesPID() protocol.ID {
	return Prefix + "/classes/0.1.0-rc.0"
}

func StateDiffPID() protocol.ID {
	return Prefix + "/state_diffs/0.1.0-rc.0"
}

func ChainPID(chainID string) protocol.ID {
	return protocol.ID("/" + chainID)
}
