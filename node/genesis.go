package node

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/vm"
)

func buildGenesis(
	genesisPath string,
	bc *blockchain.Blockchain,
	v vm.VM,
	maxSteps uint64,
	maxGas uint64,
) error {
	if _, err := bc.Height(); !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	var diff core.StateDiff
	var classes map[felt.Felt]core.ClassDefinition
	switch {
	case genesisPath != "":
		genesisConfig, err := genesis.Read(genesisPath)
		if err != nil {
			return err
		}

		diff, classes, err = genesis.GenesisStateDiff(
			genesisConfig,
			v,
			bc.Network(),
			maxSteps,
			maxGas,
		)
		if err != nil {
			return err
		}

	default:
		diff = core.EmptyStateDiff()
	}

	return bc.StoreGenesis(&diff, classes)
}
