package state

import "github.com/NethermindEth/juno/core/felt"

type StateContract struct {
	Nonce        felt.Felt
	ClassHash    felt.Felt
	StorageRoot  felt.Felt
	DeployHeight uint64
}
