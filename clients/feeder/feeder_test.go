package feeder_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	ua = "Juno/v0.0.1-test Starknet Implementation"
)

func TestDeclareTransactionUnmarshal(t *testing.T) {
	t.Run("pre-v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Mainnet)
		txnHash := felt.NewUnsafeFromString[felt.Felt](
			"0x93f542728e403f1edcea4a41f1509a39be35ebcad7d4b5aa77623e5e6480d",
		)
		status, err := client.Transaction(t.Context(), txnHash)
		require.NoError(t, err)

		declareTx := status.Transaction
		assert.Equal(
			t,
			"0x93f542728e403f1edcea4a41f1509a39be35ebcad7d4b5aa77623e5e6480d",
			declareTx.Hash.String(),
		)
		assert.Equal(t, "0x1", declareTx.Version.String())
		assert.Equal(t, "0x5af3107a4000", declareTx.MaxFee.String())
		assert.Equal(t, "0x1d", declareTx.Nonce.String())
		assert.Equal(
			t,
			"0x2ed6bb4d57ad27a22972b81feb9d09798ff8c273684376ec72c154d90343453",
			declareTx.ClassHash.String(),
		)
		assert.Equal(
			t,
			"0xb8a60857ed233885155f1d839086ca7ad03e6d4237cc10b085a4652a61a23",
			declareTx.SenderAddress.String(),
		)
		assert.Equal(t, starknet.TxnDeclare, declareTx.Type)
		assert.Equal(t, 2, len(*declareTx.Signature))
		assert.Equal(
			t,
			"0x516b5999b47509105675dd4c6ed9c373448038cfd00549fe868695916eee0ff",
			(*declareTx.Signature)[0].String(),
		)
		assert.Equal(
			t,
			"0x6c0189aaa56bfcb2a3e97198d04bd7a9750a4354b88f4e5edf57cf4d966ddda",
			(*declareTx.Signature)[1].String(),
		)
	})

	t.Run("v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Integration)
		txnHash := felt.NewUnsafeFromString[felt.Felt](
			"0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3",
		)
		status, err := client.Transaction(t.Context(), txnHash)
		require.NoError(t, err)

		require.Equal(t, &starknet.Transaction{
			Hash: felt.NewUnsafeFromString[felt.Felt](
				"0x41d1f5206ef58a443e7d3d1ca073171ec25fa75313394318fc83a074a6631c3",
			),
			Version: felt.NewUnsafeFromString[felt.Felt]("0x3"),
			Signature: &[]*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt](
					"0x29a49dff154fede73dd7b5ca5a0beadf40b4b069f3a850cd8428e54dc809ccc",
				),
				felt.NewUnsafeFromString[felt.Felt](
					"0x429d142a17223b4f2acde0f5ecb9ad453e188b245003c86fab5c109bad58fc3",
				),
			},
			Nonce:       new(felt.Felt).SetUint64(1),
			NonceDAMode: utils.HeapPtr(starknet.DAModeL1),
			FeeDAMode:   utils.HeapPtr(starknet.DAModeL1),
			ResourceBounds: &map[starknet.Resource]starknet.ResourceBounds{
				starknet.ResourceL1Gas: {
					MaxAmount:       felt.NewUnsafeFromString[felt.Felt]("0x186a0"),
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x2540be400"),
				},
				starknet.ResourceL1DataGas: {
					MaxAmount:       felt.NewUnsafeFromString[felt.Felt]("0x186a0"),
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x2540be400"),
				},
				starknet.ResourceL2Gas: {
					MaxAmount:       new(felt.Felt),
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:           new(felt.Felt),
			PaymasterData: &[]*felt.Felt{},
			SenderAddress: felt.NewUnsafeFromString[felt.Felt](
				"0x2fab82e4aef1d8664874e1f194951856d48463c3e6bf9a8c68e234a629a6f50",
			),
			ClassHash: felt.NewUnsafeFromString[felt.Felt](
				"0x5ae9d09292a50ed48c5930904c880dab56e85b825022a7d689cfc9e65e01ee7",
			),
			CompiledClassHash: felt.NewUnsafeFromString[felt.Felt](
				"0x1add56d64bebf8140f3b8a38bdf102b7874437f0c861ab4ca7526ec33b4d0f8",
			),
			AccountDeploymentData: &[]*felt.Felt{},
			Type:                  starknet.TxnDeclare,
		}, status.Transaction)
	})
}

func TestInvokeTransactionUnmarshal(t *testing.T) {
	t.Run("pre-v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Mainnet)

		txnHash := felt.NewUnsafeFromString[felt.Felt](
			"0x631333277e88053336d8c302630b4420dc3ff24018a1c464da37d5e36ea19df",
		)
		status, err := client.Transaction(t.Context(), txnHash)
		require.NoError(t, err)

		invokeTx := status.Transaction
		assert.Equal(
			t,
			"0x631333277e88053336d8c302630b4420dc3ff24018a1c464da37d5e36ea19df",
			invokeTx.Hash.String(),
		)
		assert.Equal(t, "0x0", invokeTx.Version.String())
		assert.Equal(t, "0x0", invokeTx.MaxFee.String())
		assert.Equal(t, 0, len(*invokeTx.Signature))
		assert.Equal(
			t,
			"0x17daeb497b6fe0f7adaa32b44677c3a9712b6856b792ad993fcef20aed21ac8",
			invokeTx.ContractAddress.String(),
		)
		assert.Equal(
			t,
			"0x218f305395474a84a39307fa5297be118fe17bf65e27ac5e2de6617baa44c64",
			invokeTx.EntryPointSelector.String(),
		)
		assert.Equal(t, 2, len(*invokeTx.CallData))
		assert.Equal(
			t,
			"0x346f2b6376b4b57f714ba187716fce9edff1361628cc54783ed0351538faa5e",
			(*invokeTx.CallData)[0].String(),
		)
		assert.Equal(t, "0x2", (*invokeTx.CallData)[1].String())
		assert.Equal(t, starknet.TxnInvoke, invokeTx.Type)
	})

	t.Run("v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Integration)
		txnHash := felt.NewUnsafeFromString[felt.Felt](
			"0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd",
		)
		status, err := client.Transaction(t.Context(), txnHash)
		require.NoError(t, err)

		require.Equal(t, &starknet.Transaction{
			Hash: felt.NewUnsafeFromString[felt.Felt](
				"0x49728601e0bb2f48ce506b0cbd9c0e2a9e50d95858aa41463f46386dca489fd",
			),
			Version: new(felt.Felt).SetUint64(3),
			Signature: &[]*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt](
					"0x71a9b2cd8a8a6a4ca284dcddcdefc6c4fd20b92c1b201bd9836e4ce376fad16",
				),
				felt.NewUnsafeFromString[felt.Felt](
					"0x6bef4745194c9447fdc8dd3aec4fc738ab0a560b0d2c7bf62fbf58aef3abfc5",
				),
			},
			Nonce:       felt.NewUnsafeFromString[felt.Felt]("0xe97"),
			NonceDAMode: utils.HeapPtr(starknet.DAModeL1),
			FeeDAMode:   utils.HeapPtr(starknet.DAModeL1),
			ResourceBounds: &map[starknet.Resource]starknet.ResourceBounds{
				starknet.ResourceL1Gas: {
					MaxAmount:       felt.NewUnsafeFromString[felt.Felt]("0x186a0"),
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x5af3107a4000"),
				},
				starknet.ResourceL1DataGas: {
					MaxAmount:       felt.NewUnsafeFromString[felt.Felt]("0x186a0"),
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x5af3107a4000"),
				},
				starknet.ResourceL2Gas: {
					MaxAmount:       new(felt.Felt),
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:           new(felt.Felt),
			PaymasterData: &[]*felt.Felt{},
			SenderAddress: felt.NewUnsafeFromString[felt.Felt](
				"0x3f6f3bc663aedc5285d6013cc3ffcbc4341d86ab488b8b68d297f8258793c41",
			),
			CallData: &[]*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt]("0x2"),
				felt.NewUnsafeFromString[felt.Felt](
					"0x450703c32370cf7ffff540b9352e7ee4ad583af143a361155f2b485c0c39684",
				),
				felt.NewUnsafeFromString[felt.Felt](
					"0x27c3334165536f239cfd400ed956eabff55fc60de4fb56728b6a4f6b87db01c",
				),
				felt.NewUnsafeFromString[felt.Felt]("0x0"),
				felt.NewUnsafeFromString[felt.Felt]("0x4"),
				felt.NewUnsafeFromString[felt.Felt](
					"0x4c312760dfd17a954cdd09e76aa9f149f806d88ec3e402ffaf5c4926f568a42",
				),
				felt.NewUnsafeFromString[felt.Felt](
					"0x5df99ae77df976b4f0e5cf28c7dcfe09bd6e81aab787b19ac0c08e03d928cf",
				),
				felt.NewUnsafeFromString[felt.Felt]("0x4"),
				felt.NewUnsafeFromString[felt.Felt]("0x1"),
				felt.NewUnsafeFromString[felt.Felt]("0x5"),
				felt.NewUnsafeFromString[felt.Felt](
					"0x450703c32370cf7ffff540b9352e7ee4ad583af143a361155f2b485c0c39684",
				),
				felt.NewUnsafeFromString[felt.Felt](
					"0x5df99ae77df976b4f0e5cf28c7dcfe09bd6e81aab787b19ac0c08e03d928cf",
				),
				felt.NewUnsafeFromString[felt.Felt]("0x1"),
				felt.NewUnsafeFromString[felt.Felt](
					"0x7fe4fd616c7fece1244b3616bb516562e230be8c9f29668b46ce0369d5ca829",
				),
				felt.NewUnsafeFromString[felt.Felt](
					"0x287acddb27a2f9ba7f2612d72788dc96a5b30e401fc1e8072250940e024a587",
				),
			},
			AccountDeploymentData: &[]*felt.Felt{},
			Type:                  starknet.TxnInvoke,
		}, status.Transaction)
	})
}

//nolint:dupl
func TestDeployTransactionUnmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	txnHash := felt.NewUnsafeFromString[felt.Felt](
		"0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee",
	)
	status, err := client.Transaction(t.Context(), txnHash)
	require.NoError(t, err)

	deployTx := status.Transaction
	assert.Equal(
		t,
		"0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee",
		deployTx.Hash.String(),
	)
	assert.Equal(t, "0x0", deployTx.Version.String())
	assert.Equal(
		t,
		"0x7cc55b21de4b7d6d7389df3b27de950924ac976d263ac8d71022d0b18155fc",
		deployTx.ContractAddress.String(),
	)
	assert.Equal(
		t,
		"0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8",
		deployTx.ContractAddressSalt.String(),
	)
	assert.Equal(
		t,
		"0x3131fa018d520a037686ce3efddeab8f28895662f019ca3ca18a626650f7d1e",
		deployTx.ClassHash.String(),
	)
	assert.Equal(t, 4, len(*deployTx.ConstructorCallData))
	assert.Equal(
		t,
		"0x69577e6756a99b584b5d1ce8e60650ae33b6e2b13541783458268f07da6b38a",
		(*deployTx.ConstructorCallData)[0].String(),
	)
	assert.Equal(
		t,
		"0x2dd76e7ad84dbed81c314ffe5e7a7cacfb8f4836f01af4e913f275f89a3de1a",
		(*deployTx.ConstructorCallData)[1].String(),
	)
	assert.Equal(t, "0x1", (*deployTx.ConstructorCallData)[2].String())
	assert.Equal(
		t,
		"0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8",
		(*deployTx.ConstructorCallData)[3].String(),
	)
	assert.Equal(t, starknet.TxnDeploy, deployTx.Type)
}

func TestDeployAccountTransactionUnmarshal(t *testing.T) {
	t.Run("pre-v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Mainnet)

		txnHash := felt.NewUnsafeFromString[felt.Felt](
			"0x32b272b6d0d584305a460197aa849b5c7a9a85903b66e9d3e1afa2427ef093e",
		)
		status, err := client.Transaction(t.Context(), txnHash)
		require.NoError(t, err)

		deployTx := status.Transaction
		assert.Equal(
			t,
			"0x32b272b6d0d584305a460197aa849b5c7a9a85903b66e9d3e1afa2427ef093e",
			deployTx.Hash.String(),
		)
		assert.Equal(t, "0x1", deployTx.Version.String())
		assert.Equal(t, "0x59f5f9f474b0", deployTx.MaxFee.String())
		assert.Equal(t, 2, len(*deployTx.Signature))
		assert.Equal(
			t,
			"0x467ae89bbbbaa0139e8f8a02ddc614bd80252998f3c033239f59f9f2ab973c5",
			(*deployTx.Signature)[0].String(),
		)
		assert.Equal(
			t,
			"0x92938929b5afcd596d651a6d28ed38baf90b000192897617d98de19d475331",
			(*deployTx.Signature)[1].String(),
		)
		assert.Equal(t, "0x0", deployTx.Nonce.String())
		assert.Equal(
			t,
			"0x104714313388bd0ab569ac247fed6cf0b7a2c737105c00d64c23e24bd8dea40",
			deployTx.ContractAddress.String(),
		)
		assert.Equal(
			t,
			"0x25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3",
			deployTx.ContractAddressSalt.String(),
		)
		assert.Equal(
			t,
			"0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918",
			deployTx.ClassHash.String(),
		)

		assert.Equal(t, 5, len(*deployTx.ConstructorCallData))
		assert.Equal(
			t,
			"0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2",
			(*deployTx.ConstructorCallData)[0].String(),
		)
		assert.Equal(
			t,
			"0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463",
			(*deployTx.ConstructorCallData)[1].String(),
		)
		assert.Equal(t, "0x2", (*deployTx.ConstructorCallData)[2].String())
		assert.Equal(
			t,
			"0x25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3",
			(*deployTx.ConstructorCallData)[3].String(),
		)
		assert.Equal(t, "0x0", (*deployTx.ConstructorCallData)[4].String())
		assert.Equal(t, starknet.TxnDeployAccount, deployTx.Type)
	})

	t.Run("v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Integration)
		txnHash := felt.NewUnsafeFromString[felt.Felt](
			"0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0",
		)
		status, err := client.Transaction(t.Context(), txnHash)
		require.NoError(t, err)

		require.Equal(t, &starknet.Transaction{
			Hash: felt.NewUnsafeFromString[felt.Felt](
				"0x29fd7881f14380842414cdfdd8d6c0b1f2174f8916edcfeb1ede1eb26ac3ef0",
			),
			Version: felt.NewUnsafeFromString[felt.Felt]("0x3"),
			ClassHash: felt.NewUnsafeFromString[felt.Felt](
				"0x2338634f11772ea342365abd5be9d9dc8a6f44f159ad782fdebd3db5d969738",
			),
			Signature: &[]*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt](
					"0x6d756e754793d828c6c1a89c13f7ec70dbd8837dfeea5028a673b80e0d6b4ec",
				),
				felt.NewUnsafeFromString[felt.Felt](
					"0x4daebba599f860daee8f6e100601d98873052e1c61530c630cc4375c6bd48e3",
				),
			},
			Nonce:       new(felt.Felt),
			NonceDAMode: utils.HeapPtr(starknet.DAModeL1),
			FeeDAMode:   utils.HeapPtr(starknet.DAModeL1),
			ResourceBounds: &map[starknet.Resource]starknet.ResourceBounds{
				starknet.ResourceL1Gas: {
					MaxAmount:       felt.NewUnsafeFromString[felt.Felt]("0x186a0"),
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x5af3107a4000"),
				},
				starknet.ResourceL1DataGas: {
					MaxAmount:       felt.NewUnsafeFromString[felt.Felt]("0x186a0"),
					MaxPricePerUnit: felt.NewUnsafeFromString[felt.Felt]("0x5af3107a4000"),
				},
				starknet.ResourceL2Gas: {
					MaxAmount:       new(felt.Felt),
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:           new(felt.Felt),
			PaymasterData: &[]*felt.Felt{},
			SenderAddress: felt.NewUnsafeFromString[felt.Felt](
				"0x2fab82e4aef1d8664874e1f194951856d48463c3e6bf9a8c68e234a629a6f50",
			),
			ContractAddressSalt: new(felt.Felt),
			ConstructorCallData: &[]*felt.Felt{
				felt.NewUnsafeFromString[felt.Felt](
					"0x5cd65f3d7daea6c63939d659b8473ea0c5cd81576035a4d34e52fb06840196c",
				),
			},
			Type: starknet.TxnDeployAccount,
		}, status.Transaction)
	})
}

//nolint:dupl
func TestL1HandlerTransactionUnmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	txnHash := felt.NewUnsafeFromString[felt.Felt](
		"0x218adbb5aea7985d67fe49b45d44a991380b63db41622f9f4adc36274d02190",
	)
	status, err := client.Transaction(t.Context(), txnHash)
	require.NoError(t, err)

	handlerTx := status.Transaction
	assert.Equal(
		t,
		"0x218adbb5aea7985d67fe49b45d44a991380b63db41622f9f4adc36274d02190",
		handlerTx.Hash.String(),
	)
	assert.Equal(t, "0x0", handlerTx.Version.String())
	assert.Equal(
		t,
		"0x73314940630fd6dcda0d772d4c972c4e0a9946bef9dabf4ef84eda8ef542b82",
		handlerTx.ContractAddress.String(),
	)
	assert.Equal(
		t,
		"0x2d757788a8d8d6f21d1cd40bce38a8222d70654214e96ff95d8086e684fbee5",
		handlerTx.EntryPointSelector.String(),
	)
	assert.Equal(t, "0x1654d", handlerTx.Nonce.String())
	assert.Equal(t, 4, len(*handlerTx.CallData))
	assert.Equal(
		t,
		"0xae0ee0a63a2ce6baeeffe56e7714fb4efe48d419",
		(*handlerTx.CallData)[0].String(),
	)
	assert.Equal(
		t,
		"0x218559e75713ca564d6eaf043b73388e9ac7c2f459ef8905988052051d3ef5e",
		(*handlerTx.CallData)[1].String(),
	)
	assert.Equal(t, "0x2386f26fc10000", (*handlerTx.CallData)[2].String())
	assert.Equal(t, "0x0", (*handlerTx.CallData)[3].String())
	assert.Equal(t, starknet.TxnL1Handler, handlerTx.Type)
}

func TestBlockWithoutSequencerAddressUnmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	block, err := client.Block(t.Context(), strconv.Itoa(11817))
	require.NoError(t, err)

	assert.Equal(
		t,
		"0x24c692acaed3b486990bd9d2b2fbbee802b37b3bd79c59f295bad3277200a83",
		block.Hash.String(),
	)
	assert.Equal(
		t,
		"0x3873ccb937f14429b169c654dda28886d2cc2d6ea17b3cff9748fe5cfdb67e0",
		block.ParentHash.String(),
	)
	assert.Equal(t, uint64(11817), block.Number)
	assert.Equal(
		t,
		"0x3df24be7b5fed6b41de08d38686b6142944119ca2a345c38793590d6804bba4",
		block.StateRoot.String(),
	)
	assert.Equal(t, "ACCEPTED_ON_L2", block.Status)
	assert.Equal(t, "0x27ad16775", block.L1GasPriceETH().String())
	assert.Equal(t, 52, len(block.Transactions))
	assert.Equal(t, 52, len(block.Receipts))
	assert.Equal(t, uint64(1669465009), block.Timestamp)
	assert.Equal(t, "0.10.1", block.Version)
}

func TestBlockWithSequencerAddressUnmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	block, err := client.Block(t.Context(), strconv.Itoa(19199))
	require.NoError(t, err)

	assert.Equal(
		t,
		"0x41811b69473f26503e0375806ee97d05951ccc7840e3d2bbe14ffb2522e5be1",
		block.Hash.String(),
	)
	assert.Equal(
		t,
		"0x68427fb6f1f5e687fbd779b3cc0d4ee31b49575ed0f8c749f827e4a45611efc",
		block.ParentHash.String(),
	)
	assert.Equal(t, uint64(19199), block.Number)
	assert.Equal(
		t,
		"0x541b796ea02703d02ff31459815f65f410ceefe80a4e3499f7ef9ccc36d26ee",
		block.StateRoot.String(),
	)
	assert.Equal(t, "ACCEPTED_ON_L2", block.Status)
	assert.Equal(t, "0x31c4e2d75", block.L1GasPriceETH().String())
	assert.Equal(t, 324, len(block.Transactions))
	assert.Equal(t, 324, len(block.Receipts))
	assert.Equal(t, uint64(1674728186), block.Timestamp)
	assert.Equal(t, "0.10.3", block.Version)
	assert.Equal(
		t,
		"0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9",
		block.SequencerAddress.String(),
	)
}

func TestBlockHeaderV013Unmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)
	block, err := client.Block(t.Context(), "319132")
	require.NoError(t, err)

	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x50e864db6b81ce69fbeb70e6a7284ee2febbb9a2e707415de7adab83525e9cd"),
		block.Hash,
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x39dfc381180356734085e2d70b640e153c241c7f65936cacbdff9fad84bbc0c",
		),
		block.ParentHash,
	)
	require.Equal(t, uint64(319132), block.Number)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x2a6b9a8b60e1de80dc50e6b704b415a38e8fd03d82244cec92cbff0821a8975"),
		block.StateRoot,
	)
	require.Equal(t, "ACCEPTED_ON_L2", block.Status)
	require.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x3b9aca08"), block.L1GasPriceETH())
	require.Equal(t, felt.NewUnsafeFromString[felt.Felt]("0x2540be400"), block.L1GasPriceSTRK())
	require.Equal(t, uint64(1700075354), block.Timestamp)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8",
		),
		block.SequencerAddress,
	)
	require.Equal(t, "0.13.0", block.Version)
}

func TestBlockHeaderV0131Unmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)
	block, err := client.Block(t.Context(), "330363")
	require.NoError(t, err)

	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x8ab8117e952f95efd96de0bc66dc6f13fe68dfda14b95fe1972759dee283a8",
		),
		block.Hash,
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x13367121d0b7e34a9b10c8a5a1c269811cd9afc3ce680c88888f1a22d2f017a",
		),
		block.TransactionCommitment,
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x1090dd2ab2aa22bd5fc5a59d3b1394d54461bb2a80156c4b2c2622d2c474ca2",
		),
		block.EventCommitment,
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt]("0x3b9aca0a"),
		block.L1GasPriceETH(),
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt]("0x2b6fdb70"),
		block.L1GasPriceSTRK(),
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt]("0x5265a14ef"),
		block.L1DataGasPrice.PriceInWei,
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt]("0x3c0c00c87"),
		block.L1DataGasPrice.PriceInFri,
	)
	require.Equal(t, starknet.Blob, block.L1DAMode)
	require.Equal(t, "0.13.1", block.Version)
	require.Equal(t, uint64(0), block.Receipts[0].ExecutionResources.DataAvailability.L1Gas)
	require.Equal(t, uint64(128), block.Receipts[0].ExecutionResources.DataAvailability.L1DataGas)
}

func TestBlockHeaderv0132Unmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
	block, err := client.Block(t.Context(), "35748")
	require.NoError(t, err)

	// Only focus on checking the new fields
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x1ea2a9cfa3df5297d58c0a04d09d276bc68d40fe64701305bbe2ed8f417e869",
		),
		block.Hash,
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x77140bef51bbb4d1932f17cc5081825ff18465a1df4440ca0429a4fa80f1dc5",
		),
		block.ParentHash,
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x6f12628d21a8df7f158b631d801fc0dd20034b9e22eca255bddc0c1c1bc283f",
		),
		block.ReceiptCommitment,
	)
	require.Equal(
		t,
		felt.NewUnsafeFromString[felt.Felt](
			"0x23587c54d590b57b8e25acbf1e1a422eb4cd104e95ee4a681021a6bb7456afa",
		),
		block.StateDiffCommitment,
	)
	require.Equal(t, uint64(6), block.StateDiffLength)
	require.Equal(t, "0.13.2", block.Version)
	require.Equal(t, uint64(117620), block.Receipts[0].ExecutionResources.TotalGasConsumed.L1Gas)
	require.Equal(t, uint64(192), block.Receipts[0].ExecutionResources.TotalGasConsumed.L1DataGas)
}

func TestClassV0Unmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	hash := felt.NewUnsafeFromString[felt.Felt](
		"0x01efa8f84fd4dff9e2902ec88717cf0dafc8c188f80c3450615944a469428f7f",
	)
	class, err := client.ClassDefinition(t.Context(), hash)
	require.NoError(t, err)

	assert.NotNil(t, class.DeprecatedCairo)
	assert.Nil(t, class.Sierra)

	assert.Equal(t, 1, len(class.DeprecatedCairo.EntryPoints.Constructor))
	assert.Equal(t, "0xa1", class.DeprecatedCairo.EntryPoints.Constructor[0].Offset.String())
	assert.Equal(
		t,
		"0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194",
		class.DeprecatedCairo.EntryPoints.Constructor[0].Selector.String(),
	)
	assert.Equal(t, 1, len(class.DeprecatedCairo.EntryPoints.L1Handler))
	assert.Equal(t, 1, len(class.DeprecatedCairo.EntryPoints.External))
	assert.NotEmpty(t, class.DeprecatedCairo.Program)
}

func TestClassV1Unmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)

	hash := felt.NewUnsafeFromString[felt.Felt](
		"0x4e70b19333ae94bd958625f7b61ce9eec631653597e68645e13780061b2136c",
	)
	class, err := client.ClassDefinition(t.Context(), hash)
	require.NoError(t, err)

	assert.NotNil(t, class.Sierra)
	assert.Nil(t, class.DeprecatedCairo)

	assert.Equal(t, 6626, len(class.Sierra.Program))
	assert.Equal(t, 704, len(class.Sierra.Abi))
	assert.Equal(t, "0.1.0", class.Sierra.Version)
	assert.Equal(t, 0, len(class.Sierra.EntryPoints.Constructor))
	assert.Equal(t, 0, len(class.Sierra.EntryPoints.L1Handler))

	selectors := []string{
		"0x22ff5f21f0b81b113e63f7db6da94fedef11b2119b4088b89664fb9a3cb658",
		"0x1fc3f77ebc090777f567969ad9823cf6334ab888acb385ca72668ec5adbde80",
		"0x3d778356014c91effae9863ee4a8c2663d8fa2e9f0c4145c1e01f5435ced0be",
	}
	assert.Equal(t, len(selectors), len(class.Sierra.EntryPoints.External))

	for idx, selector := range selectors {
		assert.Equal(t, uint64(idx), class.Sierra.EntryPoints.External[idx].Index)
		assert.Equal(t, selector, class.Sierra.EntryPoints.External[idx].Selector.String())
	}
}

func TestBuildQueryString_WithErrorUrl(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			require.Fail(t, "The code did not panic")
		}
	}()
	baseURL := "https\t://mock_feeder.io"
	client := feeder.NewClient(baseURL).WithUserAgent(ua)
	_, _ = client.Block(t.Context(), strconv.Itoa(0))
}

func TestStateUpdate(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		stateUpdate, err := client.StateUpdate(t.Context(), "0")
		require.NoError(t, err)

		assert.Equal(
			t,
			"0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943",
			stateUpdate.BlockHash.String(),
		)
		assert.Equal(
			t,
			"0x21870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6",
			stateUpdate.NewRoot.String(),
		)
		assert.Equal(t, "0x0", stateUpdate.OldRoot.String())
		assert.Equal(t, 0, len(stateUpdate.StateDiff.Nonces))
		assert.Equal(t, 0, len(stateUpdate.StateDiff.OldDeclaredContracts))
		assert.Equal(t, 5, len(stateUpdate.StateDiff.DeployedContracts))
		assert.Equal(
			t,
			"0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
			stateUpdate.StateDiff.DeployedContracts[0].Address.String(),
		)
		assert.Equal(
			t,
			"0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
			stateUpdate.StateDiff.DeployedContracts[0].ClassHash.String(),
		)
		assert.Equal(t, 5, len(stateUpdate.StateDiff.StorageDiffs))

		diff, ok := stateUpdate.
			StateDiff.
			StorageDiffs["0x735596016a37ee972c42adef6a3cf628c19bb3794369c65d2c82ba034aecf2c"]
		require.True(t, ok)
		assert.Equal(t, 2, len(diff))
		assert.Equal(t, "0x5", diff[0].Key.String())
		assert.Equal(t, "0x64", diff[0].Value.String())
	})
	t.Run("Test block number out of boundary", func(t *testing.T) {
		stateUpdate, err := client.StateUpdate(t.Context(), "1000000")
		assert.Nil(t, stateUpdate)
		assert.Error(t, err)
	})

	t.Run("v0.11.0 state update", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Integration)

		t.Run("declared Cairo0 classes", func(t *testing.T) {
			update, err := client.StateUpdate(t.Context(), "283746")
			require.NoError(t, err)
			assert.NotEmpty(t, update.StateDiff.OldDeclaredContracts)
		})

		t.Run("declared Cairo1 classes", func(t *testing.T) {
			update, err := client.StateUpdate(t.Context(), "283364")
			require.NoError(t, err)
			assert.NotEmpty(t, update.StateDiff.DeclaredClasses)
		})

		t.Run("replaced classes", func(t *testing.T) {
			update, err := client.StateUpdate(t.Context(), "283428")
			require.NoError(t, err)
			assert.NotEmpty(t, update.StateDiff.ReplacedClasses)
		})
	})
}

func TestTransaction(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		transactionHash := felt.NewUnsafeFromString[felt.Felt](
			"0x631333277e88053336d8c302630b4420dc3ff24018a1c464da37d5e36ea19df",
		)
		actualStatus, err := client.Transaction(t.Context(), transactionHash)
		require.NoError(t, err)
		assert.NotNil(t, actualStatus)
	})
	t.Run("Test case when transaction_hash does not exist", func(t *testing.T) {
		transactionHash := felt.NewUnsafeFromString[felt.Felt]("0xffff")
		actualStatus, err := client.Transaction(t.Context(), transactionHash)
		assert.NoError(t, err)
		assert.Equal(t, actualStatus.FinalityStatus, starknet.NotReceived)
	})
}

func TestBlock(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		actualBlock, err := client.Block(t.Context(), strconv.Itoa(11817))
		assert.Equal(t, nil, err, "Unexpected error")
		assert.NotNil(t, actualBlock)
	})
	t.Run("Test block number out of boundary", func(t *testing.T) {
		actualBlock, err := client.Block(t.Context(), strconv.Itoa(1000000))
		assert.Nil(t, actualBlock)
		assert.Error(t, err)
	})
	t.Run("Test latest block", func(t *testing.T) {
		actualBlock, err := client.Block(t.Context(), "latest")
		assert.Equal(t, nil, err, "Unexpected error")
		assert.NotNil(t, actualBlock)
	})
}

func TestClassDefinition(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		classHash := felt.NewUnsafeFromString[felt.Felt](
			"0x01efa8f84fd4dff9e2902ec88717cf0dafc8c188f80c3450615944a469428f7f",
		)

		actualClass, err := client.ClassDefinition(t.Context(), classHash)
		assert.Equal(t, nil, err, "Unexpected error")
		assert.NotNil(t, actualClass)
	})
	t.Run("Test classHash not find", func(t *testing.T) {
		classHash := felt.NewUnsafeFromString[felt.Felt]("0x000")
		actualClass, err := client.ClassDefinition(t.Context(), classHash)
		assert.Nil(t, actualClass)
		assert.Error(t, err)
	})
}

func TestHttpError(t *testing.T) {
	maxRetries := 2
	callCount := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount[r.URL.String()]++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	client := feeder.
		NewClient(srv.URL).
		WithBackoff(feeder.NopBackoff).
		WithMaxRetries(maxRetries).
		WithUserAgent(ua)

	t.Run("HTTP err in GetBlock", func(t *testing.T) {
		_, err := client.Block(t.Context(), strconv.Itoa(0))
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	t.Run("HTTP err in GetTransaction", func(t *testing.T) {
		_, err := client.Transaction(t.Context(), new(felt.Felt))
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	t.Run("HTTP err in GetClassDefinition", func(t *testing.T) {
		_, err := client.ClassDefinition(t.Context(), new(felt.Felt))
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	t.Run("HTTP err in GetStateUpdate", func(t *testing.T) {
		_, err := client.StateUpdate(t.Context(), "0")
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	for _, called := range callCount {
		assert.Equal(t, maxRetries+1, called)
	}
}

func TestBackoffFailure(t *testing.T) {
	maxRetries := 5
	try := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		try += 1
	}))
	t.Cleanup(srv.Close)

	c := feeder.
		NewClient(srv.URL).
		WithBackoff(feeder.NopBackoff).
		WithMaxRetries(maxRetries).
		WithUserAgent(ua)

	_, err := c.Block(t.Context(), strconv.Itoa(0))
	assert.EqualError(t, err, "500 Internal Server Error")
	assert.Equal(t, maxRetries, try-1) // we have retried `maxRetries` times
}

func TestCompiledClassDefinition(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)

	classHash := felt.NewUnsafeFromString[felt.Felt](
		"0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5",
	)
	class, err := client.CasmClassDefinition(t.Context(), classHash)
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", class.CompilerVersion)
	assert.Equal(
		t,
		"0x800000000000011000000000000000000000000000000000000000000000001",
		class.Prime,
	)
	assert.Equal(t, 3900, len(class.Bytecode))
	assert.Equal(t, 10, len(class.EntryPoints.External))
	assert.Equal(t, 1, len(class.EntryPoints.External[9].Builtins))
	assert.Equal(t, "range_check", class.EntryPoints.External[9].Builtins[0])
	assert.Equal(
		t,
		"0x3604cea1cdb094a73a31144f14a3e5861613c008e1e879939ebc4827d10cd50",
		class.EntryPoints.External[9].Selector.String(),
	)
}

func TestTransactionStatusRevertError(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)

	txnHash := felt.NewUnsafeFromString[felt.Felt](
		"0x19abec18bbacec23c2eee160c70190a48e4b41dd5ff98ad8f247f9393559998",
	)
	status, err := client.Transaction(t.Context(), txnHash)
	require.NoError(t, err)
	require.NotEmpty(t, status.RevertError)
}

func TestTransactionStatusTransactionFailureReason(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.SepoliaIntegration)

	txnHash := felt.NewUnsafeFromString[felt.Felt]("0x1111")
	expectedMessage := "some error"
	expectedErrorCode := "SOME_ERROR_CODE"
	status, err := client.Transaction(t.Context(), txnHash)
	require.NoError(t, err)
	require.Equal(t, expectedMessage, status.FailureReason.Message)
	require.Equal(t, expectedErrorCode, status.FailureReason.Code)
}

func TestPublicKey(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)

	actualPublicKey, err := client.PublicKey(t.Context())
	assert.NoError(t, err)
	assert.Equal(
		t,
		"0x52934be54ce926b1e715f15dc2542849a97ecfdf829cd0b7384c64eeeb2264e",
		actualPublicKey.String(),
	)
}

func TestSignature(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)

	t.Run("Test normal case", func(t *testing.T) {
		actualSignature, err := client.Signature(t.Context(), strconv.Itoa(214584))
		assert.NoError(t, err)
		assert.Equal(t, 2, len(actualSignature.Signature))
		assert.Equal(
			t,
			"0x351c1b3fdd944ec8a787085b386ae9adddc5e4e839525b0cdfa8fac7419fe16",
			actualSignature.Signature[0].String(),
		)
		assert.Equal(
			t,
			"0x63507ca773169dd5cf5c27036c69b7676b9c1c60538d1d91811e7cd7a5c0b64",
			actualSignature.Signature[1].String(),
		)
		assert.Equal(
			t,
			"0x5decb56a6651b829e01d8700235e7d99880bac258fd97fac4e30a3e5f1993f0",
			actualSignature.SignatureInput.BlockHash.String(),
		)
		assert.Equal(
			t,
			"0x4253056094397f30399b01aa6a9eb44e59f8298545c26f5f746d86940b6cab8",
			actualSignature.SignatureInput.StateDiffCommitment.String(),
		)
	})
	t.Run("Test on unexisting block", func(t *testing.T) {
		actualSignature, err := client.Signature(t.Context(), strconv.Itoa(10000000000))
		assert.Error(t, err)
		assert.Nil(t, actualSignature)
	})
	t.Run("Test on latest block", func(t *testing.T) {
		actualSignature, err := client.Signature(t.Context(), "latest")
		assert.NoError(t, err)
		assert.NotNil(t, actualSignature)
	})
}

func TestStateUpdateWithBlock(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)

	t.Run("Test normal case", func(t *testing.T) {
		actualStateUpdate, err := client.StateUpdateWithBlock(t.Context(), strconv.Itoa(0))
		assert.NoError(t, err)
		assert.Equal(
			t,
			"0x3ae41b0f023e53151b0c8ab8b9caafb7005d5f41c9ab260276d5bdc49726279",
			actualStateUpdate.Block.Hash.String(),
		)
		assert.Equal(t, "0x0", actualStateUpdate.Block.ParentHash.String())
		assert.Equal(
			t,
			"0x1f386a54db7796872829c9168cdc567980daad382daa4df3b71641a2551e833",
			actualStateUpdate.Block.StateRoot.String(),
		)
		assert.Equal(
			t,
			"0x3ae41b0f023e53151b0c8ab8b9caafb7005d5f41c9ab260276d5bdc49726279",
			actualStateUpdate.StateUpdate.BlockHash.String(),
		)
		assert.Equal(
			t,
			"0x1f386a54db7796872829c9168cdc567980daad382daa4df3b71641a2551e833",
			actualStateUpdate.StateUpdate.NewRoot.String(),
		)
		assert.Equal(t, "0x0", actualStateUpdate.StateUpdate.OldRoot.String())
		assert.Empty(t, actualStateUpdate.StateUpdate.StateDiff.Nonces)
		assert.Empty(t, actualStateUpdate.StateUpdate.StateDiff.DeclaredClasses)
	})
	t.Run("Test on unexisting block", func(t *testing.T) {
		actualStateUpdate, err := client.StateUpdateWithBlock(
			t.Context(),
			strconv.Itoa(10000000000),
		)
		assert.Error(t, err)
		assert.Nil(t, actualStateUpdate)
	})
	t.Run("Test on latest block", func(t *testing.T) {
		actualStateUpdate, err := client.StateUpdateWithBlock(t.Context(), "latest")
		assert.NoError(t, err)
		assert.NotNil(t, actualStateUpdate)
	})
}

func TestBlockTrace(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)

	t.Run("old block", func(t *testing.T) {
		trace, err := client.BlockTrace(
			t.Context(),
			"0x3ae41b0f023e53151b0c8ab8b9caafb7005d5f41c9ab260276d5bdc49726279",
		)
		require.NoError(t, err)
		require.Len(t, trace.Traces, 4)
	})

	t.Run("newer block", func(t *testing.T) {
		trace, err := client.BlockTrace(
			t.Context(),
			"0xe3828bd9154ab385e2cbb95b3b650365fb3c6a4321660d98ce8b0a9194f9a3",
		)
		require.NoError(t, err)
		require.Len(t, trace.Traces, 2)
	})
}

func TestEventListener(t *testing.T) {
	isCalled := false
	client := feeder.
		NewTestClient(t, &utils.Integration).
		WithListener(&feeder.SelectiveListener{
			OnResponseCb: func(urlPath string, status int, _ time.Duration) {
				isCalled = true
				require.Equal(t, 200, status)
				require.Equal(t, "/get_block", urlPath)
			},
		})
	_, err := client.Block(t.Context(), "0")
	require.NoError(t, err)
	require.True(t, isCalled)
}

func TestClientRetryBehavior(t *testing.T) {
	t.Run("succeeds after retrying with increased timeout", func(t *testing.T) {
		requestCount := 0
		srv := httptest.
			NewServer(http.
				HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						requestCount++

						if requestCount == 2 || requestCount == 1 {
							time.Sleep(800 * time.Millisecond)
							w.WriteHeader(http.StatusGatewayTimeout)
							return
						}

						_, err := w.Write([]byte(`{"block_hash": "0x123", "block_number": 1}`))
						if err != nil {
							panic("TestClientRetryBehavior: write error")
						}
					}))
		defer srv.Close()

		client := feeder.NewClient(srv.URL).
			WithTimeouts(
				[]time.Duration{250 * time.Millisecond, 750 * time.Millisecond, 2 * time.Second},
				false,
			).
			WithMaxRetries(2).
			WithBackoff(feeder.NopBackoff)

		block, err := client.Block(t.Context(), "1")
		require.NoError(t, err)
		require.NotNil(t, block)
		require.Equal(t, 3, requestCount)
	})

	t.Run("fails when max retries exceeded", func(t *testing.T) {
		requestCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			time.Sleep(300 * time.Millisecond)
			w.WriteHeader(http.StatusGatewayTimeout)
		}))
		defer srv.Close()

		client := feeder.NewClient(srv.URL).
			WithTimeouts([]time.Duration{250 * time.Millisecond}, false).
			WithMaxRetries(2).
			WithBackoff(feeder.NopBackoff)

		_, err := client.Block(t.Context(), "1")
		require.Error(t, err)
		require.Equal(t, 3, requestCount)
	})

	t.Run("stops retrying on success", func(t *testing.T) {
		requestCount := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount++
			if requestCount == 1 {
				time.Sleep(300 * time.Millisecond)
				w.WriteHeader(http.StatusGatewayTimeout)
				return
			}
			_, err := w.Write([]byte(`{"block_hash": "0x123", "block_number": 1}`))
			if err != nil {
				panic("TestClientRetryBehavior: write error")
			}
		}))
		defer srv.Close()

		client := feeder.NewClient(srv.URL).
			WithTimeouts([]time.Duration{250 * time.Millisecond, 750 * time.Millisecond}, false).
			WithMaxRetries(1).
			WithBackoff(feeder.NopBackoff)

		block, err := client.Block(t.Context(), "1")
		require.NoError(t, err)
		require.NotNil(t, block)
		require.Equal(t, 2, requestCount)
	})
}

func TestPreConfirmedBlock(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.SepoliaIntegration)

	snPreConfirmedBlock, err := client.PreConfirmedBlock(t.Context(), strconv.Itoa(1204672))
	assert.NoError(t, err)

	assert.Equal(t, "0.14.0", snPreConfirmedBlock.Version)
	assert.Equal(t, len(snPreConfirmedBlock.Transactions), len(snPreConfirmedBlock.Receipts))
	assert.Equal(
		t,
		len(snPreConfirmedBlock.TransactionStateDiffs),
		len(snPreConfirmedBlock.Receipts),
	)
	assert.NotNil(t, snPreConfirmedBlock.L1DAMode)
	assert.NotNil(t, snPreConfirmedBlock.L1GasPrice)
	assert.NotNil(t, snPreConfirmedBlock.L1DataGasPrice)
	assert.NotNil(t, snPreConfirmedBlock.L2GasPrice)
	assert.NotNil(t, snPreConfirmedBlock.SequencerAddress)
	assert.NotNil(t, snPreConfirmedBlock.Timestamp)
}
