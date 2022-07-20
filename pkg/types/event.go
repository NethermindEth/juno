package types

import (
	"github.com/NethermindEth/juno/pkg/felt"
)

type Event struct {
	FromAddress *felt.Felt
	Keys        []*felt.Felt
	Data        []*felt.Felt
}
