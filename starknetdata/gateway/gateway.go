package gateway

import (
	"context"
	"errors"

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

func NewGatewayWithClient(client *clients.GatewayClient) *Gateway {
	return &Gateway{
		client: client,
	}
}

// BlockByNumber gets the block for a given block number from the feeder gateway,
// then adapts it to the core.Block type.
func (g *Gateway) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	response, err := g.client.GetBlock(ctx, blockNumber)
	if err != nil {
		return nil, err
	}

	return AdaptBlock(response)
}

func AdaptBlock(response *clients.Block) (*core.Block, error) {
	if response == nil {
		return nil, errors.New("nil client block")
	}

	txns := make([]core.Transaction, len(response.Transactions))
	receipts := make([]*core.TransactionReceipt, len(response.Receipts))
	for i, txn := range response.Transactions {
		var err error
		txns[i], err = adaptTransaction(txn)
		if err != nil {
			return nil, err
		}
		receipts[i] = adaptTransactionReceipt(response.Receipts[i])
	}

	return &core.Block{
		Header: core.Header{
			Hash:             response.Hash,
			ParentHash:       response.ParentHash,
			Number:           response.Number,
			GlobalStateRoot:  response.StateRoot,
			Timestamp:        new(felt.Felt).SetUint64(response.Timestamp),
			ProtocolVersion:  response.Version,
			ExtraData:        nil,
			SequencerAddress: response.SequencerAddress,
		},
		Transactions: txns,
		Receipts:     receipts,
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
		Fee:                response.ActualFee,
		TransactionHash:    response.TransactionHash,
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

// Transaction gets the transaction for a given transaction hash from the feeder gateway,
// then adapts it to the appropriate core.Transaction types.
func (g *Gateway) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	response, err := g.client.GetTransaction(ctx, transactionHash)
	if err != nil {
		return nil, err
	}

	tx, err := adaptTransaction(response.Transaction)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func adaptTransaction(transaction *clients.Transaction) (core.Transaction, error) {
	txType := transaction.Type
	switch txType {
	case "DECLARE":
		return adaptDeclareTransaction(transaction), nil
	case "DEPLOY":
		return adaptDeployTransaction(transaction), nil
	case "INVOKE_FUNCTION":
		return adaptInvokeTransaction(transaction), nil
	case "DEPLOY_ACCOUNT":
		return adaptDeployAccountTransaction(transaction), nil // todo
	case "L1_HANDLER":
		return adaptL1HandlerTransaction(transaction), nil // todo
	default:
		return nil, core.ErrUnknownTransaction
	}
}

func adaptDeclareTransaction(t *clients.Transaction) *core.DeclareTransaction {
	return &core.DeclareTransaction{
		Hash:          t.Hash,
		SenderAddress: t.SenderAddress,
		MaxFee:        t.MaxFee,
		Signature:     t.Signature,
		Nonce:         t.Nonce,
		Version:       t.Version,
		ClassHash:     t.ClassHash,
	}
}

func adaptDeployTransaction(t *clients.Transaction) *core.DeployTransaction {
	return &core.DeployTransaction{
		Hash:                t.Hash,
		ContractAddressSalt: t.ContractAddressSalt,
		ContractAddress:     t.ContractAddress,
		ClassHash:           t.ClassHash,
		ConstructorCallData: t.ConstructorCalldata,
		Version:             t.Version,
	}
}

func adaptInvokeTransaction(t *clients.Transaction) *core.InvokeTransaction {
	return &core.InvokeTransaction{
		Hash:               t.Hash,
		ContractAddress:    t.ContractAddress,
		EntryPointSelector: t.EntryPointSelector,
		Nonce:              t.Nonce,
		CallData:           t.Calldata,
		Signature:          t.Signature,
		MaxFee:             t.MaxFee,
		Version:            t.Version,
	}
}

func adaptL1HandlerTransaction(t *clients.Transaction) *core.L1HandlerTransaction {
	return &core.L1HandlerTransaction{
		Hash: t.Hash,
	}
}

func adaptDeployAccountTransaction(t *clients.Transaction) *core.DeployAccountTransaction {
	return &core.DeployAccountTransaction{
		Hash:      t.Hash,
		Signature: t.Signature,
	}
}

// Class gets the class for a given class hash from the feeder gateway,
// then adapts it to the core.Class type.
func (g *Gateway) Class(ctx context.Context, classHash *felt.Felt) (*core.Class, error) {
	response, err := g.client.GetClassDefinition(ctx, classHash)
	if err != nil {
		return nil, err
	}

	return adaptClass(response)
}

func adaptClass(response *clients.ClassDefinition) (*core.Class, error) {
	class := new(core.Class)
	class.APIVersion = new(felt.Felt).SetUint64(0)

	class.Externals = []core.EntryPoint{}
	for _, v := range response.EntryPoints.External {
		class.Externals = append(class.Externals, core.EntryPoint{Selector: v.Selector, Offset: v.Offset})
	}

	class.L1Handlers = []core.EntryPoint{}
	for _, v := range response.EntryPoints.L1Handler {
		class.L1Handlers = append(class.L1Handlers, core.EntryPoint{Selector: v.Selector, Offset: v.Offset})
	}

	class.Constructors = []core.EntryPoint{}
	for _, v := range response.EntryPoints.Constructor {
		class.Constructors = append(class.Constructors, core.EntryPoint{Selector: v.Selector, Offset: v.Offset})
	}

	class.Builtins = []*felt.Felt{}
	for _, v := range response.Program.Builtins {
		builtin := new(felt.Felt).SetBytes([]byte(v))
		class.Builtins = append(class.Builtins, builtin)
	}

	var err error
	class.ProgramHash, err = clients.ProgramHash(response)
	if err != nil {
		return nil, err
	}

	class.Bytecode = []*felt.Felt{}
	for _, v := range response.Program.Data {
		datum, err := new(felt.Felt).SetString(v)
		if err != nil {
			return nil, err
		}

		class.Bytecode = append(class.Bytecode, datum)
	}

	return class, nil
}

// StateUpdate gets the state update for a given block number from the feeder gateway,
// then adapts it to the core.StateUpdate type.
func (g *Gateway) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	response, err := g.client.GetStateUpdate(ctx, blockNumber)
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
