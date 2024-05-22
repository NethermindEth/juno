package starknet

import (
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// Todo: consider merging this with HeadersPID
func CurrentBlockHeaderPID(n *utils.Network) protocol.ID {
	return n.ProtocolID() + "/current_header/0"
}

func HeadersPID(n *utils.Network) protocol.ID {
	return n.ProtocolID() + "/headers/0.1.0-rc.0"
}

func BlockBodiesPID(n *utils.Network) protocol.ID {
	return n.ProtocolID() + "/block_bodies/0.1.0-rc.0"
}

func EventsPID(n *utils.Network) protocol.ID {
	return n.ProtocolID() + "/events/0.1.0-rc.0"
}

func ReceiptsPID(n *utils.Network) protocol.ID {
	return n.ProtocolID() + "/receipts/0.1.0-rc.0"
}

func TransactionsPID(n *utils.Network) protocol.ID {
	return n.ProtocolID() + "/transactions/0.1.0-rc.0"
}

func ClassesPID(n *utils.Network) protocol.ID {
	return n.ProtocolID() + "/classes/0.1.0-rc.0"
}

func StateDiffPID(n *utils.Network) protocol.ID {
	return n.ProtocolID() + "/state_diffs/0.1.0-rc.0"
}
