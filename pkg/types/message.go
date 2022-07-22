package types

import (
	"github.com/NethermindEth/juno/pkg/felt"
)

type MsgToL1 struct {
	ToAddress EthAddress
	Payload   []*felt.Felt
}

type MsgToL2 struct {
	FromAddress EthAddress
	Payload     []*felt.Felt
}
