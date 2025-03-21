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

type testBlockchain struct {
	Blockchain
	t           *testing.T
	Account     TestClass
	Deployer    TestClass
	erc20       TestClass
	ClassHashes map[address.ContractAddress]hash.ClassHash
}

func NewTestBlockchain(t *testing.T) testBlockchain {
	t.Helper()

	testDB := pebble.NewMemTest(t)
	chain := testBlockchain{
		Blockchain:  *New(testDB, &utils.Sepolia),
		t:           t,
		ClassHashes: make(map[address.ContractAddress]hash.ClassHash),
	}

	genesisBlock := &core.Block{
		Header: &core.Header{
			Number:          0,
			Timestamp:       0,
			ProtocolVersion: "0.13.5", // I really don't like this
			Hash:            utils.HexToFelt(t, "0xb00"),
			ParentHash:      &felt.Zero,
			GlobalStateRoot: &felt.Zero,
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
	// Predeploy presets
	// https://github.com/OpenZeppelin/cairo-contracts/tree/main/packages/presets
	chain.Account = NewClass(t, prefix+"juno_AccountUpgradeable.contract_class.json")
	chain.Account.AddAccount(
		utils.HexTo[address.ContractAddress](t, "0xc01"),
		utils.HexToFelt(t, "0x10000000000000000000000000000"),
	)
	chain.Deployer = NewClass(t, prefix+"juno_UniversalDeployer.contract_class.json")
	chain.Deployer.AddAccount(
		utils.HexTo[address.ContractAddress](t, "0xc02"),
		&felt.Zero,
	)
	chain.erc20 = NewClass(t, prefix+"juno_ERC20Upgradeable.contract_class.json")
	chain.erc20.AddAccount(
		utils.HexTo[address.ContractAddress](t, "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
		&felt.Zero,
	)
	chain.erc20.AddAccount(
		utils.HexTo[address.ContractAddress](t, "0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d"),
		&felt.Zero,
	)

	chain.Prepare(t, []TestClass{chain.Account, chain.Deployer, chain.erc20})

	return chain
}

func (b *testBlockchain) MainAccount() *testAccount {
	return &b.Account.Accounts[0]
}

func (b *testBlockchain) DeployerAddress() *address.ContractAddress {
	return &b.Deployer.Accounts[0].Address
}

func (b *testBlockchain) AccountClassHash() *hash.ClassHash {
	return &b.Account.Hash
}

func (b *testBlockchain) NewRoot(t *testing.T,
	blockNumber uint64, stateUpdate *core.StateUpdate, newClasses map[felt.Felt]core.Class,
) *felt.Felt {
	var root *felt.Felt
	require.NoError(t, b.database.Update(func(txn db.Transaction) (err error) {
		root, err = core.NewTestState(txn).RootWithStateUpdate(blockNumber, stateUpdate, newClasses)
		return err
	}))
	return root
}

type testAccount struct {
	t       *testing.T
	Address address.ContractAddress
	Balance felt.Felt
}

func (a *testAccount) BalanceKey() felt.Felt {
	return feltFromNameAndKey(a.t, "ERC20_balances", a.Address.AsFelt())
}

type TestClass struct {
	t         *testing.T
	Hash      hash.ClassHash
	Accounts  []testAccount
	JunoClass core.Cairo1Class
	SnSierra  starknet.SierraDefinition
	SnCasm    starknet.CompiledClass
}

func NewClass(t *testing.T, path string) TestClass {
	t.Helper()

	sierra, casm, junoClass := classFromFile(t, path)
	classHash, err := junoClass.Hash()
	require.NoError(t, err)

	return TestClass{
		t:         t,
		JunoClass: junoClass,
		Hash:      hash.ClassHash(*classHash),
		SnSierra:  sierra,
		SnCasm:    casm,
	}
}

func (t *TestClass) AddAccount(address *address.ContractAddress, balance *felt.Felt) {
	t.Accounts = append(t.Accounts, testAccount{
		t:       t.t,
		Address: *address,
		Balance: *balance,
	})
}

func (b *testBlockchain) Prepare(t *testing.T, testClasses []TestClass) {
	t.Helper()

	// This have to remain the same due to compatibility with old code
	// due to compatibility with old code
	// This should be class hash to class
	junoClasses := make(map[felt.Felt]core.Class, len(testClasses))
	// This should be: contract address to class hash
	deployedContracts := make(map[felt.Felt]*felt.Felt) // use pointer because of DeployedContracts field
	// This should be storage key to balance
	balances := make(map[felt.Felt]*felt.Felt) // use pointer because of StorageDiff field

	for i := range testClasses {
		class := &testClasses[i]
		junoClasses[*class.Hash.AsFelt()] = &class.JunoClass
		for _, account := range class.Accounts {
			deployedContracts[*account.Address.AsFelt()] = class.Hash.AsFelt()
			b.ClassHashes[account.Address] = class.Hash
			if balance := account.Balance; balance != felt.Zero {
				balances[account.BalanceKey()] = &balance
			}
		}
	}

	height, err := b.Height()
	require.NoError(t, err)
	parentBlock, err := b.BlockByNumber(height)
	require.NoError(t, err)

	stateUpdate := &core.StateUpdate{
		OldRoot: parentBlock.GlobalStateRoot,
		StateDiff: &core.StateDiff{
			DeployedContracts: deployedContracts,
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				*b.erc20.Accounts[0].Address.AsFelt(): balances,
				*b.erc20.Accounts[1].Address.AsFelt(): balances,
			},
		},
	}

	stateUpdate.NewRoot = b.NewRoot(t, parentBlock.Number+1, stateUpdate, junoClasses)

	block := &core.Block{
		Header: &core.Header{
			Hash:             utils.HexTo[felt.Felt](t, "0xb01"),
			ParentHash:       parentBlock.Hash,
			Number:           parentBlock.Number + 1,
			GlobalStateRoot:  stateUpdate.NewRoot,
			SequencerAddress: utils.HexToFelt(t, "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
			Timestamp:        parentBlock.Timestamp + 1,
			ProtocolVersion:  parentBlock.ProtocolVersion,
			L1GasPriceETH:    utils.HexToFelt(t, "0x1"),
			L1GasPriceSTRK:   utils.HexToFelt(t, "0x2"),
			L1DAMode:         core.Blob,
			L1DataGasPrice: &core.GasPrice{
				PriceInWei: utils.HexToFelt(t, "0x2"),
				PriceInFri: utils.HexToFelt(t, "0x2"),
			},
			L2GasPrice: &core.GasPrice{
				PriceInWei: utils.HexToFelt(t, "0x1"),
				PriceInFri: utils.HexToFelt(t, "0x1"),
			},
		},
	}

	require.NoError(t, b.Store(block, &core.BlockCommitments{}, stateUpdate, junoClasses))
}

func classFromFile(t *testing.T, path string) (
	starknet.SierraDefinition, starknet.CompiledClass, core.Cairo1Class,
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
	return snSierra, *snCasm, *junoClass
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
