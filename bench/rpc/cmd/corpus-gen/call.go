package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type callArgs struct {
	blockIDArgs
	Verify verifyFlag `json:"verify"`
}

func (a *callArgs) bind(cmd *cobra.Command, client *rpcClient) {
	a.Verify.bind(cmd, client)
	bindExecutionRange(cmd, client, &a.blockIDArgs)
}

// callSampler turns one call of an account multicall in block N+1 into a
// request against block N, the state that call ran on.
func callSampler(input samplerInput[callArgs]) (callParams, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	next := blockNumber + 1
	block, err := input.client.blockWithTxs(input.ctx, next)
	if err != nil {
		return callParams{}, err
	}

	call, err := pickRandom(input.rng, invokeCalls(block.Transactions))
	if err != nil {
		return callParams{}, fmt.Errorf("no account multicall in block %d: %w", next, err)
	}
	id, err := resolveBlockID(input.ctx, input.client, input.args.BlockIDKind, blockNumber)
	if err != nil {
		return callParams{}, err
	}

	params := callParams{Request: call, BlockID: id}
	if input.args.Verify {
		if err := verify(input.ctx, input.client, input.method, params); err != nil {
			return callParams{}, err
		}
	}
	return params, nil
}

// callHeaderFields counts the felts that precede a call's arguments in an
// account multicall: target address, selector and argument count.
const callHeaderFields = 3

// multicallCalls splits an account invoke's calldata into the calls it carries,
// and returns nil for anything that is not the Cairo 1 multicall layout.
func multicallCalls(calldata []string) []functionCall {
	if len(calldata) == 0 {
		return nil
	}
	count, ok := feltToUint(calldata[0])
	if !ok || count > uint64((len(calldata)-1)/callHeaderFields) {
		return nil
	}

	calls := make([]functionCall, count)
	next := 1
	for i := range calls {
		if len(calldata)-next < callHeaderFields {
			return nil
		}
		args, ok := feltToUint(calldata[next+2])
		if !ok || args > uint64(len(calldata)) {
			return nil
		}
		start := next + callHeaderFields
		end := start + int(args)
		if end > len(calldata) {
			return nil
		}
		calls[i] = functionCall{
			ContractAddress:    calldata[next],
			EntryPointSelector: calldata[next+1],
			Calldata:           calldata[start:end],
		}
		next = end
	}
	if next != len(calldata) {
		return nil
	}
	return calls
}

// invokeCalls lists the calls carried by the invoke transactions of a block.
func invokeCalls(txs []broadcastedTx) []functionCall {
	var calls []functionCall
	for _, tx := range txs {
		if tx.text("type") != invokeTxType {
			continue
		}
		var calldata []string
		if err := json.Unmarshal(tx["calldata"], &calldata); err != nil {
			continue
		}
		calls = append(calls, multicallCalls(calldata)...)
	}
	return calls
}

func feltToUint(value string) (uint64, bool) {
	parsed, err := strconv.ParseUint(value, 0, 64)
	return parsed, err == nil
}
