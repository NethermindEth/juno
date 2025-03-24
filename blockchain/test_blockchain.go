package blockchain

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/hash"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

// Test blockchain used
type Testchain struct {
	Blockchain
	t           *testing.T
	Account     classDefinition
	Deployer    classDefinition
	erc20       classDefinition
	ClassHashes map[address.ContractAddress]hash.ClassHash
}

// Creates a new test chain with a default account, erc20 contracts and universal deployer
func NewTestchain(t *testing.T) Testchain {
	t.Helper()

	testDB := pebble.NewMemTest(t)
	chain := Testchain{
		Blockchain:  *New(testDB, &utils.Sepolia),
		t:           t,
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
	account := NewClass(t, prefix+"juno_AccountUpgradeable.contract_class.json")
	account.AddInstance(
		utils.HexTo[address.ContractAddress](t, "0xc01"),
		utils.HexToFelt(t, "0x10000000000000000000000000000"),
	)
	deployer := NewClass(t, prefix+"juno_UniversalDeployer.contract_class.json")
	deployer.AddInstance(
		utils.HexTo[address.ContractAddress](t, "0xc02"),
		&felt.Zero,
	)
	erc20 := NewClass(t, prefix+"juno_ERC20Upgradeable.contract_class.json")
	erc20.AddInstance(
		utils.HexTo[address.ContractAddress](t, "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
		&felt.Zero,
	)
	erc20.AddInstance(
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

type deployedContract struct {
	t       *testing.T
	address address.ContractAddress
	balance felt.Felt
}

func (c *deployedContract) Address() *address.ContractAddress {
	return &c.address
}

func (c *deployedContract) Balance() *felt.Felt {
	return &c.balance
}

func (a *deployedContract) BalanceKey() felt.Felt {
	return feltFromNameAndKey(a.t, "ERC20_balances", a.address.AsFelt())
}

type classDefinition struct {
	t         *testing.T
	Hash      hash.ClassHash
	Instances []deployedContract
	Class     core.Cairo1Class
	Sierra    starknet.SierraDefinition
}

func NewClass(t *testing.T, path string) classDefinition {
	t.Helper()

	sierra, classDef := classFromFile(t, path)
	classHash, err := classDef.Hash()
	require.NoError(t, err)

	return classDefinition{
		t:      t,
		Hash:   hash.ClassHash(*classHash),
		Class:  classDef,
		Sierra: sierra,
	}
}

func (t *classDefinition) AddInstance(address *address.ContractAddress, balance *felt.Felt) {
	t.Instances = append(t.Instances, deployedContract{
		t:       t.t,
		address: *address,
		balance: *balance,
	})
}

func classFromFile(t *testing.T, path string) (
	starknet.SierraDefinition, core.Cairo1Class,
) {
	t.Helper()

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	intermediate := new(struct {
		Abi         json.RawMessage            `json:"abi"`
		EntryPoints starknet.SierraEntryPoints `json:"entry_points_by_type"`
		Program     []*felt.Felt               `json:"sierra_program"`
		Version     string                     `json:"contract_class_version"`
	})
	require.NoError(t, json.NewDecoder(file).Decode(intermediate))

	snSierra := starknet.SierraDefinition{
		Abi:         string(intermediate.Abi),
		EntryPoints: intermediate.EntryPoints,
		Program:     intermediate.Program,
		Version:     intermediate.Version,
	}

	snCasm, err := compiler.Compile(&snSierra)
	require.NoError(t, err)

	junoClass, err := sn2core.AdaptCairo1Class(&snSierra, snCasm)
	require.NoError(t, err)

	// TODO: I don't like this too much (the deference using *), but every method that's currently
	//       returning a reference type will eventually return a value type. Since this is for
	//       testing is not critical.
	return snSierra, *junoClass
}

// https://github.com/eqlabs/pathfinder/blob/7664cba5145d8100ba1b6b2e2980432bc08d72a2/crates/common/src/lib.rs#L124
func feltFromNameAndKey(t *testing.T, name string, key *felt.Felt) felt.Felt {
	// TODO: The use of Big ints is not necessary at all. I am leaving it here because it is not critical
	//       but it should be change to using the felt implementation directly
	t.Helper()

	intermediate := crypto.StarknetKeccak([]byte(name))
	byteArr := crypto.Pedersen(intermediate, key).Bytes()
	value := new(big.Int).SetBytes(byteArr[:])

	maxAddr, ok := new(big.Int).SetString("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00", 0)
	require.True(t, ok)

	value = value.Rem(value, maxAddr)

	res := felt.Felt{}
	res.SetBigInt(value)

	return res
}
