package blockchain

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

type testBlockchain struct {
	*Blockchain
	t                        *testing.T
	account, deployer, erc20 *TestClass
	addr2classHash           map[felt.Felt]*felt.Felt
}

func NewTestBlockchain(t *testing.T, protocolVersion string) *testBlockchain {
	t.Helper()

	testDB := pebble.NewMemTest(t)
	chain := testBlockchain{
		Blockchain:     New(testDB, &utils.Sepolia),
		t:              t,
		addr2classHash: make(map[felt.Felt]*felt.Felt),
	}

	genesisBlock := &core.Block{
		Header: &core.Header{
			Number:          0,
			Timestamp:       0,
			ProtocolVersion: protocolVersion,
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

	// Predeploy presets
	// https://github.com/OpenZeppelin/cairo-contracts/tree/main/packages/presets
	chain.account = NewClass(t, "../../cairo/target/dev/juno_AccountUpgradeable.contract_class.json")
	chain.account.AddAccount(
		utils.HexToFelt(t, "0xc01"),
		utils.HexToFelt(t, "0x10000000000000000000000000000"),
	)
	chain.deployer = NewClass(t, "../../cairo/target/dev/juno_UniversalDeployer.contract_class.json")
	chain.deployer.AddAccount(
		utils.HexToFelt(t, "0xc02"),
		nil,
	)
	chain.erc20 = NewClass(t, "../../cairo/target/dev/juno_ERC20Upgradeable.contract_class.json")
	chain.erc20.AddAccount(
		utils.HexToFelt(t, "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
		nil,
	)
	chain.erc20.AddAccount(
		utils.HexToFelt(t, "0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d"),
		nil,
	)

	chain.PredeployContracts(t, []*TestClass{chain.account, chain.deployer, chain.erc20})

	return &chain
}

func (b *testBlockchain) AccountAddress() *felt.Felt {
	return b.account.accounts[0].address
}

func (b *testBlockchain) AccountClassHash() *felt.Felt {
	return b.account.hash
}

func (b *testBlockchain) ClassHashByAddress(address *felt.Felt) *felt.Felt {
	classHash, ok := b.addr2classHash[*address]
	require.True(b.t, ok)
	return classHash
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
	address *felt.Felt
	balance *felt.Felt
}

func (a *testAccount) BalanceKey() *felt.Felt {
	return fromNameAndKey(a.t, "ERC20_balances", a.address)
}

type TestClass struct {
	t        *testing.T
	hash     *felt.Felt
	class    *core.Cairo1Class
	accounts []*testAccount
}

func NewClass(t *testing.T, path string) *TestClass {
	t.Helper()

	_, _, class := classFromFile(t, path)
	classHash, err := class.Hash()
	require.NoError(t, err)

	return &TestClass{
		t:     t,
		class: class,
		hash:  classHash,
	}
}

func (b *TestClass) AddAccount(address, balance *felt.Felt) {
	b.accounts = append(b.accounts, &testAccount{
		t:       b.t,
		address: address,
		balance: balance,
	})
}

func (b *testBlockchain) PredeployContracts(t *testing.T, classes []*TestClass) {
	t.Helper()

	newClasses := make(map[felt.Felt]core.Class, len(classes))
	deployedContracts := make(map[felt.Felt]*felt.Felt)
	balances := make(map[felt.Felt]*felt.Felt)

	for _, class := range classes {
		newClasses[*class.hash] = class.class
		for _, account := range class.accounts {
			deployedContracts[*account.address] = class.hash
			b.addr2classHash[*account.address] = class.hash
			if balance := account.balance; balance != nil {
				balances[*account.BalanceKey()] = balance
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
				*b.erc20.accounts[0].address: balances,
				*b.erc20.accounts[1].address: balances,
			},
		},
	}

	stateUpdate.NewRoot = b.NewRoot(t, parentBlock.Number+1, stateUpdate, newClasses)

	block := &core.Block{
		Header: &core.Header{
			Hash:             utils.HexToFelt(t, "0xb01"),
			ParentHash:       parentBlock.Hash,
			Number:           parentBlock.Number + 1,
			GlobalStateRoot:  stateUpdate.NewRoot,
			SequencerAddress: utils.HexToFelt(t, "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
			Timestamp:        parentBlock.Timestamp + 1,
			ProtocolVersion:  parentBlock.ProtocolVersion,
			L1GasPriceETH:    utils.HexToFelt(t, "0x1"),
			L1GasPriceSTRK:   utils.HexToFelt(t, "0x2"),
			L1DAMode:         core.Blob,
			L1DataGasPrice:   &core.GasPrice{PriceInWei: utils.HexToFelt(t, "0x2"), PriceInFri: utils.HexToFelt(t, "0x2")},
			L2GasPrice:       &core.GasPrice{PriceInWei: utils.HexToFelt(t, "0x1"), PriceInFri: utils.HexToFelt(t, "0x1")},
		},
	}

	require.NoError(t, b.Store(block, &core.BlockCommitments{}, stateUpdate, newClasses))
}

func classFromFile(t *testing.T, path string) (*starknet.SierraDefinition, *starknet.CompiledClass, *core.Cairo1Class) {
	t.Helper()

	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	snClass := new(starknet.SierraDefinition)
	require.NoError(t, json.NewDecoder(file).Decode(snClass))

	compliedClass, err := compiler.Compile(snClass)
	require.NoError(t, err)

	class, err := sn2core.AdaptCairo1Class(snClass, compliedClass)
	require.NoError(t, err)

	return snClass, compliedClass, class
}

// https://github.com/eqlabs/pathfinder/blob/7664cba5145d8100ba1b6b2e2980432bc08d72a2/crates/common/src/lib.rs#L124
func fromNameAndKey(t *testing.T, name string, key *felt.Felt) *felt.Felt {
	t.Helper()

	intermediate := crypto.StarknetKeccak([]byte(name))
	byteArr := crypto.Pedersen(intermediate, key).Bytes()
	value := new(big.Int).SetBytes(byteArr[:])

	maxAddr, ok := new(big.Int).SetString("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00", 0)
	require.True(t, ok)

	value = value.Rem(value, maxAddr)

	return new(felt.Felt).SetBigInt(value)
}
