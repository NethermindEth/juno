package rpc

import (
	"errors"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

// ChainID returns the chain ID of the currently configured network.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L542
func (h *Handler) ChainID() (*felt.Felt, *jsonrpc.Error) {
	return h.bcReader.Network().L2ChainIDFelt(), nil
}

func nilToZero(f *felt.Felt) *felt.Felt {
	if f == nil {
		return &felt.Zero
	}
	return f
}

// Nonce returns the nonce associated with the given address in the given block number
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L633
func (h *Handler) Nonce(id BlockID, address felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getNonce")

	nonce, err := stateReader.ContractNonce(&address)
	if err != nil {
		return nil, ErrContractNotFound
	}

	return nonce, nil
}

// StorageAt gets the value of the storage at the given address and key.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L110
func (h *Handler) StorageAt(address, key felt.Felt, id BlockID) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getStorageAt")

	value, err := stateReader.ContractStorage(&address, &key)
	if err != nil {
		return nil, ErrContractNotFound
	}

	return value, nil
}

// ClassHashAt gets the class hash for the contract deployed at the given address in the given block.
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/a789ccc3432c57777beceaa53a34a7ae2f25fda0/api/starknet_api_openrpc.json#L292
func (h *Handler) ClassHashAt(id BlockID, address felt.Felt) (*felt.Felt, *jsonrpc.Error) {
	stateReader, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getClassHashAt")

	classHash, err := stateReader.ContractClassHash(&address)
	if err != nil {
		return nil, ErrContractNotFound
	}

	return classHash, nil
}

// Class gets the contract class definition in the given block associated with the given hash
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L248
func (h *Handler) Class(id BlockID, classHash felt.Felt) (*Class, *jsonrpc.Error) {
	state, stateCloser, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(stateCloser, "Error closing state reader in getClass")

	declared, err := state.Class(&classHash)
	if err != nil {
		return nil, ErrClassHashNotFound
	}

	var rpcClass *Class
	switch c := declared.Class.(type) {
	case *core.Cairo0Class:
		adaptEntryPoint := func(ep core.EntryPoint) EntryPoint {
			return EntryPoint{
				Offset:   ep.Offset,
				Selector: ep.Selector,
			}
		}

		rpcClass = &Class{
			Abi:     c.Abi,
			Program: c.Program,
			EntryPoints: EntryPoints{
				// Note that utils.Map returns nil if provided slice is nil
				// but this is not the case here, because we rely on sn2core adapters that will set it to empty slice
				// if it's nil. In the API spec these fields are required.
				Constructor: utils.Map(c.Constructors, adaptEntryPoint),
				External:    utils.Map(c.Externals, adaptEntryPoint),
				L1Handler:   utils.Map(c.L1Handlers, adaptEntryPoint),
			},
		}
	case *core.Cairo1Class:
		adaptEntryPoint := func(ep core.SierraEntryPoint) EntryPoint {
			return EntryPoint{
				Index:    &ep.Index,
				Selector: ep.Selector,
			}
		}

		rpcClass = &Class{
			Abi:                  c.Abi,
			SierraProgram:        c.Program,
			ContractClassVersion: c.SemanticVersion,
			EntryPoints: EntryPoints{
				// Note that utils.Map returns nil if provided slice is nil
				// but this is not the case here, because we rely on sn2core adapters that will set it to empty slice
				// if it's nil. In the API spec these fields are required.
				Constructor: utils.Map(c.EntryPoints.Constructor, adaptEntryPoint),
				External:    utils.Map(c.EntryPoints.External, adaptEntryPoint),
				L1Handler:   utils.Map(c.EntryPoints.L1Handler, adaptEntryPoint),
			},
		}
	default:
		return nil, ErrClassHashNotFound
	}

	return rpcClass, nil
}

// ClassAt gets the contract class definition in the given block instantiated by the given contract address
//
// It follows the specification defined here:
// https://github.com/starkware-libs/starknet-specs/blob/master/api/starknet_api_openrpc.json#L329
func (h *Handler) ClassAt(id BlockID, address felt.Felt) (*Class, *jsonrpc.Error) {
	classHash, err := h.ClassHashAt(id, address)
	if err != nil {
		return nil, err
	}
	return h.Class(id, *classHash)
}

// https://github.com/starkware-libs/starknet-specs/blob/e0b76ed0d8d8eba405e182371f9edac8b2bcbc5a/api/starknet_api_openrpc.json#L401-L445
func (h *Handler) Call(funcCall FunctionCall, id BlockID) ([]*felt.Felt, *jsonrpc.Error) { //nolint:gocritic
	return h.call(funcCall, id, true)
}

func (h *Handler) CallV0_6(call FunctionCall, id BlockID) ([]*felt.Felt, *jsonrpc.Error) { //nolint:gocritic
	return h.call(call, id, false)
}

func (h *Handler) call(funcCall FunctionCall, id BlockID, useBlobData bool) ([]*felt.Felt, *jsonrpc.Error) { //nolint:gocritic
	state, closer, rpcErr := h.stateByBlockID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}
	defer h.callAndLogErr(closer, "Failed to close state in starknet_call")

	header, rpcErr := h.blockHeaderByID(&id)
	if rpcErr != nil {
		return nil, rpcErr
	}

	classHash, err := state.ContractClassHash(&funcCall.ContractAddress)
	if err != nil {
		return nil, ErrContractNotFound
	}

	blockHashToBeRevealed, err := h.getRevealedBlockHash(header.Number)
	if err != nil {
		return nil, ErrInternal.CloneWithData(err)
	}

	res, err := h.vm.Call(&vm.CallInfo{
		ContractAddress: &funcCall.ContractAddress,
		Selector:        &funcCall.EntryPointSelector,
		Calldata:        funcCall.Calldata,
		ClassHash:       classHash,
	}, &vm.BlockInfo{
		Header:                header,
		BlockHashToBeRevealed: blockHashToBeRevealed,
	}, state, h.bcReader.Network(), h.callMaxSteps, useBlobData)
	if err != nil {
		if errors.Is(err, utils.ErrResourceBusy) {
			return nil, ErrInternal.CloneWithData(throttledVMErr)
		}
		return nil, makeContractError(err)
	}
	return res, nil
}

func (h *Handler) EstimateFee(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, *jsonrpc.Error) {
	result, err := h.simulateTransactions(id, broadcastedTxns, append(simulationFlags, SkipFeeChargeFlag), false, true)
	if err != nil {
		return nil, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), nil
}

func (h *Handler) EstimateFeeV0_6(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, *jsonrpc.Error) {
	result, err := h.simulateTransactions(id, broadcastedTxns, append(simulationFlags, SkipFeeChargeFlag), true, true)
	if err != nil {
		return nil, err
	}

	return utils.Map(result, func(tx SimulatedTransaction) FeeEstimate {
		return tx.FeeEstimation
	}), nil
}

func (h *Handler) EstimateMessageFee(msg MsgFromL1, id BlockID) (*FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	return h.estimateMessageFee(msg, id, h.EstimateFee)
}

func (h *Handler) EstimateMessageFeeV0_6(msg MsgFromL1, id BlockID) (*FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	feeEstimate, rpcErr := h.estimateMessageFee(msg, id, h.EstimateFeeV0_6)
	if rpcErr != nil {
		return nil, rpcErr
	}

	feeEstimate.v0_6Response = true
	feeEstimate.DataGasPrice = nil
	feeEstimate.DataGasConsumed = nil

	return feeEstimate, nil
}

type estimateFeeHandler func(broadcastedTxns []BroadcastedTransaction,
	simulationFlags []SimulationFlag, id BlockID,
) ([]FeeEstimate, *jsonrpc.Error)

func (h *Handler) estimateMessageFee(msg MsgFromL1, id BlockID, f estimateFeeHandler) (*FeeEstimate, *jsonrpc.Error) { //nolint:gocritic
	calldata := make([]*felt.Felt, 0, len(msg.Payload)+1)
	// The order of the calldata parameters matters. msg.From must be prepended.
	calldata = append(calldata, new(felt.Felt).SetBytes(msg.From.Bytes()))
	for payloadIdx := range msg.Payload {
		calldata = append(calldata, &msg.Payload[payloadIdx])
	}
	tx := BroadcastedTransaction{
		Transaction: Transaction{
			Type:               TxnL1Handler,
			ContractAddress:    &msg.To,
			EntryPointSelector: &msg.Selector,
			CallData:           &calldata,
			Version:            &felt.Zero, // Needed for transaction hash calculation.
			Nonce:              &felt.Zero, // Needed for transaction hash calculation.
		},
		// Needed to marshal to blockifier type.
		// Must be greater than zero to successfully execute transaction.
		PaidFeeOnL1: new(felt.Felt).SetUint64(1),
	}
	estimates, rpcErr := f([]BroadcastedTransaction{tx}, nil, id)
	if rpcErr != nil {
		if rpcErr.Code == ErrTransactionExecutionError.Code {
			data := rpcErr.Data.(TransactionExecutionErrorData)
			return nil, makeContractError(errors.New(data.ExecutionError))
		}
		return nil, rpcErr
	}
	return &estimates[0], nil
}

type ContractErrorData struct {
	RevertError string `json:"revert_error"`
}

func makeContractError(err error) *jsonrpc.Error {
	return ErrContractError.CloneWithData(ContractErrorData{
		RevertError: err.Error(),
	})
}

func (h *Handler) stateByBlockID(id *BlockID) (core.StateReader, blockchain.StateCloser, *jsonrpc.Error) {
	var reader core.StateReader
	var closer blockchain.StateCloser
	var err error
	switch {
	case id.Latest:
		reader, closer, err = h.bcReader.HeadState()
	case id.Hash != nil:
		reader, closer, err = h.bcReader.StateAtBlockHash(id.Hash)
	case id.Pending:
		reader, closer, err = h.bcReader.PendingState()
	default:
		reader, closer, err = h.bcReader.StateAtBlockNumber(id.Number)
	}

	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil, nil, ErrBlockNotFound
		}
		return nil, nil, ErrInternal.CloneWithData(err)
	}
	return reader, closer, nil
}
