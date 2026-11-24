package rpctest_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/rpc/rpctest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const version = rpctest.Version0_10

var strk = networks.DefaultFeeTokenAddresses.StrkL2TokenAddress

func strkBalance(t *testing.T, node *rpctest.Node, owner *felt.Address) *big.Int {
	t.Helper()
	balanceOf := crypto.StarknetKeccak([]byte("balanceOf"))
	result := node.Result(t, version, "starknet_call", fmt.Sprintf(
		`{"request":{"contract_address":%q,"entry_point_selector":%q,"calldata":[%q]},`+
			`"block_id":"latest"}`,
		strk.String(), balanceOf.String(), owner.String(),
	))
	var words []string
	require.NoError(t, json.Unmarshal(result, &words))
	require.Len(t, words, 2) // u256: low, high
	low, ok := new(big.Int).SetString(words[0], 0)
	require.True(t, ok)
	high, ok := new(big.Int).SetString(words[1], 0)
	require.True(t, ok)
	return high.Lsh(high, 128).Add(high, low)
}

func classHashAt(t *testing.T, node *rpctest.Node, address *felt.Address) felt.ClassHash {
	t.Helper()
	var classHash felt.ClassHash
	result := node.Result(t, version, "starknet_getClassHashAt",
		fmt.Sprintf(`{"block_id":"latest","contract_address":%q}`, address.String()))
	require.NoError(t, json.Unmarshal(result, &classHash))
	return classHash
}

func mustSucceed(t *testing.T, node *rpctest.Node, hash *felt.TransactionHash) *big.Int {
	t.Helper()
	var receipt struct {
		ExecutionStatus string `json:"execution_status"`
		ActualFee       struct {
			Amount string `json:"amount"`
			Unit   string `json:"unit"`
		} `json:"actual_fee"`
	}
	result := node.Result(t, version, "starknet_getTransactionReceipt",
		fmt.Sprintf(`{"transaction_hash":%q}`, hash.String()))
	require.NoError(t, json.Unmarshal(result, &receipt))
	require.Equal(t, "SUCCEEDED", receipt.ExecutionStatus)
	require.Equal(t, "FRI", receipt.ActualFee.Unit)
	fee, ok := new(big.Int).SetString(receipt.ActualFee.Amount, 0)
	require.True(t, ok)
	return fee
}

type blockHeader struct {
	BlockNumber uint64 `json:"block_number"`
	BlockHash   string `json:"block_hash"`
	L1GasPrice  struct {
		PriceInFri string `json:"price_in_fri"`
	} `json:"l1_gas_price"`
	L2GasPrice struct {
		PriceInFri string `json:"price_in_fri"`
	} `json:"l2_gas_price"`
	L1DataGasPrice struct {
		PriceInFri string `json:"price_in_fri"`
	} `json:"l1_data_gas_price"`
}

func getBlock(t *testing.T, node *rpctest.Node, blockID string) blockHeader {
	t.Helper()
	var header blockHeader
	result := node.Result(t, version, "starknet_getBlockWithTxHashes", `{"block_id":`+blockID+`}`)
	require.NoError(t, json.Unmarshal(result, &header))
	return header
}

func TestNewDevnet(t *testing.T) {
	t.Parallel()
	devnet := rpctest.NewDevnet(t)

	latest := getBlock(t, devnet.Node, `"latest"`)
	assert.Equal(t, uint64(1), latest.BlockNumber)
	assert.Equal(t, "0x1", latest.L1GasPrice.PriceInFri)
	assert.Equal(t, "0x1", latest.L2GasPrice.PriceInFri)
	assert.Equal(t, "0x1", latest.L1DataGasPrice.PriceInFri)
	assert.Equal(t, uint64(2), getBlock(t, devnet.Node, `"pre_confirmed"`).BlockNumber)

	// genesis_prefund_accounts.json mints this amount of STRK to 0x101, and no STRK to 0x102.
	funded := devnet.Account(t, "0x101")
	assert.Equal(t, "0x66bc75e2d63100000", "0x"+strkBalance(t, devnet.Node, &funded.Address).Text(16))
	assert.Zero(t, strkBalance(t, devnet.Node, &devnet.Account(t, "0x102").Address).Sign())

	assert.Equal(t, rpctest.AccountClassHash, classHashAt(t, devnet.Node, &funded.Address))
	assert.Equal(t, rpctest.L1HandlerClassHash, classHashAt(t, devnet.Node, &rpctest.L1HandlerAddress))
}

func TestDevnetMakeBlock(t *testing.T) {
	t.Parallel()

	t.Run("a transfer pays its fee to the sequencer", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		sender := devnet.Account(t, "0x101")
		recipient := felt.FromUint64[felt.Address](0x105)
		senderBefore := strkBalance(t, devnet.Node, &sender.Address)
		sequencerBefore := strkBalance(t, devnet.Node, &rpctest.SequencerAddress)

		transfer := sender.InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000))
		devnet.Submit(t, transfer)
		header := devnet.MakeBlock(t)
		assert.Equal(t, uint64(2), header.Number)
		assert.Equal(t, uint64(1), header.TransactionCount)

		fee := mustSucceed(t, devnet.Node, &transfer.Hash)
		assert.Positive(t, fee.Sign())
		spent := new(big.Int).Sub(senderBefore, strkBalance(t, devnet.Node, &sender.Address))
		assert.Equal(t, new(big.Int).Add(big.NewInt(1000), fee), spent)
		assert.Equal(t, big.NewInt(1000), strkBalance(t, devnet.Node, &recipient))
		sequencerAfter := strkBalance(t, devnet.Node, &rpctest.SequencerAddress)
		assert.Equal(t, fee, new(big.Int).Sub(sequencerAfter, sequencerBefore))

		nonce := devnet.Result(t, version, "starknet_getNonce",
			fmt.Sprintf(`{"block_id":"latest","contract_address":%q}`, sender.Address.String()))
		assert.JSONEq(t, `"0x1"`, string(nonce))
	})

	t.Run("a new account deploys itself", func(t *testing.T) {
		t.Parallel()
		devnet := rpctest.NewDevnet(t)
		account := devnet.NewAccount(t)
		funder := devnet.Account(t, "0x101")
		devnet.Submit(t, funder.InvokeV3(t, 0, rpctest.TransferSTRK(&account.Address, 1e15)))
		devnet.MakeBlock(t)

		deploy := account.DeployAccountV3(t)
		devnet.Submit(t, deploy)
		devnet.MakeBlock(t)
		mustSucceed(t, devnet.Node, &deploy.Hash)
		assert.Equal(t, rpctest.AccountClassHash, classHashAt(t, devnet.Node, &account.Address))
	})

	t.Run("each devnet has its own chain", func(t *testing.T) {
		t.Parallel()
		first := rpctest.NewDevnet(t)
		second := rpctest.NewDevnet(t)
		recipient := felt.FromUint64[felt.Address](0x105)

		first.Submit(t, first.Account(t, "0x101").InvokeV3(t, 0, rpctest.TransferSTRK(&recipient, 1000)))
		first.MakeBlock(t)

		assert.Equal(t, uint64(2), getBlock(t, first.Node, `"latest"`).BlockNumber)
		assert.Equal(t, uint64(1), getBlock(t, second.Node, `"latest"`).BlockNumber)
		assert.Zero(t, strkBalance(t, second.Node, &recipient).Sign())
	})
}

func TestNodeServesEachVersion(t *testing.T) {
	t.Parallel()
	devnet := rpctest.NewDevnet(t)
	tests := []struct {
		version rpctest.Version
		prefix  string
	}{
		{version: rpctest.Version0_9, prefix: "0.9."},
		{version: rpctest.Version0_10, prefix: "0.10."},
	}
	for _, test := range tests {
		t.Run(string(test.version), func(t *testing.T) {
			t.Parallel()
			var specVersion string
			result := devnet.Result(t, test.version, "starknet_specVersion", `[]`)
			require.NoError(t, json.Unmarshal(result, &specVersion))
			assert.Truef(t, strings.HasPrefix(specVersion, test.prefix), "spec version %s", specVersion)
		})
	}
}

func TestNewSepolia(t *testing.T) {
	t.Parallel()

	t.Run("the node holds the fixture blocks", func(t *testing.T) {
		t.Parallel()
		sepolia := rpctest.NewSepolia(t, 4)
		latest := getBlock(t, sepolia, `"latest"`)
		assert.Equal(t, uint64(3), latest.BlockNumber)
		// The hash of Sepolia block 3, from clients/feeder/testdata/sepolia/block/3.json.
		const block3Hash = "0x37644818236ee05b7e3b180bed64ea70ee3dd1553ca334a5c2a290ee276f380"
		assert.Equal(t, block3Hash, latest.BlockHash)
	})

	t.Run("a node with no blocks", func(t *testing.T) {
		t.Parallel()
		response := rpctest.NewSepolia(t, 0).Call(t, version, "starknet_blockNumber", `[]`)
		require.JSONEq(t, `{"code":32,"message":"There are no blocks"}`, string(response.Error))
	})
}
