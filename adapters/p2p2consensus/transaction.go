package p2p2consensus

import (
	"fmt"

	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	p2pconsensus "github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
)

//nolint:funlen
func AdaptTransaction(t *p2pconsensus.ConsensusTransaction, network *utils.Network) (core.Transaction, core.Class) {
	if t == nil {
		return nil, nil
	}

	switch t.Txn.(type) {
	case *p2pconsensus.ConsensusTransaction_DeclareV3:
		tx := t.GetDeclareV3()

		nDAMode, err := adaptVolitionDomain(tx.Common.NonceDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Nonce DA mode: %v to uint32", tx.Common.NonceDataAvailabilityMode))
		}

		fDAMode, err := adaptVolitionDomain(tx.Common.FeeDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Fee DA mode: %v to uint32", tx.Common.FeeDataAvailabilityMode))
		}

		class := AdaptClass(tx.Class)
		classHash, err := class.Hash()
		if err != nil {
			panic(err)
		}
		declareTx := &core.DeclareTransaction{
			TransactionHash:      p2p2core.AdaptHash(t.TransactionHash),
			ClassHash:            classHash,
			SenderAddress:        p2p2core.AdaptAddress(tx.Common.Sender),
			MaxFee:               nil, // in 3 version this field was removed
			TransactionSignature: adaptAccountSignature(tx.Common.Signature),
			Nonce:                p2p2core.AdaptFelt(tx.Common.Nonce),
			Version:              txVersion(3),
			CompiledClassHash:    p2p2core.AdaptHash(tx.Common.CompiledClassHash),
			Tip:                  tx.Common.Tip,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas:     adaptResourceLimits(tx.Common.ResourceBounds.L1Gas),
				core.ResourceL2Gas:     adaptResourceLimits(tx.Common.ResourceBounds.L2Gas),
				core.ResourceL1DataGas: adaptResourceLimits(tx.Common.ResourceBounds.L1DataGas),
			},
			PaymasterData:         utils.Map(tx.Common.PaymasterData, p2p2core.AdaptFelt),
			AccountDeploymentData: utils.Map(tx.Common.AccountDeploymentData, p2p2core.AdaptFelt),
			NonceDAMode:           nDAMode,
			FeeDAMode:             fDAMode,
		}

		return declareTx, class
	case *p2pconsensus.ConsensusTransaction_DeployAccountV3:
		tx := t.GetDeployAccountV3()

		nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Nonce DA mode: %v to uint32", tx.NonceDataAvailabilityMode))
		}

		fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Fee DA mode: %v to uint32", tx.FeeDataAvailabilityMode))
		}

		addressSalt := p2p2core.AdaptFelt(tx.AddressSalt)
		classHash := p2p2core.AdaptHash(tx.ClassHash)
		callData := utils.Map(tx.Calldata, p2p2core.AdaptFelt)
		deployAccTx := &core.DeployAccountTransaction{
			DeployTransaction: core.DeployTransaction{
				TransactionHash:     p2p2core.AdaptHash(t.TransactionHash),
				ContractAddressSalt: addressSalt,
				ContractAddress:     core.ContractAddress(&felt.Zero, classHash, addressSalt, callData),
				ClassHash:           classHash,
				ConstructorCallData: callData,
				Version:             txVersion(3),
			},
			MaxFee:               nil, // todo(kirill) update spec? missing field
			TransactionSignature: adaptAccountSignature(tx.Signature),
			Nonce:                p2p2core.AdaptFelt(tx.Nonce),
			Tip:                  tx.Tip,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas:     adaptResourceLimits(tx.ResourceBounds.L1Gas),
				core.ResourceL2Gas:     adaptResourceLimits(tx.ResourceBounds.L2Gas),
				core.ResourceL1DataGas: adaptResourceLimits(tx.ResourceBounds.L1DataGas),
			},
			PaymasterData: utils.Map(tx.PaymasterData, p2p2core.AdaptFelt),
			NonceDAMode:   nDAMode,
			FeeDAMode:     fDAMode,
		}

		return deployAccTx, nil
	case *p2pconsensus.ConsensusTransaction_InvokeV3:
		tx := t.GetInvokeV3()

		nDAMode, err := adaptVolitionDomain(tx.NonceDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Nonce DA mode: %v to uint32", tx.NonceDataAvailabilityMode))
		}

		fDAMode, err := adaptVolitionDomain(tx.FeeDataAvailabilityMode)
		if err != nil {
			panic(fmt.Sprintf("Failed to convert Fee DA mode: %v to uint32", tx.FeeDataAvailabilityMode))
		}

		invTx := &core.InvokeTransaction{
			TransactionHash:      p2p2core.AdaptHash(t.TransactionHash),
			ContractAddress:      nil, // todo call core.ContractAddress() ?
			CallData:             utils.Map(tx.Calldata, p2p2core.AdaptFelt),
			TransactionSignature: adaptAccountSignature(tx.Signature),
			MaxFee:               nil, // in 3 version this field was removed
			Version:              txVersion(3),
			Nonce:                p2p2core.AdaptFelt(tx.Nonce),
			SenderAddress:        p2p2core.AdaptAddress(tx.Sender),
			EntryPointSelector:   nil,
			Tip:                  tx.Tip,
			ResourceBounds: map[core.Resource]core.ResourceBounds{
				core.ResourceL1Gas:     adaptResourceLimits(tx.ResourceBounds.L1Gas),
				core.ResourceL2Gas:     adaptResourceLimits(tx.ResourceBounds.L2Gas),
				core.ResourceL1DataGas: adaptResourceLimits(tx.ResourceBounds.L1DataGas),
			},
			PaymasterData:         utils.Map(tx.PaymasterData, p2p2core.AdaptFelt),
			NonceDAMode:           nDAMode,
			FeeDAMode:             fDAMode,
			AccountDeploymentData: nil,
		}

		return invTx, nil
	case *p2pconsensus.ConsensusTransaction_L1Handler:
		tx := t.GetL1Handler()
		l1Tx := &core.L1HandlerTransaction{
			TransactionHash:    p2p2core.AdaptHash(t.TransactionHash),
			ContractAddress:    p2p2core.AdaptAddress(tx.Address),
			EntryPointSelector: p2p2core.AdaptFelt(tx.EntryPointSelector),
			Nonce:              p2p2core.AdaptFelt(tx.Nonce),
			CallData:           utils.Map(tx.Calldata, p2p2core.AdaptFelt),
			Version:            txVersion(0),
		}

		return l1Tx, nil
	default:
		panic(fmt.Errorf("unsupported tx type %T", t.Txn))
	}
}

func adaptResourceLimits(limits *transaction.ResourceLimits) core.ResourceBounds {
	return core.ResourceBounds{
		MaxAmount:       p2p2core.AdaptFelt(limits.MaxAmount).Uint64(),
		MaxPricePerUnit: p2p2core.AdaptFelt(limits.MaxPricePerUnit),
	}
}

func adaptAccountSignature(s *transaction.AccountSignature) []*felt.Felt {
	return utils.Map(s.Parts, p2p2core.AdaptFelt)
}

func txVersion(v uint64) *core.TransactionVersion {
	return new(core.TransactionVersion).SetUint64(v)
}

func adaptVolitionDomain(v common.VolitionDomain) (core.DataAvailabilityMode, error) {
	switch v {
	case common.VolitionDomain_L1:
		return core.DAModeL1, nil
	case common.VolitionDomain_L2:
		return core.DAModeL2, nil
	default:
		return 0, fmt.Errorf("unknown volition domain %d", v)
	}
}
