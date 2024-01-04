package node

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/vm"
)

func buildGenesis(genesisPath string, bc *blockchain.Blockchain, v vm.VM) error {
	if _, err := bc.Height(); !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	genesisConfig, err := genesis.Read(genesisPath)
	if err != nil {
		return err
	}

	diff, classes, err := genesis.GenesisStateDiff(genesisConfig, v, bc.Network())
	if err != nil {
		return err
	}

	return bc.StoreGenesis(diff, classes)
}
