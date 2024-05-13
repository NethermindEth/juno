package feeder_test

import (
	"context"
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
		txnHash := utils.HexToFelt(t, "0x93f542728e403f1edcea4a41f1509a39be35ebcad7d4b5aa77623e5e6480d")
		status, err := client.Transaction(context.Background(), txnHash)
		require.NoError(t, err)

		declareTx := status.Transaction
		assert.Equal(t, "0x93f542728e403f1edcea4a41f1509a39be35ebcad7d4b5aa77623e5e6480d", declareTx.Hash.String())
		assert.Equal(t, "0x1", declareTx.Version.String())
		assert.Equal(t, "0x5af3107a4000", declareTx.MaxFee.String())
		assert.Equal(t, "0x1d", declareTx.Nonce.String())
		assert.Equal(t, "0x2ed6bb4d57ad27a22972b81feb9d09798ff8c273684376ec72c154d90343453", declareTx.ClassHash.String())
		assert.Equal(t, "0xb8a60857ed233885155f1d839086ca7ad03e6d4237cc10b085a4652a61a23", declareTx.SenderAddress.String())
		assert.Equal(t, starknet.TxnDeclare, declareTx.Type)
		assert.Equal(t, 2, len(*declareTx.Signature))
		assert.Equal(t, "0x516b5999b47509105675dd4c6ed9c373448038cfd00549fe868695916eee0ff", (*declareTx.Signature)[0].String())
		assert.Equal(t, "0x6c0189aaa56bfcb2a3e97198d04bd7a9750a4354b88f4e5edf57cf4d966ddda", (*declareTx.Signature)[1].String())
	})

	t.Run("v0.1", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Sepolia)
		txnHash := utils.HexToFelt(t, "0x1dde7d379485cceb9ec0a5aacc5217954985792f12b9181cf938ec341046491")
		status, err := client.Transaction(context.Background(), txnHash)
		require.NoError(t, err)

		require.Equal(t, &starknet.Transaction{
			Hash:    utils.HexToFelt(t, "0x1dde7d379485cceb9ec0a5aacc5217954985792f12b9181cf938ec341046491"),
			Version: utils.HexToFelt(t, "0x3"),
			Signature: &[]*felt.Felt{
				utils.HexToFelt(t, "0x5be36745b03aaeb76712c68869f944f7c711f9e734763b8d0b4e5b834408ea4"),
				utils.HexToFelt(t, "0x66c9dba8bb26ada30cf3a393a6c26bfd3a40538f19b3b4bfb57c7507962ae79"),
			},
			Nonce:       new(felt.Felt).SetUint64(3),
			NonceDAMode: utils.Ptr(starknet.DAModeL1),
			FeeDAMode:   utils.Ptr(starknet.DAModeL1),
			ResourceBounds: &map[starknet.Resource]starknet.ResourceBounds{
				starknet.ResourceL1Gas: {
					MaxAmount:       utils.HexToFelt(t, "0x1f40"),
					MaxPricePerUnit: utils.HexToFelt(t, "0x5af3107a4000"),
				},
				starknet.ResourceL2Gas: {
					MaxAmount:       new(felt.Felt),
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:                   new(felt.Felt),
			PaymasterData:         &[]*felt.Felt{},
			SenderAddress:         utils.HexToFelt(t, "0x6aac79bb6c90e1e41c33cd20c67c0281c4a95f01b4e15ad0c3b53fcc6010cf8"),
			ClassHash:             utils.HexToFelt(t, "0x2404dffbfe2910bd921f5935e628c01e457629fc779420a03b7e5e507212f36"),
			CompiledClassHash:     utils.HexToFelt(t, "0x5047109bf7eb550c5e6b0c37714f6e0db4bb8b5b227869e0797ecfc39240aa7"),
			AccountDeploymentData: &[]*felt.Felt{},
			Type:                  starknet.TxnDeclare,
		}, status.Transaction)
	})
}

func TestInvokeTransactionUnmarshal(t *testing.T) {
	t.Run("pre-v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Mainnet)

		txnHash := utils.HexToFelt(t, "0x631333277e88053336d8c302630b4420dc3ff24018a1c464da37d5e36ea19df")
		status, err := client.Transaction(context.Background(), txnHash)
		require.NoError(t, err)

		invokeTx := status.Transaction
		assert.Equal(t, "0x631333277e88053336d8c302630b4420dc3ff24018a1c464da37d5e36ea19df", invokeTx.Hash.String())
		assert.Equal(t, "0x0", invokeTx.Version.String())
		assert.Equal(t, "0x0", invokeTx.MaxFee.String())
		assert.Equal(t, 0, len(*invokeTx.Signature))
		assert.Equal(t, "0x17daeb497b6fe0f7adaa32b44677c3a9712b6856b792ad993fcef20aed21ac8", invokeTx.ContractAddress.String())
		assert.Equal(t, "0x218f305395474a84a39307fa5297be118fe17bf65e27ac5e2de6617baa44c64", invokeTx.EntryPointSelector.String())
		assert.Equal(t, 2, len(*invokeTx.CallData))
		assert.Equal(t, "0x346f2b6376b4b57f714ba187716fce9edff1361628cc54783ed0351538faa5e", (*invokeTx.CallData)[0].String())
		assert.Equal(t, "0x2", (*invokeTx.CallData)[1].String())
		assert.Equal(t, starknet.TxnInvoke, invokeTx.Type)
	})

	t.Run("v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Sepolia)
		txnHash := utils.HexToFelt(t, "0x6e7ae47173b6935899320dd41d540a27f8d5712febbaf13fe8d8aeaf4ac9164")
		status, err := client.Transaction(context.Background(), txnHash)
		require.NoError(t, err)

		require.Equal(t, &starknet.Transaction{
			Hash:    utils.HexToFelt(t, "0x6e7ae47173b6935899320dd41d540a27f8d5712febbaf13fe8d8aeaf4ac9164"),
			Version: new(felt.Felt).SetUint64(3),
			Signature: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				utils.HexToFelt(t, "0x4235b7a9cad6024cbb3296325e23b2a03d34a95c3ee3d5c10e2b6076c257d77"),
				utils.HexToFelt(t, "0x439de4b0c238f624c14c2619aa9d190c6c1d17f6556af09f1697cfe74f192fc"),
			},
			Nonce:       utils.HexToFelt(t, "0x8"),
			NonceDAMode: utils.Ptr(starknet.DAModeL1),
			FeeDAMode:   utils.Ptr(starknet.DAModeL1),
			ResourceBounds: &map[starknet.Resource]starknet.ResourceBounds{
				starknet.ResourceL1Gas: {
					MaxAmount:       utils.HexToFelt(t, "0xa0"),
					MaxPricePerUnit: utils.HexToFelt(t, "0xe91444530acc"),
				},
				starknet.ResourceL2Gas: {
					MaxAmount:       new(felt.Felt),
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:           new(felt.Felt),
			PaymasterData: &[]*felt.Felt{},
			SenderAddress: utils.HexToFelt(t, "0x6247aaebf5d2ff56c35cce1585bf255963d94dd413a95020606d523c8c7f696"),
			CallData: &[]*felt.Felt{
				utils.HexToFelt(t, "0x1"),
				utils.HexToFelt(t, "0x19c92fa87f4d5e3be25c3dd6a284f30282a07e87cd782f5fd387b82c8142017"),
				utils.HexToFelt(t, "0x3059098e39fbb607bc96a8075eb4d17197c3a6c797c166470997571e6fa5b17"),
				utils.HexToFelt(t, "0x0"),
			},
			AccountDeploymentData: &[]*felt.Felt{},
			Type:                  starknet.TxnInvoke,
		}, status.Transaction)
	})
}

//nolint:dupl
func TestDeployTransactionUnmarshal(t *testing.T) {
	t.Run("pre-v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Mainnet)

		txnHash := utils.HexToFelt(t, "0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee")
		status, err := client.Transaction(context.Background(), txnHash)
		require.NoError(t, err)

		deployTx := status.Transaction
		assert.Equal(t, "0x6d3e06989ee2245139cd677f59b4da7f360a27b2b614a4eb088fdf5862d23ee", deployTx.Hash.String())
		assert.Equal(t, "0x0", deployTx.Version.String())
		assert.Equal(t, "0x7cc55b21de4b7d6d7389df3b27de950924ac976d263ac8d71022d0b18155fc", deployTx.ContractAddress.String())
		assert.Equal(t, "0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8", deployTx.ContractAddressSalt.String())
		assert.Equal(t, "0x3131fa018d520a037686ce3efddeab8f28895662f019ca3ca18a626650f7d1e", deployTx.ClassHash.String())
		assert.Equal(t, 4, len(*deployTx.ConstructorCallData))
		assert.Equal(t, "0x69577e6756a99b584b5d1ce8e60650ae33b6e2b13541783458268f07da6b38a", (*deployTx.ConstructorCallData)[0].String())
		assert.Equal(t, "0x2dd76e7ad84dbed81c314ffe5e7a7cacfb8f4836f01af4e913f275f89a3de1a", (*deployTx.ConstructorCallData)[1].String())
		assert.Equal(t, "0x1", (*deployTx.ConstructorCallData)[2].String())
		assert.Equal(t, "0x614b9e0c3cb7a8f4ed73b673eba239c41a172859bf129c4b269c4b8057e21d8", (*deployTx.ConstructorCallData)[3].String())
		assert.Equal(t, starknet.TxnDeploy, deployTx.Type)
	})
}

func TestDeployAccountTransactionUnmarshal(t *testing.T) {
	t.Run("pre-v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Mainnet)

		txnHash := utils.HexToFelt(t, "0x32b272b6d0d584305a460197aa849b5c7a9a85903b66e9d3e1afa2427ef093e")
		status, err := client.Transaction(context.Background(), txnHash)
		require.NoError(t, err)

		deployTx := status.Transaction
		assert.Equal(t, "0x32b272b6d0d584305a460197aa849b5c7a9a85903b66e9d3e1afa2427ef093e", deployTx.Hash.String())
		assert.Equal(t, "0x1", deployTx.Version.String())
		assert.Equal(t, "0x59f5f9f474b0", deployTx.MaxFee.String())
		assert.Equal(t, 2, len(*deployTx.Signature))
		assert.Equal(t, "0x467ae89bbbbaa0139e8f8a02ddc614bd80252998f3c033239f59f9f2ab973c5", (*deployTx.Signature)[0].String())
		assert.Equal(t, "0x92938929b5afcd596d651a6d28ed38baf90b000192897617d98de19d475331", (*deployTx.Signature)[1].String())
		assert.Equal(t, "0x0", deployTx.Nonce.String())
		assert.Equal(t, "0x104714313388bd0ab569ac247fed6cf0b7a2c737105c00d64c23e24bd8dea40", deployTx.ContractAddress.String())
		assert.Equal(t, "0x25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3", deployTx.ContractAddressSalt.String())
		assert.Equal(t, "0x25ec026985a3bf9d0cc1fe17326b245dfdc3ff89b8fde106542a3ea56c5a918", deployTx.ClassHash.String())

		assert.Equal(t, 5, len(*deployTx.ConstructorCallData))
		assert.Equal(t, "0x33434ad846cdd5f23eb73ff09fe6fddd568284a0fb7d1be20ee482f044dabe2", (*deployTx.ConstructorCallData)[0].String())
		assert.Equal(t, "0x79dc0da7c54b95f10aa182ad0a46400db63156920adb65eca2654c0945a463", (*deployTx.ConstructorCallData)[1].String())
		assert.Equal(t, "0x2", (*deployTx.ConstructorCallData)[2].String())
		assert.Equal(t, "0x25b9dbdab19b190a556aa42cdfbc07ad6ffe415031e42a8caffd4a2438d5cc3", (*deployTx.ConstructorCallData)[3].String())
		assert.Equal(t, "0x0", (*deployTx.ConstructorCallData)[4].String())
		assert.Equal(t, starknet.TxnDeployAccount, deployTx.Type)
	})

	t.Run("v0.3", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Sepolia)
		txnHash := utils.HexToFelt(t, "0x138c9f01c27c56ceff5c9adb05f2a025ae4ebeb35ba4ac88572abd23c5623f")
		status, err := client.Transaction(context.Background(), txnHash)
		require.NoError(t, err)

		require.Equal(t, &starknet.Transaction{
			Hash:      utils.HexToFelt(t, "0x138c9f01c27c56ceff5c9adb05f2a025ae4ebeb35ba4ac88572abd23c5623f"),
			Version:   utils.HexToFelt(t, "0x3"),
			ClassHash: utils.HexToFelt(t, "0x29927c8af6bccf3f6fda035981e765a7bdbf18a2dc0d630494f8758aa908e2b"),
			Signature: &[]*felt.Felt{
				utils.HexToFelt(t, "0x79ec88c0f655e07f49a66bc6d4d9e696cf578addf6a0538f81dc3b47ca66c64"),
				utils.HexToFelt(t, "0x78d3f2549f6f5b8533730a0f4f76c4277bc1b358f805d7cf66414289ce0a46d"),
			},
			Nonce:       new(felt.Felt),
			NonceDAMode: utils.Ptr(starknet.DAModeL1),
			FeeDAMode:   utils.Ptr(starknet.DAModeL1),
			ResourceBounds: &map[starknet.Resource]starknet.ResourceBounds{
				starknet.ResourceL1Gas: {
					MaxAmount:       utils.HexToFelt(t, "0x1b52"),
					MaxPricePerUnit: utils.HexToFelt(t, "0x15416c61bfea"),
				},
				starknet.ResourceL2Gas: {
					MaxAmount:       new(felt.Felt),
					MaxPricePerUnit: new(felt.Felt),
				},
			},
			Tip:                 new(felt.Felt),
			PaymasterData:       &[]*felt.Felt{},
			SenderAddress:       utils.HexToFelt(t, "0x7ac1f9f2dde09f938e7ace23009160bf4b2f48a69363983abc1f6d51cb39e37"),
			ContractAddressSalt: utils.HexToFelt(t, "0x202674c5f7f0ee6ea3248496afccc6e27f77fd5634628d07c5710f8a4fbf1a2"),
			ConstructorCallData: &[]*felt.Felt{
				utils.HexToFelt(t, "0x202674c5f7f0ee6ea3248496afccc6e27f77fd5634628d07c5710f8a4fbf1a2"),
				utils.HexToFelt(t, "0x0"),
			},
			Type: starknet.TxnDeployAccount,
		}, status.Transaction)
	})
}

//nolint:dupl
func TestL1HandlerTransactionUnmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	txnHash := utils.HexToFelt(t, "0x218adbb5aea7985d67fe49b45d44a991380b63db41622f9f4adc36274d02190")
	status, err := client.Transaction(context.Background(), txnHash)
	require.NoError(t, err)

	handlerTx := status.Transaction
	assert.Equal(t, "0x218adbb5aea7985d67fe49b45d44a991380b63db41622f9f4adc36274d02190", handlerTx.Hash.String())
	assert.Equal(t, "0x0", handlerTx.Version.String())
	assert.Equal(t, "0x73314940630fd6dcda0d772d4c972c4e0a9946bef9dabf4ef84eda8ef542b82", handlerTx.ContractAddress.String())
	assert.Equal(t, "0x2d757788a8d8d6f21d1cd40bce38a8222d70654214e96ff95d8086e684fbee5", handlerTx.EntryPointSelector.String())
	assert.Equal(t, "0x1654d", handlerTx.Nonce.String())
	assert.Equal(t, 4, len(*handlerTx.CallData))
	assert.Equal(t, "0xae0ee0a63a2ce6baeeffe56e7714fb4efe48d419", (*handlerTx.CallData)[0].String())
	assert.Equal(t, "0x218559e75713ca564d6eaf043b73388e9ac7c2f459ef8905988052051d3ef5e", (*handlerTx.CallData)[1].String())
	assert.Equal(t, "0x2386f26fc10000", (*handlerTx.CallData)[2].String())
	assert.Equal(t, "0x0", (*handlerTx.CallData)[3].String())
	assert.Equal(t, starknet.TxnL1Handler, handlerTx.Type)
}

func TestBlockWithoutSequencerAddressUnmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	block, err := client.Block(context.Background(), strconv.Itoa(11817))
	require.NoError(t, err)

	assert.Equal(t, "0x24c692acaed3b486990bd9d2b2fbbee802b37b3bd79c59f295bad3277200a83", block.Hash.String())
	assert.Equal(t, "0x3873ccb937f14429b169c654dda28886d2cc2d6ea17b3cff9748fe5cfdb67e0", block.ParentHash.String())
	assert.Equal(t, uint64(11817), block.Number)
	assert.Equal(t, "0x3df24be7b5fed6b41de08d38686b6142944119ca2a345c38793590d6804bba4", block.StateRoot.String())
	assert.Equal(t, "ACCEPTED_ON_L2", block.Status)
	assert.Equal(t, "0x27ad16775", block.GasPriceETH().String())
	assert.Equal(t, 52, len(block.Transactions))
	assert.Equal(t, 52, len(block.Receipts))
	assert.Equal(t, uint64(1669465009), block.Timestamp)
	assert.Equal(t, "0.10.1", block.Version)
}

func TestBlockWithSequencerAddressUnmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	block, err := client.Block(context.Background(), strconv.Itoa(19199))
	require.NoError(t, err)

	assert.Equal(t, "0x41811b69473f26503e0375806ee97d05951ccc7840e3d2bbe14ffb2522e5be1", block.Hash.String())
	assert.Equal(t, "0x68427fb6f1f5e687fbd779b3cc0d4ee31b49575ed0f8c749f827e4a45611efc", block.ParentHash.String())
	assert.Equal(t, uint64(19199), block.Number)
	assert.Equal(t, "0x541b796ea02703d02ff31459815f65f410ceefe80a4e3499f7ef9ccc36d26ee", block.StateRoot.String())
	assert.Equal(t, "ACCEPTED_ON_L2", block.Status)
	assert.Equal(t, "0x31c4e2d75", block.GasPriceETH().String())
	assert.Equal(t, 324, len(block.Transactions))
	assert.Equal(t, 324, len(block.Receipts))
	assert.Equal(t, uint64(1674728186), block.Timestamp)
	assert.Equal(t, "0.10.3", block.Version)
	assert.Equal(t, "0x5dcd266a80b8a5f29f04d779c6b166b80150c24f2180a75e82427242dab20a9", block.SequencerAddress.String())
}

func TestBlockHeaderV0123Unmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Sepolia)
	block, err := client.Block(context.Background(), "4850")
	require.NoError(t, err)

	require.Equal(t, utils.HexToFelt(t, "0x410dfca0f99545e62aef946e228329ce3a906f6785f5e6f97389f30ad1c1088"), block.Hash)
	require.Equal(t, utils.HexToFelt(t, "0x61d60ea141cd3677a36da9ac7bdc5b4535b76bf9c5e6dd0bddfb04036b460c6"), block.ParentHash)
	require.Equal(t, uint64(4850), block.Number)
	require.Equal(t, utils.HexToFelt(t, "0x42f9257e7075f9cffffcfbda7dd7b318ee3474436b6a7d17ad152c45e8738ce"), block.StateRoot)
	require.Equal(t, "ACCEPTED_ON_L1", block.Status)
	require.Equal(t, utils.HexToFelt(t, "0x80197ea0"), block.GasPriceETH())
	require.Equal(t, utils.HexToFelt(t, "0x0"), block.GasPriceSTRK())
	require.Equal(t, uint64(1702188621), block.Timestamp)
	require.Equal(t, utils.HexToFelt(t, "0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8"), block.SequencerAddress)
	require.Equal(t, "0.12.3", block.Version)
}

func TestBlockHeaderV0131Unmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Sepolia)
	block, err := client.Block(context.Background(), "56377")
	require.NoError(t, err)

	require.Equal(t, utils.HexToFelt(t, "0x609e8ffabfdca05b5a2e7c1bd99fc95a757e7b4ef9186aeb1f301f3741458ce"), block.Hash)
	require.Equal(t, utils.HexToFelt(t, "0x5e3ae2b28d867474a618d81f120e193a6e1e01e4cd1c95139706ce0bca35a88"), block.TransactionCommitment)
	require.Equal(t, utils.HexToFelt(t, "0x505188f7acc80cd76b56832b86dd94638de937aecb5fa408f51abc5ec8fad8e"), block.EventCommitment)
	require.Equal(t, utils.HexToFelt(t, "0x4a817c800"), block.GasPriceETH())
	require.Equal(t, utils.HexToFelt(t, "0x1d1a94a20000"), block.GasPriceSTRK())
	require.Equal(t, utils.HexToFelt(t, "0x6b85dda55"), block.L1DataGasPrice.PriceInWei)
	require.Equal(t, utils.HexToFelt(t, "0x2dfb78bf913d"), block.L1DataGasPrice.PriceInFri)
	require.Equal(t, starknet.Blob, block.L1DAMode)
	require.Equal(t, "0.13.1", block.Version)
	require.Equal(t, uint64(0), block.Receipts[0].ExecutionResources.DataAvailability.L1Gas)
	require.Equal(t, uint64(0x380), block.Receipts[0].ExecutionResources.DataAvailability.L1DataGas)
}

func TestClassV0Unmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	hash := utils.HexToFelt(t, "0x01efa8f84fd4dff9e2902ec88717cf0dafc8c188f80c3450615944a469428f7f")
	class, err := client.ClassDefinition(context.Background(), hash)
	require.NoError(t, err)

	assert.NotNil(t, class.V0)
	assert.Nil(t, class.V1)

	assert.Equal(t, 1, len(class.V0.EntryPoints.Constructor))
	assert.Equal(t, "0xa1", class.V0.EntryPoints.Constructor[0].Offset.String())
	assert.Equal(t, "0x28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194", class.V0.EntryPoints.Constructor[0].Selector.String())
	assert.Equal(t, 1, len(class.V0.EntryPoints.L1Handler))
	assert.Equal(t, 1, len(class.V0.EntryPoints.External))
	assert.NotEmpty(t, class.V0.Program)
}

func TestClassV1Unmarshal(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Sepolia)

	hash := utils.HexToFelt(t, "0x16342ade8a7cc8296920731bc34b5a6530f5ee1dc1bfd3cc83cb3f519d6530a")
	class, err := client.ClassDefinition(context.Background(), hash)
	require.NoError(t, err)

	assert.NotNil(t, class.V1)
	assert.Nil(t, class.V0)

	assert.Equal(t, 440, len(class.V1.Program))
	assert.Equal(t, 686, len(class.V1.Abi))
	assert.Equal(t, "0.1.0", class.V1.Version)
	assert.Equal(t, 0, len(class.V1.EntryPoints.Constructor))
	assert.Equal(t, 0, len(class.V1.EntryPoints.L1Handler))

	selectors := []string{
		"0x10b7e63d3ca05c9baffd985d3e1c3858d4dbf0759f066be0eaddc5d71c2cab5",
		"0x3370263ab53343580e77063a719a5865004caff7f367ec136a6cdd34b6786ca",
		"0x3147e009aa1d3b7827f0cf9ce80b10dd02b119d549eb0a2627600662354eba",
	}
	assert.Equal(t, len(selectors), len(class.V1.EntryPoints.External))

	for idx, selector := range selectors {
		assert.Equal(t, uint64(idx), class.V1.EntryPoints.External[idx].Index)
		assert.Equal(t, selector, class.V1.EntryPoints.External[idx].Selector.String())
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
	_, _ = client.Block(context.Background(), strconv.Itoa(0))
}

func TestStateUpdate(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		stateUpdate, err := client.StateUpdate(context.Background(), "0")
		require.NoError(t, err)

		assert.Equal(t, "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943", stateUpdate.BlockHash.String())
		assert.Equal(t, "0x21870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6", stateUpdate.NewRoot.String())
		assert.Equal(t, "0x0", stateUpdate.OldRoot.String())
		assert.Equal(t, 0, len(stateUpdate.StateDiff.Nonces))
		assert.Equal(t, 0, len(stateUpdate.StateDiff.OldDeclaredContracts))
		assert.Equal(t, 5, len(stateUpdate.StateDiff.DeployedContracts))
		assert.Equal(t, "0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6", stateUpdate.StateDiff.DeployedContracts[0].Address.String())
		assert.Equal(t, "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8", stateUpdate.StateDiff.DeployedContracts[0].ClassHash.String())
		assert.Equal(t, 5, len(stateUpdate.StateDiff.StorageDiffs))

		diff, ok := stateUpdate.StateDiff.StorageDiffs["0x735596016a37ee972c42adef6a3cf628c19bb3794369c65d2c82ba034aecf2c"]
		require.True(t, ok)
		assert.Equal(t, 2, len(diff))
		assert.Equal(t, "0x5", diff[0].Key.String())
		assert.Equal(t, "0x64", diff[0].Value.String())
	})
	t.Run("Test block number out of boundary", func(t *testing.T) {
		stateUpdate, err := client.StateUpdate(context.Background(), "1000000")
		assert.Nil(t, stateUpdate)
		assert.Error(t, err)
	})

	t.Run("v0.11.0 state update", func(t *testing.T) {
		client := feeder.NewTestClient(t, &utils.Sepolia)

		t.Run("declared Cairo0 classes", func(t *testing.T) {
			update, err := client.StateUpdate(context.Background(), "7")
			require.NoError(t, err)
			assert.NotEmpty(t, update.StateDiff.OldDeclaredContracts)
		})

		t.Run("declared Cairo1 classes", func(t *testing.T) {
			update, err := client.StateUpdate(context.Background(), "7")
			require.NoError(t, err)
			assert.NotEmpty(t, update.StateDiff.DeclaredClasses)
		})

		t.Run("replaced classes", func(t *testing.T) {
			update, err := client.StateUpdate(context.Background(), "6500")
			require.NoError(t, err)
			assert.NotEmpty(t, update.StateDiff.ReplacedClasses)
		})
	})
}

func TestTransaction(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		transactionHash := utils.HexToFelt(t, "0x631333277e88053336d8c302630b4420dc3ff24018a1c464da37d5e36ea19df")
		actualStatus, err := client.Transaction(context.Background(), transactionHash)
		require.NoError(t, err)
		assert.NotNil(t, actualStatus)
	})
	t.Run("Test case when transaction_hash does not exist", func(t *testing.T) {
		transactionHash := utils.HexToFelt(t, "0xffff")
		actualStatus, err := client.Transaction(context.Background(), transactionHash)
		assert.NoError(t, err)
		assert.Equal(t, actualStatus.FinalityStatus, starknet.NotReceived)
	})
}

func TestBlock(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		actualBlock, err := client.Block(context.Background(), strconv.Itoa(11817))
		assert.Equal(t, nil, err, "Unexpected error")
		assert.NotNil(t, actualBlock)
	})
	t.Run("Test block number out of boundary", func(t *testing.T) {
		actualBlock, err := client.Block(context.Background(), strconv.Itoa(1000000))
		assert.Nil(t, actualBlock)
		assert.Error(t, err)
	})
	t.Run("Test latest block", func(t *testing.T) {
		actualBlock, err := client.Block(context.Background(), "latest")
		assert.Equal(t, nil, err, "Unexpected error")
		assert.NotNil(t, actualBlock)
	})
}

func TestClassDefinition(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		classHash := utils.HexToFelt(t, "0x01efa8f84fd4dff9e2902ec88717cf0dafc8c188f80c3450615944a469428f7f")

		actualClass, err := client.ClassDefinition(context.Background(), classHash)
		assert.Equal(t, nil, err, "Unexpected error")
		assert.NotNil(t, actualClass)
	})
	t.Run("Test classHash not find", func(t *testing.T) {
		classHash := utils.HexToFelt(t, "0x000")
		actualClass, err := client.ClassDefinition(context.Background(), classHash)
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
	client := feeder.NewClient(srv.URL).WithBackoff(feeder.NopBackoff).WithMaxRetries(maxRetries).WithUserAgent(ua)

	t.Run("HTTP err in GetBlock", func(t *testing.T) {
		_, err := client.Block(context.Background(), strconv.Itoa(0))
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	t.Run("HTTP err in GetTransaction", func(t *testing.T) {
		_, err := client.Transaction(context.Background(), new(felt.Felt))
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	t.Run("HTTP err in GetClassDefinition", func(t *testing.T) {
		_, err := client.ClassDefinition(context.Background(), new(felt.Felt))
		assert.EqualError(t, err, "500 Internal Server Error")
	})

	t.Run("HTTP err in GetStateUpdate", func(t *testing.T) {
		_, err := client.StateUpdate(context.Background(), "0")
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

	c := feeder.NewClient(srv.URL).WithBackoff(feeder.NopBackoff).WithMaxRetries(maxRetries).WithUserAgent(ua)

	_, err := c.Block(context.Background(), strconv.Itoa(0))
	assert.EqualError(t, err, "500 Internal Server Error")
	assert.Equal(t, maxRetries, try-1) // we have retried `maxRetries` times
}

func TestCompiledClassDefinition(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	classHash := utils.HexToFelt(t, "0x1338d85d3e579f6944ba06c005238d145920afeb32f94e3a1e234d21e1e9292")
	class, err := client.CompiledClassDefinition(context.Background(), classHash)
	require.NoError(t, err)
	assert.Equal(t, "1.1.0", class.CompilerVersion)
	assert.Equal(t, "0x800000000000011000000000000000000000000000000000000000000000001", class.Prime)
	assert.Equal(t, 4809, len(class.Bytecode))
	assert.Equal(t, 13, len(class.EntryPoints.External))
	assert.Equal(t, 1, len(class.EntryPoints.External[9].Builtins))
	assert.Equal(t, "range_check", class.EntryPoints.External[9].Builtins[0])
	assert.Equal(t, "0x2913ee03e5e3308c41e308bd391ea4faac9b9cb5062c76a6b3ab4f65397e106", class.EntryPoints.External[9].Selector.String())
}

func TestTransactionStatusRevertError(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	txnHash := utils.HexToFelt(t, "0x34e02ab97e273f9a4a9b616fe549cf14a1dcb9431673896290c9543e051240e")
	status, err := client.Transaction(context.Background(), txnHash)
	require.NoError(t, err)
	require.NotEmpty(t, status.RevertError)
}

func TestPublicKey(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	actualPublicKey, err := client.PublicKey(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "0x48253ff2c3bed7af18bde0b611b083b39445959102d4947c51c4db6aa4f4e58", actualPublicKey.String())
}

func TestSignature(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		actualSignature, err := client.Signature(context.Background(), strconv.Itoa(19199))
		assert.NoError(t, err)
		assert.Equal(t, 2, len(actualSignature.Signature))
		assert.Equal(t, "0xba322e5df3fbebb28f61395a8e8103d224a8dc60a17668d90a806fc12ce04b", actualSignature.Signature[0].String())
		assert.Equal(t, "0x521f3894198aa13e668682279e58cb525ed4c0d80a58ec1ad1c11b8ea2701b8", actualSignature.Signature[1].String())
		assert.Equal(t, "0x41811b69473f26503e0375806ee97d05951ccc7840e3d2bbe14ffb2522e5be1", actualSignature.SignatureInput.BlockHash.String())
		assert.Equal(t, "0x5b063f2921d0a3eb38e77d71d9dba14972f27f787df8056e06dca2242b6de1d", actualSignature.SignatureInput.StateDiffCommitment.String())
	})
	t.Run("Test on unexisting block", func(t *testing.T) {
		actualSignature, err := client.Signature(context.Background(), strconv.Itoa(10000000000))
		assert.Error(t, err)
		assert.Nil(t, actualSignature)
	})
	t.Run("Test on latest block", func(t *testing.T) {
		actualSignature, err := client.Signature(context.Background(), "latest")
		assert.NoError(t, err)
		assert.NotNil(t, actualSignature)
	})
}

func TestStateUpdateWithBlock(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("Test normal case", func(t *testing.T) {
		actualStateUpdate, err := client.StateUpdateWithBlock(context.Background(), strconv.Itoa(0))
		assert.NoError(t, err)
		assert.Equal(t, "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943", actualStateUpdate.Block.Hash.String())
		assert.Equal(t, "0x0", actualStateUpdate.Block.ParentHash.String())
		assert.Equal(t, "0x21870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6", actualStateUpdate.Block.StateRoot.String())
		assert.Equal(t, "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943", actualStateUpdate.StateUpdate.BlockHash.String())
		assert.Equal(t, "0x21870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6", actualStateUpdate.StateUpdate.NewRoot.String())
		assert.Equal(t, "0x0", actualStateUpdate.StateUpdate.OldRoot.String())
		assert.Empty(t, actualStateUpdate.StateUpdate.StateDiff.Nonces)
		assert.Empty(t, actualStateUpdate.StateUpdate.StateDiff.DeclaredClasses)
	})
	t.Run("Test on unexisting block", func(t *testing.T) {
		actualStateUpdate, err := client.StateUpdateWithBlock(context.Background(), strconv.Itoa(10000000000))
		assert.Error(t, err)
		assert.Nil(t, actualStateUpdate)
	})
	t.Run("Test on latest block", func(t *testing.T) {
		actualStateUpdate, err := client.StateUpdateWithBlock(context.Background(), "latest")
		assert.NoError(t, err)
		assert.NotNil(t, actualStateUpdate)
	})
}

func TestBlockTrace(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)

	t.Run("old block", func(t *testing.T) {
		trace, err := client.BlockTrace(context.Background(), "0x4e1f77f39545afe866ac151ac908bd1a347a2a8a7d58bef1276db4f06fdf2f6")
		require.NoError(t, err)
		require.Len(t, trace.Traces, 6)
	})

	t.Run("newer block", func(t *testing.T) {
		trace, err := client.BlockTrace(context.Background(), "0x47c3637b57c2b079b93c61539950c17e868a28f46cdef28f88521067f21e943")
		require.NoError(t, err)
		require.Len(t, trace.Traces, 18)
	})
}

func TestEventListener(t *testing.T) {
	isCalled := false
	client := feeder.NewTestClient(t, &utils.Mainnet).WithListener(&feeder.SelectiveListener{
		OnResponseCb: func(urlPath string, status int, _ time.Duration) {
			isCalled = true
			require.Equal(t, 200, status)
			require.Equal(t, "/get_block", urlPath)
		},
	})
	_, err := client.Block(context.Background(), "0")
	require.NoError(t, err)
	require.True(t, isCalled)
}
