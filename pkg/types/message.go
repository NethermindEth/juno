package types

import (
	"github.com/NethermindEth/juno/pkg/felt"
)

type MsgToL1 struct {
	ToAddress   EthAddress
	FromAddress *felt.Felt
	Payload     []*felt.Felt
}

type MsgToL2 struct {
	FromAddress EthAddress
	ToAddress   *felt.Felt
	Payload     []*felt.Felt
}
