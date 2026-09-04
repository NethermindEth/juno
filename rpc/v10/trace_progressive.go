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

	blockHash := *header.Hash
	for {
		lookup := h.blockTraceCache.lookupOrStart(blockHash, target)
		switch lookup.kind {
		case traceCacheHit:
			return shapeTraceResponse(lookup.response, returnInitialReads), defaultExecutionHeader(), nil
		case traceCacheWait:
			select {
			case <-ctx.Done():
				return TraceBlockTransactionsResponse{}, defaultExecutionHeader(),
					rpccore.ErrUnexpectedError.CloneWithData(ctx.Err().Error())
			case <-lookup.done:
				continue
			}
		case traceCacheExtend:
			response, responseHeader, rpcErr := h.executeTraceCacheWork(
				lookup.work, header, transactions, target,
			)
			return shapeTraceResponse(response, returnInitialReads), responseHeader, rpcErr
		default:
			panic("unknown trace cache lookup result")
		}
	}
}

func (h *Handler) executeTraceCacheWork(
	work *traceCacheWork,
	header *core.Header,
	transactions []core.Transaction,
	target uint64,
) (TraceBlockTransactionsResponse, http.Header, *jsonrpc.Error) {
	// Always release waiters, including when VM execution or adaptation panics.
	defer work.abort()

	response, responseHeader, rpcErr := h.executeTraceExtension(
		header, transactions, work.prefix(), target,
	)
	if rpcErr != nil {
		return TraceBlockTransactionsResponse{}, responseHeader, rpcErr
	}

	return work.commit(response, len(transactions)), responseHeader, nil
}

func (h *Handler) executeTraceExtension(
	header *core.Header,
	transactions []core.Transaction,
	cachedPrefix []TracedBlockTransaction,
	target uint64,
) (TraceBlockTransactionsResponse, http.Header, *jsonrpc.Error) {
	start := uint64(len(cachedPrefix))
	base, baseCloser, err := h.bcReader.StateAtBlockHash(header.ParentHash)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpccore.ErrBlockNotFound
		}
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(),
			rpccore.ErrInternal.CloneWithData(err)
	}
	defer h.callAndLogErr(baseCloser, "Failed to close base state after trace extension")

	headState, headCloser, err := h.bcReader.HeadState()
	if err != nil {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(),
			jsonrpc.Err(jsonrpc.InternalError, err.Error())
	}
	defer h.callAndLogErr(headCloser, "Failed to close head state after trace extension")

	executionState := base
	if start > 0 {
		checkpoint := checkpointFromTraces(cachedPrefix)
		newClasses, rpcErr := checkpointClasses(&checkpoint, headState)
		if rpcErr != nil {
			return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpcErr
		}
		executionState = pending.NewState(&checkpoint, newClasses, base, header.Number)
	}

	blockInfo, rpcErr := h.buildBlockInfo(header)
	if rpcErr != nil {
		return TraceBlockTransactionsResponse{}, defaultExecutionHeader(), rpcErr
	}
	// Always collect reads for future flagged requests; stitching retains Blockifier's earliest
	// pre-state value.
	traces, vmInitialReads, responseHeader, rpcErr := traceTransactionsWithState(
		h.vm,
		transactions[start:target+1],
		executionState,
		headState,
		&blockInfo,
		vm.TraceOptions{ReturnInitialReads: true},
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
	if vmInitialReads == nil {
		return TraceBlockTransactionsResponse{}, responseHeader,
			rpccore.ErrUnexpectedError.CloneWithData("VM omitted initial reads for trace extension")
	}
	adaptedReads := adaptVMInitialReads(vmInitialReads)
	return TraceBlockTransactionsResponse{
		Traces: traces, InitialReads: &adaptedReads,
	}, responseHeader, nil
}

func checkpointClasses(
	diff *core.StateDiff,
	classLookup core.StateReader,
) (map[felt.Felt]core.ClassDefinition, *jsonrpc.Error) {
	classes := make(
		map[felt.Felt]core.ClassDefinition,
		len(diff.DeclaredV0Classes)+len(diff.DeclaredV1Classes),
	)
	load := func(hash felt.Felt) *jsonrpc.Error {
		if _, exists := classes[hash]; exists {
			return nil
		}
		declared, err := classLookup.Class(&hash)
		if err != nil {
			return jsonrpc.Err(jsonrpc.InternalError, err.Error())
		}
		classes[hash] = declared.Class
		return nil
	}
	for _, hash := range diff.DeclaredV0Classes {
		if rpcErr := load(*hash); rpcErr != nil {
			return nil, rpcErr
		}
	}
	for hash := range diff.DeclaredV1Classes {
		if rpcErr := load(hash); rpcErr != nil {
			return nil, rpcErr
		}
	}
	return classes, nil
}

// checkpointFromTraces rebuilds the continuation checkpoint from cached per-transaction state
// diffs. Progressive records only contain traces with non-nil state diffs; executeTraceExtension
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
