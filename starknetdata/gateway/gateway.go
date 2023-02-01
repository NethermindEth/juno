package gateway

import (
	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
)

type Gateway struct {
	client *clients.GatewayClient
}

func NewGateway(n utils.Network) *Gateway {
	return &Gateway{
		client: clients.NewGatewayClient(n.URL()),
	}
}

// BlockByNumber gets the block for a given block number from the feeder gateway,
// then adapts it to the core.Block type.
func (g *Gateway) BlockByNumber(blockNumber uint64) (*core.Block, error) {
	response, err := g.client.GetBlock(blockNumber)
	if err != nil {
		return nil, err
	}

	return AdaptBlock(response)
}

func AdaptBlock(response *clients.Block) (*core.Block, error) {
	if response == nil {
		return nil, nil
	}

	// Receipts
	receipts := make([]*core.TransactionReceipt, len(response.Receipts))
	for i, receipt := range response.Receipts {
		receipts[i] = adaptTransactionReceipt(receipt)
	}

	// Events
	eventCommitment, eventCount, err := core.EventCommitmentAndCount(receipts)
	if err != nil {
		return nil, err
	}

	// Transaction Commitment
	txCommitment, err := core.TransactionCommitment(receipts)
	if err != nil {
		return nil, err
	}

	return &core.Block{
		Hash:                  response.Hash,
		ParentHash:            response.ParentHash,
		Number:                response.Number,
		GlobalStateRoot:       response.StateRoot,
		Timestamp:             new(felt.Felt).SetUint64(response.Timestamp),
		TransactionCount:      new(felt.Felt).SetUint64(uint64(len(response.Transactions))),
		TransactionCommitment: txCommitment,
		EventCount:            new(felt.Felt).SetUint64(eventCount),
		EventCommitment:       eventCommitment,
		ProtocolVersion:       new(felt.Felt),
		ExtraData:             nil,
	}, nil
}

func adaptTransactionReceipt(response *clients.TransactionReceipt) *core.TransactionReceipt {
	if response == nil {
		return nil
	}

	events := make([]*core.Event, len(response.Events))
	for i, event := range response.Events {
		events[i] = adaptEvent(event)
	}

	l2ToL1Messages := make([]*core.L2ToL1Message, len(response.L2ToL1Message))
	for i, msg := range response.L2ToL1Message {
		l2ToL1Messages[i] = adaptL2ToL1Message(msg)
	}

	return &core.TransactionReceipt{
		ActualFee:          response.ActualFee,
		TransactionHash:    response.TransactionHash,
		TransactionIndex:   response.TransactionIndex,
		Events:             events,
		ExecutionResources: adaptExecutionResources(response.ExecutionResources),
		L1ToL2Message:      adaptL1ToL2Message(response.L1ToL2Message),
		L2ToL1Message:      l2ToL1Messages,
	}
}

func adaptEvent(response *clients.Event) *core.Event {
	if response == nil {
		return nil
	}

	return &core.Event{
		Data: response.Data,
		From: response.From,
		Keys: response.Keys,
	}
}

func adaptExecutionResources(response *clients.ExecutionResources) *core.ExecutionResources {
	if response == nil {
		return nil
	}

	return &core.ExecutionResources{
		BuiltinInstanceCounter: adaptBuiltinInstanceCounter(response.BuiltinInstanceCounter),
		MemoryHoles:            response.MemoryHoles,
		Steps:                  response.Steps,
	}
}

func adaptBuiltinInstanceCounter(response clients.BuiltinInstanceCounter) core.BuiltinInstanceCounter {
	return core.BuiltinInstanceCounter{
		Bitwise:    response.Bitwise,
		EcOp:       response.EcOp,
		Ecsda:      response.Ecsda,
		Output:     response.Output,
		Pedersen:   response.Pedersen,
		RangeCheck: response.RangeCheck,
	}
}

func adaptL1ToL2Message(response *clients.L1ToL2Message) *core.L1ToL2Message {
	if response == nil {
		return nil
	}

	return &core.L1ToL2Message{
		From:     common.HexToAddress(response.From),
		Nonce:    response.Nonce,
		Payload:  response.Payload,
		Selector: response.Selector,
		To:       response.To,
	}
}

func adaptL2ToL1Message(response *clients.L2ToL1Message) *core.L2ToL1Message {
	if response == nil {
		return nil
	}

	return &core.L2ToL1Message{
		From:    response.From,
		Payload: response.Payload,
		To:      common.HexToAddress(response.To),
	}
}

// GetTransaction gets the transaction for a given transaction hash from the feeder gateway,
// then adapts it to the appropriate core.Transaction types.
func (g *Gateway) Transaction(transactionHash *felt.Felt) (core.Transaction, error) {
	response, err := g.client.GetTransaction(transactionHash)
	if err != nil {
		return nil, err
	}

	tx, err := adaptTransaction(response, g)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func adaptTransaction(response *clients.TransactionStatus, g *Gateway) (core.Transaction, error) {
	txType := response.Transaction.Type
	switch txType {
	case "DECLARE":
		declareTx, err := adaptDeclareTransaction(response)
		if err != nil {
			return nil, err
		}
		return declareTx, nil
	case "DEPLOY":
		deployTx, err := adaptDeployTransaction(response)
		if err != nil {
			return nil, err
		}
		return deployTx, nil
	case "INVOKE":
		invokeTx := adaptInvokeTransaction(response)
		return invokeTx, nil
	default:
		return nil, nil
	}
}

func adaptDeclareTransaction(response *clients.TransactionStatus) (*core.DeclareTransaction, error) {
	declareTx := new(core.DeclareTransaction)
	declareTx.SenderAddress = response.Transaction.SenderAddress
	declareTx.MaxFee = response.Transaction.MaxFee
	declareTx.Signature = response.Transaction.Signature
	declareTx.Nonce = response.Transaction.Nonce
	declareTx.Version = response.Transaction.Version
	declareTx.ClassHash = response.Transaction.ClassHash

	return declareTx, nil
}

func adaptDeployTransaction(response *clients.TransactionStatus) (*core.DeployTransaction, error) {
	deployTx := new(core.DeployTransaction)
	deployTx.ContractAddressSalt = response.Transaction.ContractAddressSalt
	deployTx.ConstructorCallData = response.Transaction.ConstructorCalldata
	deployTx.CallerAddress = response.Transaction.ContractAddress
	deployTx.Version = response.Transaction.Version
	deployTx.ClassHash = response.Transaction.ClassHash

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
func (g *Gateway) Class(classHash *felt.Felt) (*core.Class, error) {
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

	programHash, err := clients.ProgramHash(response)
	if err != nil {
		return nil, err
	}
	class.ProgramHash = programHash

	var data []*felt.Felt
	for _, v := range response.Program.Data {
		datum := new(felt.Felt).SetBytes([]byte(v))
		data = append(data, datum)
	}
	class.Bytecode = data

	return class, nil
}

// StateUpdate gets the state update for a given block number from the feeder gateway,
// then adapts it to the core.StateUpdate type.
func (g *Gateway) StateUpdate(blockNumber uint64) (*core.StateUpdate, error) {
	response, err := g.client.GetStateUpdate(blockNumber)
	if err != nil {
		return nil, err
	}

	return AdaptStateUpdate(response)
}

func AdaptStateUpdate(response *clients.StateUpdate) (*core.StateUpdate, error) {
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
