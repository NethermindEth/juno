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

func buildGenesis(genesisPath string, sequencerMode bool, bc *blockchain.Blockchain, v vm.VM) error {
	if _, err := bc.Height(); !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	var diff *core.StateDiff
	var classes map[felt.Felt]core.Class
	switch {
	case genesisPath != "":
		genesisConfig, err := genesis.Read(genesisPath)
		if err != nil {
			return err
		}

		diff, classes, err = genesis.GenesisStateDiff(genesisConfig, v, bc.Network())
		if err != nil {
			return err
		}
	case sequencerMode:
		diff = core.EmptyStateDiff()
	default:
		return nil
	}

	return bc.StoreGenesis(diff, classes)
}
