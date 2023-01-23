package datasource

import (
	"errors"

	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type Gateway struct {
	client *clients.GatewayClient
}

func NewGateway(n utils.Network) *Gateway {
	return &Gateway{
		client: clients.NewGatewayClient(n.URL()),
	}
}

// GetBlockByNumber gets the block for a given block number from the feeder gateway,
// then adapts it to the core.Block type.
func (g *Gateway) GetBlockByNumber(blockNumber uint64) (*core.Block, error) {
	return nil, errors.New("not implemented")
}

// GetTransaction gets the transaction for a given transaction hash from the feeder gateway,
// then adapts it to the appropriate core.Transaction types.
func (g *Gateway) GetTransaction(transactionHash *felt.Felt) (core.Transaction, error) {
	response, err := g.client.GetTransaction(transactionHash)
	if err != nil {
		return nil, err
	}

	declareTx, deployTx, invokeTx, err := adaptTransaction(response, g)
	if err != nil {
		return nil, err
	}

	if declareTx != nil {
		return declareTx, nil
	}

	if deployTx != nil {
		return deployTx, nil
	}

	return invokeTx, nil
}

func adaptTransaction(response *clients.TransactionStatus, g *Gateway) (*core.DeclareTransaction, *core.DeployTransaction, *core.InvokeTransaction, error) {
	txType := response.Transaction.Type
	switch txType {
	case "DECLARE":
		class, err := g.GetClass(response.Transaction.ClassHash)
		if err != nil {
			return nil, nil, nil, err
		}
		declareTx, err := adaptDeclareTransaction(response, class)
		if err != nil {
			return nil, nil, nil, err
		}
		return declareTx, nil, nil, nil
	case "DEPLOY":
		class, err := g.GetClass(response.Transaction.ClassHash)
		if err != nil {
			return nil, nil, nil, err
		}
		deployTx, err := adaptDeployTransaction(response, class)
		if err != nil {
			return nil, nil, nil, err
		}
		return nil, deployTx, nil, nil
	case "INVOKE":
		invokeTx := adaptInvokeTransaction(response)
		return nil, nil, invokeTx, nil
	default:
		return nil, nil, nil, nil
	}
}

func adaptDeclareTransaction(response *clients.TransactionStatus, class *core.Class) (*core.DeclareTransaction, error) {
	declareTx := new(core.DeclareTransaction)
	declareTx.SenderAddress = response.Transaction.SenderAddress
	declareTx.MaxFee = response.Transaction.MaxFee
	declareTx.Signature = response.Transaction.Signature
	declareTx.Nonce = response.Transaction.Nonce
	declareTx.Version = response.Transaction.Version

	declareTx.Class = *class

	return declareTx, nil
}

func adaptDeployTransaction(response *clients.TransactionStatus, class *core.Class) (*core.DeployTransaction, error) {
	deployTx := new(core.DeployTransaction)
	deployTx.ContractAddressSalt = response.Transaction.ContractAddressSalt
	deployTx.ConstructorCalldata = response.Transaction.ConstructorCalldata
	deployTx.CallerAddress = response.Transaction.ContractAddress
	deployTx.Version = response.Transaction.Version

	deployTx.Class = *class

	return deployTx, nil
}

func adaptInvokeTransaction(response *clients.TransactionStatus) *core.InvokeTransaction {
	invokeTx := new(core.InvokeTransaction)
	invokeTx.ContractAddress = response.Transaction.ContractAddress
	invokeTx.EntryPointSelector = response.Transaction.EntryPointSelector
	invokeTx.SenderAddress = response.Transaction.SenderAddress
	invokeTx.Nonce = response.Transaction.Nonce
	invokeTx.CallData = response.Transaction.Calldata
	invokeTx.Signature = response.Transaction.Signature
	invokeTx.MaxFee = response.Transaction.MaxFee
	invokeTx.Version = response.Transaction.Version

	return invokeTx
}

// GetClass gets the class for a given class hash from the feeder gateway,
// then adapts it to the core.Class type.
func (g *Gateway) GetClass(classHash *felt.Felt) (*core.Class, error) {
	response, err := g.client.GetClassDefinition(classHash)
	if err != nil {
		return nil, err
	}

	return adaptClass(response)
}

func adaptClass(response *clients.ClassDefinition) (*core.Class, error) {
	class := new(core.Class)
	class.APIVersion = new(felt.Felt).SetUint64(0)

	var externals []core.EntryPoint
	for _, v := range response.EntryPoints.External {
		externals = append(externals, core.EntryPoint{Selector: v.Selector, Offset: v.Offset})
	}
	class.Externals = externals

	var l1Handlers []core.EntryPoint
	for _, v := range response.EntryPoints.L1Handler {
		l1Handlers = append(l1Handlers, core.EntryPoint{Selector: v.Selector, Offset: v.Offset})
	}
	class.L1Handlers = l1Handlers

	var constructors []core.EntryPoint
	for _, v := range response.EntryPoints.Constructor {
		constructors = append(constructors, core.EntryPoint{Selector: v.Selector, Offset: v.Offset})
	}
	class.Constructors = constructors

	var builtins []*felt.Felt
	for _, v := range response.Program.Builtins {
		builtin := new(felt.Felt).SetBytes([]byte(v))
		builtins = append(builtins, builtin)
	}
	class.Builtins = builtins

	// TODO: programHash
	// programJson, err := json.Marshal(response)
	// if err != nil {
	// 	return nil, err
	// }
	// programHash, err := crypto.StarkNetKeccak(programJson)
	// if err != nil {
	// 	return nil, err
	// }

	// class.ProgramHash = programHash

	class.Bytecode = response.Program.Data

	return class, nil
}

// GetStateUpdate gets the state update for a given block number from the feeder gateway,
// then adapts it to the core.StateUpdate type.
func (g *Gateway) GetStateUpdate(blockNumber uint64) (*core.StateUpdate, error) {
	response, err := g.client.GetStateUpdate(blockNumber)
	if err != nil {
		return nil, err
	}

	return adaptStateUpdate(response)
}

func adaptStateUpdate(response *clients.StateUpdate) (*core.StateUpdate, error) {
	stateDiff := new(core.StateDiff)
	stateDiff.DeclaredContracts = response.StateDiff.DeclaredContracts
	for _, deployedContract := range response.StateDiff.DeployedContracts {
		stateDiff.DeployedContracts = append(stateDiff.DeployedContracts, core.DeployedContract{
			Address:   deployedContract.Address,
			ClassHash: deployedContract.ClassHash,
		})
	}

	stateDiff.Nonces = make(map[felt.Felt]*felt.Felt)
	for addrStr, nonce := range response.StateDiff.Nonces {
		addr, err := new(felt.Felt).SetString(addrStr)
		if err != nil {
			return nil, err
		}
		stateDiff.Nonces[*addr] = nonce
	}

	stateDiff.StorageDiffs = make(map[felt.Felt][]core.StorageDiff)
	for addrStr, diffs := range response.StateDiff.StorageDiffs {
		addr, err := new(felt.Felt).SetString(addrStr)
		if err != nil {
			return nil, err
		}

		stateDiff.StorageDiffs[*addr] = []core.StorageDiff{}
		for _, diff := range diffs {
			stateDiff.StorageDiffs[*addr] = append(stateDiff.StorageDiffs[*addr], core.StorageDiff{
				Key:   diff.Key,
				Value: diff.Value,
			})
		}
	}

	return &core.StateUpdate{
		BlockHash: response.BlockHash,
		NewRoot:   response.NewRoot,
		OldRoot:   response.OldRoot,
		StateDiff: stateDiff,
	}, nil
}
