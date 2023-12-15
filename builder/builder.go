package builder

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

type Builder struct {
	ownAddress felt.Felt

	bc *blockchain.Blockchain
	vm vm.VM

	network utils.Network
	log     utils.Logger
}

func New(ownAddr *felt.Felt, bc *blockchain.Blockchain, builderVM vm.VM, log utils.Logger) *Builder {
	return &Builder{
		ownAddress: *ownAddr,

		bc: bc,
		vm: builderVM,
	}
}

// ValidateAgainstPendingState validates a user transaction against the pending state
// only hard-failures result in an error, reverts are not reported back to caller
func (b *Builder) ValidateAgainstPendingState(userTxn *mempool.BroadcastedTransaction) error {
	declaredClasses := []core.Class{}
	if userTxn.DeclaredClass != nil {
		declaredClasses = []core.Class{userTxn.DeclaredClass}
	}

	nextHeight := uint64(0)
	if height, err := b.bc.Height(); err == nil {
		nextHeight = height + 1
	} else if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return err
	}

	pendingBlock, err := b.bc.Pending()
	if err != nil {
		return err
	}

	state, stateCloser, err := b.bc.PendingState()
	if err != nil {
		return err
	}

	defer func() {
		if err = stateCloser(); err != nil {
			b.log.Errorw("closing state in ValidateAgainstPendingState", "err", err)
		}
	}()

	_, _, err = b.vm.Execute([]core.Transaction{userTxn.Transaction}, declaredClasses, nextHeight,
		pendingBlock.Block.Timestamp, &b.ownAddress, state, b.bc.Network(), []*felt.Felt{},
		false, false, false, pendingBlock.Block.GasPrice, pendingBlock.Block.GasPriceSTRK, false)
	return err
}

func (b *Builder) GenesisStateDiff(genesisConfig GenesisConfig) (*core.StateDiff, error) {
	blockTimestamp := uint64(time.Now().Unix())

	newClasses, err := loadClasses(genesisConfig.Classes)
	if err != nil {
		return nil, err
	}

	genStateDiff, err := blockchain.MakeStateDiffForEmptyBlock(b.bc, 0)
	if err != nil {
		return nil, err
	}

	pendingState, closer, err := b.bc.PendingStateGivenNewClassesAndStateDiff(genStateDiff, newClasses)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := closer(); err != nil {
			b.log.Errorw("failed to close state in GenesisState", "err", err)
		}
	}()

	for _, fnCall := range genesisConfig.FunctionCalls {
		classHash, err := pendingState.ContractClassHash(&fnCall.ContractAddress)
		if err != nil {
			return nil, err
		}
		_, err = b.vm.Call(&fnCall.ContractAddress, classHash, &fnCall.EntryPointSelector, fnCall.Calldata, 0, blockTimestamp, pendingState, b.network)
		if err != nil {
			return nil, err
		}
	}
	return pendingState.StateDiff(), nil
}

func loadClasses(classes []string) (map[felt.Felt]core.Class, error) {
	classMap := make(map[felt.Felt]core.Class)
	for _, classPath := range classes {

		file, err := os.Open(classPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		bytes, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, err
		}

		var class core.Class
		if err := json.Unmarshal(bytes, &class); err != nil {
			return nil, err
		}

		classhash, err := class.Hash()
		if err != nil {
			return nil, err
		}
		classMap[*classhash] = class
	}
	return classMap, nil
}
