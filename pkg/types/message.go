package types

import (
	"github.com/NethermindEth/juno/pkg/felt"
)

type MessageL2ToL1 struct {
	ToAddress EthAddress
	Payload   []*felt.Felt
}

type MessageL1ToL2 struct {
	FromAddress EthAddress
	Payload     []*felt.Felt
}
