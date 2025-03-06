package testutils

import (
	"encoding/json"
	"fmt"
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

func TestBlockchain(t *testing.T, protocolVersion string) (*blockchain.Blockchain, *felt.Felt, *felt.Felt) {
	t.Helper()

	testDB := pebble.NewMemTest(t)
	chain := blockchain.New(testDB, &utils.Sepolia)

	genesis := &core.Block{
		Header: &core.Header{
			Number:          0,
			Timestamp:       0,
			ProtocolVersion: protocolVersion,
			Hash:            utils.HexToFelt(t, "0xb00"),
			ParentHash:      &felt.Zero,
		},
	}
	genesisStateUpdate := &core.StateUpdate{
		BlockHash: genesis.Hash,
		NewRoot:   &felt.Zero,
		OldRoot:   &felt.Zero,
		StateDiff: &core.StateDiff{},
	}

	require.NoError(t, chain.Store(genesis, &core.BlockCommitments{}, genesisStateUpdate, nil))

	accountAddr := utils.HexToFelt(t, "0xc01")
	_, _, accountClass := ClassFromFile(t, "../../cairo/account/target/dev/account_AccountUpgradeable.contract_class.json")
	accountClassHash, err := accountClass.Hash()
	require.NoError(t, err)

	deployerAddr := utils.HexToFelt(t, "0xc02")
	_, _, delployerClass := ClassFromFile(
		t,
		"../../cairo/universal_deployer/target/dev/universal_deployer_UniversalDeployer.contract_class.json",
	)
	delployerClassHash, err := delployerClass.Hash()
	require.NoError(t, err)

	ethFeeTokenAddr := utils.HexToFelt(t, "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7")
	strkFeeTokenAddr := utils.HexToFelt(t, "0x04718f5a0fc34cc1af16a1cdee98ffb20c31f5cd61d6ab07201858f4287c938d")
	_, _, erc20Class := ClassFromFile(t, "../../cairo/erc20/target/dev/erc20_ERC20Upgradeable.contract_class.json")
	erc20ClassHash, err := erc20Class.Hash()
	require.NoError(t, err)

	accountBalanceKey := fromNameAndKey(t, "ERC20_balances", accountAddr)
	fmt.Printf("accountBalanceKey: %s\n", accountBalanceKey.String())

	stateUpdate := &core.StateUpdate{
		OldRoot: &felt.Zero,
		NewRoot: utils.HexToFelt(t, "0x5403e8bee8d88c4a36879d7236988aeb0b2eb62df16426150181df76b5872af"),
		StateDiff: &core.StateDiff{
			DeployedContracts: map[felt.Felt]*felt.Felt{
				*accountAddr:      accountClassHash,
				*deployerAddr:     delployerClassHash,
				*ethFeeTokenAddr:  erc20ClassHash,
				*strkFeeTokenAddr: erc20ClassHash,
			},
			StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
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
			ParentHash:       genesis.Hash,
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

	return chain, accountAddr, deployerAddr
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
