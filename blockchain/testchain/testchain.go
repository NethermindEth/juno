package testchain

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

// Test blockchain used
type Testchain struct {
	t *testing.T
	blockchain.Blockchain
	database    db.DB
	Account     classDefinition
	Deployer    classDefinition
	erc20       classDefinition
	ClassHashes map[address.ContractAddress]hash.ClassHash
}

// Creates a new test chain with a default account, erc20 contracts and universal deployer
func NewTestchain(t *testing.T) Testchain {
	t.Helper()

	memoryDB := pebble.NewMemTest(t)
	chain := Testchain{
		t:           t,
		Blockchain:  *blockchain.New(memoryDB, &utils.Sepolia),
		database:    memoryDB,
		ClassHashes: make(map[address.ContractAddress]hash.ClassHash),
	}

	genesisBlock := &core.Block{
		Header: &core.Header{
			Number:          0,
			Timestamp:       0,
			ProtocolVersion: "0.13.5", // I really don't like this, who keeps this number up to date and why is it important
			Hash:            utils.HexToFelt(t, "0xb00"),
			ParentHash:      felt.Zero.Clone(),
			GlobalStateRoot: felt.Zero.Clone(),
		},
	}
	genesisStateUpdate := &core.StateUpdate{
		BlockHash: genesisBlock.Hash,
		NewRoot:   &felt.Zero,
		OldRoot:   &felt.Zero,
		StateDiff: &core.StateDiff{},
	}
	require.NoError(t, chain.Store(genesisBlock, &core.BlockCommitments{}, genesisStateUpdate, nil))

	const prefix = "../../cairo/scarb/target/dev/"
	chain.Account = NewClass(t, prefix+"juno_AccountUpgradeable.contract_class.json")
	chain.Account.AddInstance(
		utils.HexTo[address.ContractAddress](t, "0xc01"),
		utils.HexToFelt(t, "0x10000000000000000000000000000"),
	)
	chain.Deployer = NewClass(t, prefix+"juno_UniversalDeployer.contract_class.json")
	chain.Deployer.AddInstance(
		utils.HexTo[address.ContractAddress](t, "0xc02"),
		&felt.Zero,
	)
	chain.erc20 = NewClass(t, prefix+"juno_ERC20Upgradeable.contract_class.json")
	chain.erc20.AddInstance(
		utils.HexTo[address.ContractAddress](t, "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
		&felt.Zero,
	)
	chain.erc20.AddInstance(
		utils.HexTo[address.ContractAddress](t, "0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d"),
		&felt.Zero,
	)

	chain.Declare(&chain.Account, &chain.Deployer, &chain.erc20)

	return chain
}

func (b *Testchain) AccountInstance() *deployedContract {
	return &b.Account.Instances[0]
}

func (b *Testchain) AccountClassHash() *hash.ClassHash {
	return &b.Account.Hash
}

func (b *Testchain) DeployerInstance() *deployedContract {
	return &b.Deployer.Instances[0]
}

func (b *Testchain) NewRoot(t *testing.T,
	blockNumber uint64, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class,
) *felt.Felt {
	var root *felt.Felt
	require.NoError(t, b.database.Update(func(txn db.Transaction) (err error) {
		root, err = core.NewTestState(txn).RootWithStateUpdate(blockNumber, stateUpdate, newClasses)
		return err
	}))
	return root
}

// Adds each class definition to the testchain, it then make sure they are deployed and with the right balance
func (b *Testchain) Declare(classes ...*classDefinition) {
	// This have to remain the same due to compatibility with old code
	// This should be class hash to class
	classDefinitions := make(map[felt.Felt]core.Class, len(classes))
	// This should be: contract address to class hash
	deployedContracts := make(map[felt.Felt]*felt.Felt) // use pointer because of DeployedContracts field
	// This should be storage key to balance
	balances := make(map[felt.Felt]*felt.Felt) // use pointer because of StorageDiff field

	for _, classDef := range classes {
		classDefinitions[*classDef.Hash.AsFelt()] = &classDef.Class
		for _, instance := range classDef.Instances {
			deployedContracts[*instance.address.AsFelt()] = classDef.Hash.AsFelt()
			b.ClassHashes[instance.address] = classDef.Hash
			if balance := instance.balance; !balance.Equal(&felt.Zero) {
				balances[instance.BalanceKey()] = &balance
			}
		}
	}

	height, err := b.Height()
	require.NoError(b.t, err)
	parentBlock, err := b.BlockByNumber(height)
	require.NoError(b.t, err)

	stateUpdate := &core.StateUpdate{
		OldRoot: parentBlock.GlobalStateRoot,
		StateDiff: &core.StateDiff{
			DeployedContracts: deployedContracts,
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*b.erc20.Instances[0].address.AsFelt(): balances,
				*b.erc20.Instances[1].address.AsFelt(): balances,
			},
		},
	}
	stateUpdate.NewRoot = b.NewRoot(b.t, parentBlock.Number+1, stateUpdate, classDefinitions)

	block := &core.Block{
		Header: &core.Header{
			Hash:             utils.HexTo[felt.Felt](b.t, "0xb01"),
			ParentHash:       parentBlock.Hash,
			Number:           parentBlock.Number + 1,
			GlobalStateRoot:  stateUpdate.NewRoot,
			SequencerAddress: utils.HexToFelt(b.t, "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
			Timestamp:        parentBlock.Timestamp + 1,
			ProtocolVersion:  parentBlock.ProtocolVersion,
			L1GasPriceETH:    utils.HexToFelt(b.t, "0x1"),
			L1GasPriceSTRK:   utils.HexToFelt(b.t, "0x2"),
			L1DAMode:         core.Blob,
			L1DataGasPrice: &core.GasPrice{
				PriceInWei: utils.HexToFelt(b.t, "0x2"),
				PriceInFri: utils.HexToFelt(b.t, "0x2"),
			},
			L2GasPrice: &core.GasPrice{
				PriceInWei: utils.HexToFelt(b.t, "0x1"),
				PriceInFri: utils.HexToFelt(b.t, "0x1"),
			},
		},
	}

	require.NoError(b.t, b.Store(block, &core.BlockCommitments{}, stateUpdate, classDefinitions))
}
