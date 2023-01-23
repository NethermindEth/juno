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

// BlockByNumber gets the block for a given block number from the feeder gateway,
// then adapts it to the core.Block type.
func (g *Gateway) BlockByNumber(blockNumber uint64) (*core.Block, error) {
	response, err := g.client.GetBlock(blockNumber)
	if err != nil {
		return nil, err
	}

	return adaptBlock(response)
}

func adaptBlock(response *clients.Block) (*core.Block, error) {
	if response == nil {
		return nil, nil
	}

	// Receipts
	receipts := make([]*core.TransactionReceipt, len(response.Receipts))
	for i, receipt := range response.Receipts {
		receipts[i] = adaptTransactionReceipt(receipt)
	}

	// Events
	_, eventCount, err := core.EventData(receipts)
	if err != nil {
		return nil, err
	}

	// Transaction Commitment
	txCommitment, err := core.TransactionCommitment(receipts)
	if err != nil {
		return nil, err
	}

	return &core.Block{
		ParentHash:            response.ParentHash,
		Number:                response.Number,
		GlobalStateRoot:       response.StateRoot,
		Timestamp:             new(felt.Felt).SetUint64(response.Timestamp),
		TransactionCount:      new(felt.Felt).SetUint64(uint64(len(response.Transactions))),
		TransactionCommitment: txCommitment,
		EventCount:            new(felt.Felt).SetUint64(eventCount),
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

// Transaction gets the transaction for a given transaction hash from the feeder gateway,
// then adapts it to the appropriate core.Transaction types.
func (g *Gateway) Transaction(transactionHash *felt.Felt) (*core.Transaction, error) {
	return nil, errors.New("not implemented")
}

// Class gets the class for a given class hash from the feeder gateway,
// then adapts it to the core.Class type.
func (g *Gateway) Class(classHash *felt.Felt) (*core.Class, error) {
	return nil, errors.New("not implemented")
}

// StateUpdate gets the state update for a given block number from the feeder gateway,
// then adapts it to the core.StateUpdate type.
func (g *Gateway) StateUpdate(blockNumber uint64) (*core.StateUpdate, error) {
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
