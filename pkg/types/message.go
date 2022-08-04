package types

import (
	"github.com/NethermindEth/juno/pkg/felt"
)

type MsgToL1 struct {
	FromAddress *felt.Felt
	ToAddress   EthAddress
	Payload     []*felt.Felt
}

type MsgToL2 struct {
	FromAddress EthAddress
	ToAddress   *felt.Felt
	Payload     []*felt.Felt
}
