package core

import "github.com/consensys/gnark-crypto/ecc/stark-curve/fp"

type StateUpdate struct {
	BlockHash *fp.Element
	NewRoot   *fp.Element
	OldRoot   *fp.Element

	StateDiff struct {
		StorageDiffs map[string][]struct {
			Key   *fp.Element
			Value *fp.Element
		}

		Nonces            map[string]*fp.Element
		DeployedContracts []struct {
			Address   *fp.Element
			ClassHash *fp.Element
		}
		DeclaredContracts []*fp.Element
	}
}
