package testutils

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/starknet/compiler"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestBlockchain(t *testing.T, protocolVersion string) (
	*blockchain.Blockchain, *felt.Felt, *felt.Felt, *felt.Felt, *felt.Felt,
) {
	t.Helper()

	testDB := pebble.NewMemTest(t)
	chain := blockchain.New(testDB, &utils.Sepolia)

	genesisBlock := &core.Block{
		Header: &core.Header{
			Number:          0,
			Timestamp:       0,
			ProtocolVersion: protocolVersion,
			Hash:            utils.HexToFelt(t, "0xb00"),
			ParentHash:      &felt.Zero,
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

	accountAddr := utils.HexToFelt(t, "0xc01")
	_, _, accountClass := ClassFromFile(t, "../../cairo/target/dev/juno_AccountUpgradeable.contract_class.json")
	accountClassHash, err := accountClass.Hash()
	require.NoError(t, err)

	deployerAddr := utils.HexToFelt(t, "0xc02")
	_, _, delployerClass := ClassFromFile(
		t,
		"../../cairo/target/dev/juno_UniversalDeployer.contract_class.json",
	)
	delployerClassHash, err := delployerClass.Hash()
	require.NoError(t, err)

	// https://docs.starknet.io/resources/chain-info/
	ethFeeTokenAddr := utils.HexToFelt(t, "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
	strkFeeTokenAddr := utils.HexToFelt(t, "0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d")

	_, _, erc20Class := ClassFromFile(t, "../../cairo/target/dev/juno_ERC20Upgradeable.contract_class.json")
	erc20ClassHash, err := erc20Class.Hash()
	require.NoError(t, err)

	accountBalanceKey := fromNameAndKey(t, "ERC20_balances", accountAddr)

	stateUpdate := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: utils.HexToFelt(t, "0x7453641d480b482d1613edd0e760c265bd010594aa493b801850306f3e98773"),
		StateDiff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{
				*accountAddr:      accountClassHash,
				*deployerAddr:     delployerClassHash,
				*ethFeeTokenAddr:  erc20ClassHash,
				*strkFeeTokenAddr: erc20ClassHash,
			},
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
				// Set the initial balance of the account
				*ethFeeTokenAddr: {
					*accountBalanceKey: utils.HexToFelt(t, "0x10000000000000000000000000000"),
				},
				*strkFeeTokenAddr: {
					*accountBalanceKey: utils.HexToFelt(t, "0x10000000000000000000000000000"),
				},
			},
		},
	}
	newClasses := map[felt.Felt]core.Class{
		*accountClassHash:   accountClass,
		*delployerClassHash: delployerClass,
		*erc20ClassHash:     erc20Class,
	}
	block := &core.Block{
		Header: &core.Header{
			Hash:             utils.HexToFelt(t, "0xb01"),
			ParentHash:       genesisBlock.Hash,
			Number:           1,
			GlobalStateRoot:  stateUpdate.NewRoot,
			SequencerAddress: utils.HexToFelt(t, "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"),
			Timestamp:        1,
			ProtocolVersion:  protocolVersion,
			L1GasPriceETH:    utils.HexToFelt(t, "0x1"),
			L1GasPriceSTRK:   utils.HexToFelt(t, "0x2"),
			L1DAMode:         core.Blob,
			L1DataGasPrice:   &core.GasPrice{PriceInWei: utils.HexToFelt(t, "0x2"), PriceInFri: utils.HexToFelt(t, "0x2")},
			L2GasPrice:       &core.GasPrice{PriceInWei: utils.HexToFelt(t, "0x1"), PriceInFri: utils.HexToFelt(t, "0x1")},
		},
	}
	require.NoError(t, chain.Store(block, &core.BlockCommitments{}, stateUpdate, newClasses))

	return chain, accountAddr, accountClassHash, deployerAddr, delployerClassHash
}

func ClassFromFile(t *testing.T, path string) (*starknet.SierraDefinition, *starknet.CompiledClass, *core.Cairo1Class) {
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
