package gateway

import (
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
	var txType core.TransactionType
	for i, receipt := range response.Receipts {
		// Todo: TransactionReceipt's Type and Signature will be removed in future,
		// which is why explict test for AdaptTransactionReceipt is not being added.
		t := response.Transactions[i]
		switch t.Type {
		case "DEPLOY":
			txType = core.Deploy
		case "DEPLOY_ACCOUNT":
			txType = core.DeployAccount
		case "INVOKE_FUNCTION":
			txType = core.Invoke
		case "DECLARE":
			txType = core.Declare
		case "L1_HANDLER":
			txType = core.L1Handler
		default:
			return nil, errors.New("unknown transaction")

		}
		receipts[i] = adaptTransactionReceipt(receipt, txType, t.Signature)
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
		SequencerAddress:      response.SequencerAddress,
	}, nil
}

func adaptTransactionReceipt(response *clients.TransactionReceipt,
	txType core.TransactionType, signature []*felt.Felt,
) *core.TransactionReceipt {
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
		Type:               txType,
		Signatures:         signature,
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
func (g *Gateway) Transaction(transactionHash *felt.Felt) (core.Transaction, error) {
	response, err := g.client.GetTransaction(transactionHash)
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
		declareTx, err := adaptDeclareTransaction(transaction)
		if err != nil {
			return nil, err
		}
		return declareTx, nil
	case "DEPLOY":
		deployTx, err := adaptDeployTransaction(transaction)
		if err != nil {
			return nil, err
		}
		return deployTx, nil
	case "INVOKE_FUNCTION":
		invokeTx := adaptInvokeTransaction(transaction)
		return invokeTx, nil
	default:
		return nil, errors.New("txn type not recognized")
	}
}

func adaptDeclareTransaction(transaction *clients.Transaction) (*core.DeclareTransaction, error) {
	declareTx := new(core.DeclareTransaction)
	declareTx.SenderAddress = transaction.SenderAddress
	declareTx.MaxFee = transaction.MaxFee
	declareTx.Signature = transaction.Signature
	declareTx.Nonce = transaction.Nonce
	declareTx.Version = transaction.Version
	declareTx.ClassHash = transaction.ClassHash

	return declareTx, nil
}

func adaptDeployTransaction(transaction *clients.Transaction) (*core.DeployTransaction, error) {
	deployTx := new(core.DeployTransaction)
	deployTx.ContractAddressSalt = transaction.ContractAddressSalt
	deployTx.ConstructorCallData = transaction.ConstructorCalldata
	deployTx.CallerAddress = new(felt.Felt)
	deployTx.Version = transaction.Version
	deployTx.ClassHash = transaction.ClassHash
	deployTx.ContractAddress = core.ContractAddress(deployTx.CallerAddress,
		deployTx.ClassHash, deployTx.ContractAddressSalt, deployTx.ConstructorCallData)
	return deployTx, nil
}

func adaptInvokeTransaction(transaction *clients.Transaction) *core.InvokeTransaction {
	invokeTx := new(core.InvokeTransaction)
	invokeTx.ContractAddress = transaction.ContractAddress
	invokeTx.EntryPointSelector = transaction.EntryPointSelector
	invokeTx.SenderAddress = transaction.ContractAddress // todo
	invokeTx.Nonce = transaction.Nonce
	invokeTx.CallData = transaction.Calldata
	invokeTx.Signature = transaction.Signature
	invokeTx.MaxFee = transaction.MaxFee
	invokeTx.Version = transaction.Version

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
