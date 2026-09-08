package rpcv10

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/pending"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	"github.com/NethermindEth/juno/vm"
)

func (h *Handler) traceProgressiveBlock(
	ctx context.Context,
	header *core.Header,
	transactions []core.Transaction,
	target uint64,
	returnInitialReads bool,
) (TraceBlockTransactionsResponse, http.Header, *jsonrpc.Error) {
	if target >= uint64(len(transactions)) {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(),
			rpccore.ErrUnexpectedError.CloneWithData(fmt.Sprintf(
				"trace target index %d out of range for %d transactions", target, len(transactions),
			))
	}
	if returnInitialReads && target != uint64(len(transactions)-1) {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(),
			rpccore.ErrUnexpectedError.CloneWithData("initial reads require a complete block trace")
	}

	blockHash := *header.Hash
	for {
		lookup := h.blockTraceCache.lookupOrStart(blockHash, target, returnInitialReads)
		switch lookup.kind {
		case traceCacheHit:
			return lookup.response, defaultExecutionHeader(), nil
		case traceCacheWait:
			select {
			case <-ctx.Done():
				return TraceBlockTransactionsResponse{}, defaultExecutionHeader(),
					rpccore.ErrUnexpectedError.CloneWithData(ctx.Err().Error())
			case <-lookup.done:
				continue
			}
		case traceCacheExecute:
			work := lookup.work
			//nolint:gocritic // safe to defer in loop: every execution path below returns or panics.
			defer work.abort()
			response, responseHeader, rpcErr := h.executeTraceRange(
				header, transactions, work.record.traces, target, returnInitialReads,
			)
			if rpcErr != nil {
				return TraceBlockTransactionsResponse{}, responseHeader, rpcErr
			}
			return work.commit(response, len(transactions)), responseHeader, nil
		default:
			panic("unknown trace cache lookup result")
		}
	}
}

// executeTraceRange traces from the end of cachedPrefix through target, inclusive.
// An empty prefix starts at the parent state, including when replaying to collect initial reads.
func (h *Handler) executeTraceRange(
	header *core.Header,
	transactions []core.Transaction,
	cachedPrefix []TracedBlockTransaction,
	target uint64,
	returnInitialReads bool,
) (TraceBlockTransactionsResponse, http.Header, *jsonrpc.Error) {
	start := uint64(len(cachedPrefix))
	parentState, parentCloser, err := h.bcReader.StateAtBlockHash(header.ParentHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpccore.ErrBlockNotFound
		}
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(),
			rpccore.ErrInternal.CloneWithData(err)
	}
	defer h.callAndLogErr(parentCloser, "Failed to close parent state after trace execution")

	headState, headCloser, err := h.bcReader.HeadState()
	if err != nil {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(),
			jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headCloser, "Failed to close head state after trace execution")

	executionState := parentState
	if start > 0 {
		checkpoint := checkpointFromTraces(cachedPrefix)
		declaredClasses, rpcErr := loadCheckpointClasses(&checkpoint, headState)
		if rpcErr != nil {
			return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpcErr
		}
		executionState = pending.NewState(&checkpoint, declaredClasses, parentState, header.Number)
	}

	blockInfo, rpcErr := h.buildBlockInfo(header)
	if rpcErr != nil {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpcErr
	}
	traces, vmInitialReads, responseHeader, rpcErr := traceTransactionsWithState(
		h.vm,
		transactions[start:target+1],
		executionState,
		headState,
		&blockInfo,
		vm.TraceOptions{ReturnInitialReads: returnInitialReads},
		start,
	)
	if rpcErr != nil {
		return TraceBlockTransactionsResponse{}, responseHeader, rpcErr
	}
	// The production Rust VM serialises state_diff as a required field for every
	// successful transaction trace. Enforce that contract before a trace becomes
	// an extendable cache record.
	for index := range traces {
		if traces[index].TraceRoot.StateDiff == nil {
			return TraceBlockTransactionsResponse{}, responseHeader,
				rpccore.ErrUnexpectedError.CloneWithData(fmt.Sprintf(
					"VM omitted state diff for transaction trace %d", start+uint64(index),
				))
		}
	}
	response := TraceBlockTransactionsResponse{Traces: traces}
	if returnInitialReads {
		if vmInitialReads == nil {
			return TraceBlockTransactionsResponse{}, responseHeader,
				rpccore.ErrUnexpectedError.CloneWithData("VM omitted initial reads for block trace")
		}
		adaptedReads := adaptVMInitialReads(vmInitialReads)
		response.InitialReads = &adaptedReads
	}
	return response, responseHeader, nil
}

func loadCheckpointClasses(
	diff *core.StateDiff,
	classLookup core.StateReader,
) (map[felt.Felt]core.ClassDefinition, *jsonrpc.Error) {
	classes := make(
		map[felt.Felt]core.ClassDefinition,
		len(diff.DeclaredV0Classes)+len(diff.DeclaredV1Classes),
	)
	for _, hash := range diff.DeclaredV0Classes {
		classes[*hash] = nil
	}
	for hash := range diff.DeclaredV1Classes {
		classes[hash] = nil
	}
	for hash := range classes {
		declared, err := classLookup.Class(&hash)
		if err != nil {
			return nil, jsonrpc.Err(jsonrpc.InternalError, err.Error())
		}
		classes[hash] = declared.Class
	}
	return classes, nil
}

// checkpointFromTraces rebuilds the continuation checkpoint from cached per-transaction state
// diffs. Progressive records only contain traces with non-nil state diffs; executeTraceRange
// enforces this invariant before publishing an extension. The small recomputation cost avoids
// retaining a duplicate cumulative state diff in the cache.
func checkpointFromTraces(traces []TracedBlockTransaction) core.StateDiff {
	result := core.EmptyStateDiff()
	for index := range traces {
		mergeRPCStateDiff(&result, traces[index].TraceRoot.StateDiff)
	}
	return result
}

func mergeRPCStateDiff(result *core.StateDiff, diff *StateDiff) {
	for _, storage := range diff.StorageDiffs {
		entries, found := result.StorageDiffs[storage.Address]
		if !found {
			entries = make(map[felt.Felt]*felt.Felt, len(storage.StorageEntries))
			result.StorageDiffs[storage.Address] = entries
		}
		for _, entry := range storage.StorageEntries {
			entries[entry.Key] = entry.Value.Clone()
		}
	}
	for _, nonce := range diff.Nonces {
		result.Nonces[nonce.ContractAddress] = nonce.Nonce.Clone()
	}
	for _, deployed := range diff.DeployedContracts {
		result.DeployedContracts[deployed.Address] = deployed.ClassHash.Clone()
	}
	for _, hash := range diff.DeprecatedDeclaredClasses {
		result.DeclaredV0Classes = append(result.DeclaredV0Classes, hash.Clone())
	}
	for _, declared := range diff.DeclaredClasses {
		result.DeclaredV1Classes[declared.ClassHash] = declared.CompiledClassHash.Clone()
	}
	for _, replaced := range diff.ReplacedClasses {
		result.ReplacedClasses[replaced.ContractAddress] = replaced.ClassHash.Clone()
	}
	for _, migrated := range diff.MigratedCompiledClasses {
		result.MigratedClasses[migrated.ClassHash] = migrated.CompiledClassHash
	}
}
