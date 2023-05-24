package feeder

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/ethereum/go-ethereum/common"
)

var _ starknetdata.StarknetData = (*Feeder)(nil)

type Feeder struct {
	client *feeder.Client
}

func New(client *feeder.Client) *Feeder {
	return &Feeder{
		client: client,
	}
}

// BlockByNumber gets the block for a given block number from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	return f.block(ctx, strconv.FormatUint(blockNumber, 10))
}

// BlockLatest gets the latest block from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockLatest(ctx context.Context) (*core.Block, error) {
	return f.block(ctx, "latest")
}

// BlockPending gets the pending block from the feeder,
// then adapts it to the core.Block type.
func (f *Feeder) BlockPending(ctx context.Context) (*core.Block, error) {
	return f.block(ctx, "pending")
}

func (f *Feeder) block(ctx context.Context, blockID string) (*core.Block, error) {
	response, err := f.client.Block(ctx, blockID)
	if err != nil {
		return nil, err
	}

	if blockID == "pending" && response.Status != "PENDING" {
		return nil, errors.New("no pending block")
	}
	return adaptBlock(response)
}

func adaptBlock(response *feeder.Block) (*core.Block, error) {
	if response == nil {
		return nil, errors.New("nil client block")
	}

	txns := make([]core.Transaction, len(response.Transactions))
	receipts := make([]*core.TransactionReceipt, len(response.Receipts))
	eventCount := uint64(0)
	for i, txn := range response.Transactions {
		var err error
		txns[i], err = adaptTransaction(txn)
		if err != nil {
			return nil, err
		}
		receipts[i] = adaptTransactionReceipt(response.Receipts[i])
		eventCount += uint64(len(response.Receipts[i].Events))
	}

	return &core.Block{
		Header: &core.Header{
			Hash:             response.Hash,
			ParentHash:       response.ParentHash,
			Number:           response.Number,
			GlobalStateRoot:  response.StateRoot,
			Timestamp:        response.Timestamp,
			ProtocolVersion:  response.Version,
			ExtraData:        nil,
			SequencerAddress: response.SequencerAddress,
			TransactionCount: uint64(len(response.Transactions)),
			EventCount:       eventCount,
			EventsBloom:      core.EventsBloom(receipts),
		},
		Transactions: txns,
		Receipts:     receipts,
	}, nil
}

func adaptTransactionReceipt(response *feeder.TransactionReceipt) *core.TransactionReceipt {
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

func adaptEvent(response *feeder.Event) *core.Event {
	if response == nil {
		return nil
	}

	return &core.Event{
		Data: response.Data,
		From: response.From,
		Keys: response.Keys,
	}
}

func adaptExecutionResources(response *feeder.ExecutionResources) *core.ExecutionResources {
	if response == nil {
		return nil
	}

	return &core.ExecutionResources{
		BuiltinInstanceCounter: adaptBuiltinInstanceCounter(response.BuiltinInstanceCounter),
		MemoryHoles:            response.MemoryHoles,
		Steps:                  response.Steps,
	}
}

func adaptBuiltinInstanceCounter(response feeder.BuiltinInstanceCounter) core.BuiltinInstanceCounter {
	return core.BuiltinInstanceCounter{
		Bitwise:    response.Bitwise,
		EcOp:       response.EcOp,
		Ecsda:      response.Ecsda,
		Output:     response.Output,
		Pedersen:   response.Pedersen,
		RangeCheck: response.RangeCheck,
	}
}

func adaptL1ToL2Message(response *feeder.L1ToL2Message) *core.L1ToL2Message {
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

func adaptL2ToL1Message(response *feeder.L2ToL1Message) *core.L2ToL1Message {
	if response == nil {
		return nil
	}

	return &core.L2ToL1Message{
		From:    response.From,
		Payload: response.Payload,
		To:      common.HexToAddress(response.To),
	}
}

// Transaction gets the transaction for a given transaction hash from the feeder,
// then adapts it to the appropriate core.Transaction types.
func (f *Feeder) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	response, err := f.client.Transaction(ctx, transactionHash)
	if err != nil {
		return nil, err
	}

	tx, err := adaptTransaction(response.Transaction)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func adaptTransaction(transaction *feeder.Transaction) (core.Transaction, error) {
	txType := transaction.Type
	switch txType {
	case "DECLARE":
		return adaptDeclareTransaction(transaction), nil
	case "DEPLOY":
		return adaptDeployTransaction(transaction), nil
	case "INVOKE_FUNCTION":
		return adaptInvokeTransaction(transaction), nil
	case "DEPLOY_ACCOUNT":
		return adaptDeployAccountTransaction(transaction), nil
	case "L1_HANDLER":
		return adaptL1HandlerTransaction(transaction), nil
	default:
		return nil, fmt.Errorf("unknown transaction type %q", txType)
	}
}

func adaptDeclareTransaction(t *feeder.Transaction) *core.DeclareTransaction {
	return &core.DeclareTransaction{
		TransactionHash:      t.Hash,
		SenderAddress:        t.SenderAddress,
		MaxFee:               t.MaxFee,
		TransactionSignature: t.Signature,
		Nonce:                t.Nonce,
		Version:              t.Version,
		ClassHash:            t.ClassHash,
		CompiledClassHash:    t.CompiledClassHash,
	}
}

func adaptDeployTransaction(t *feeder.Transaction) *core.DeployTransaction {
	return &core.DeployTransaction{
		TransactionHash:     t.Hash,
		ContractAddressSalt: t.ContractAddressSalt,
		ContractAddress:     t.ContractAddress,
		ClassHash:           t.ClassHash,
		ConstructorCallData: t.ConstructorCallData,
		Version:             t.Version,
	}
}

func adaptInvokeTransaction(t *feeder.Transaction) *core.InvokeTransaction {
	return &core.InvokeTransaction{
		TransactionHash:      t.Hash,
		ContractAddress:      t.ContractAddress,
		EntryPointSelector:   t.EntryPointSelector,
		Nonce:                t.Nonce,
		CallData:             t.CallData,
		TransactionSignature: t.Signature,
		MaxFee:               t.MaxFee,
		Version:              t.Version,
		SenderAddress:        t.SenderAddress,
	}
}

func adaptL1HandlerTransaction(t *feeder.Transaction) *core.L1HandlerTransaction {
	return &core.L1HandlerTransaction{
		TransactionHash:    t.Hash,
		ContractAddress:    t.ContractAddress,
		EntryPointSelector: t.EntryPointSelector,
		Nonce:              t.Nonce,
		CallData:           t.CallData,
		Version:            t.Version,
	}
}

func adaptDeployAccountTransaction(t *feeder.Transaction) *core.DeployAccountTransaction {
	return &core.DeployAccountTransaction{
		DeployTransaction:    *adaptDeployTransaction(t),
		MaxFee:               t.MaxFee,
		TransactionSignature: t.Signature,
		Nonce:                t.Nonce,
	}
}

// Class gets the class for a given class hash from the feeder,
// then adapts it to the core.Class type.
func (f *Feeder) Class(ctx context.Context, classHash *felt.Felt) (core.Class, error) {
	response, err := f.client.ClassDefinition(ctx, classHash)
	if err != nil {
		return nil, err
	}

	switch {
	case response.V1 != nil:
		compiledClass, cErr := f.client.CompiledClassDefinition(ctx, classHash)
		if cErr != nil {
			return nil, cErr
		}

		return adaptCairo1Class(response.V1, compiledClass)
	case response.V0 != nil:
		return adaptCairo0Class(response.V0)
	default:
		return nil, errors.New("empty class")
	}
}

func adaptCairo1Class(response *feeder.SierraDefinition, compiledClass json.RawMessage) (core.Class, error) {
	var err error

	class := new(core.Cairo1Class)
	class.SemanticVersion = response.Version
	class.Program = response.Program
	class.ProgramHash = crypto.PoseidonArray(class.Program...)

	class.Abi = response.Abi
	class.AbiHash, err = crypto.StarknetKeccak([]byte(class.Abi))
	if err != nil {
		return nil, err
	}

	class.EntryPoints.External = make([]core.SierraEntryPoint, len(response.EntryPoints.External))
	for index, v := range response.EntryPoints.External {
		class.EntryPoints.External[index] = core.SierraEntryPoint{Index: v.Index, Selector: v.Selector}
	}
	class.EntryPoints.L1Handler = make([]core.SierraEntryPoint, len(response.EntryPoints.L1Handler))
	for index, v := range response.EntryPoints.L1Handler {
		class.EntryPoints.L1Handler[index] = core.SierraEntryPoint{Index: v.Index, Selector: v.Selector}
	}
	class.EntryPoints.Constructor = make([]core.SierraEntryPoint, len(response.EntryPoints.Constructor))
	for index, v := range response.EntryPoints.Constructor {
		class.EntryPoints.Constructor[index] = core.SierraEntryPoint{Index: v.Index, Selector: v.Selector}
	}

	if err = json.Unmarshal(compiledClass, &class.Compiled); err != nil {
		return nil, err
	}
	return class, nil
}

func adaptCairo0Class(response *feeder.Cairo0Definition) (core.Class, error) {
	class := new(core.Cairo0Class)
	class.Abi = response.Abi

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

	var program feeder.Program
	if err := json.Unmarshal(response.Program, &program); err != nil {
		return nil, err
	}

	var compressedBuffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedBuffer)
	if _, err := gzipWriter.Write(response.Program); err != nil {
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	compressedProgram := compressedBuffer.Bytes()
	base64Program := make([]byte, base64.StdEncoding.EncodedLen(len(compressedProgram)))
	base64.StdEncoding.Encode(base64Program, compressedProgram)
	class.Program = string(base64Program)

	return class, nil
}

func (f *Feeder) stateUpdate(ctx context.Context, blockID string) (*core.StateUpdate, error) {
	response, err := f.client.StateUpdate(ctx, blockID)
	if err != nil {
		return nil, err
	}

	return adaptStateUpdate(response)
}

// StateUpdate gets the state update for a given block number from the feeder,
// then adapts it to the core.StateUpdate type.
func (f *Feeder) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	return f.stateUpdate(ctx, strconv.FormatUint(blockNumber, 10))
}

// StateUpdatePending gets the state update for the pending block from the feeder,
// then adapts it to the core.StateUpdate type.
func (f *Feeder) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	return f.stateUpdate(ctx, "pending")
}

func adaptStateUpdate(response *feeder.StateUpdate) (*core.StateUpdate, error) {
	stateDiff := new(core.StateDiff)
	stateDiff.DeclaredV0Classes = response.StateDiff.OldDeclaredContracts

	stateDiff.DeclaredV1Classes = make([]core.DeclaredV1Class, len(response.StateDiff.DeclaredClasses))
	for index, declaredV1Class := range response.StateDiff.DeclaredClasses {
		stateDiff.DeclaredV1Classes[index] = core.DeclaredV1Class{
			ClassHash:         declaredV1Class.ClassHash,
			CompiledClassHash: declaredV1Class.CompiledClassHash,
		}
	}

	stateDiff.ReplacedClasses = make([]core.ReplacedClass, len(response.StateDiff.ReplacedClasses))
	for index, replacedClass := range response.StateDiff.ReplacedClasses {
		stateDiff.ReplacedClasses[index] = core.ReplacedClass{
			Address:   replacedClass.Address,
			ClassHash: replacedClass.ClassHash,
		}
	}

	stateDiff.DeployedContracts = make([]core.DeployedContract, len(response.StateDiff.DeployedContracts))
	for index, deployedContract := range response.StateDiff.DeployedContracts {
		stateDiff.DeployedContracts[index] = core.DeployedContract{
			Address:   deployedContract.Address,
			ClassHash: deployedContract.ClassHash,
		}
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
