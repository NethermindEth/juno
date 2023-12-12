package starknet

import (
	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/protocol"
)

func BlockHeadersPID(n utils.Network) protocol.ID {
	return n.ProtocolID() + "/block_headers/0"
}

func BlockBodiesPID(n utils.Network) protocol.ID {
	return n.ProtocolID() + "/block_bodies/0"
}

func EventsPID(n utils.Network) protocol.ID {
	return n.ProtocolID() + "/events/0"
}

func ReceiptsPID(n utils.Network) protocol.ID {
	return n.ProtocolID() + "/receipts/0"
}

func TransactionsPID(n utils.Network) protocol.ID {
	return n.ProtocolID() + "/transactions/0"
}
