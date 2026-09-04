package main

import "github.com/spf13/cobra"

type traceBlockArgs struct {
	blockIDArgs
	TraceFlags traceFlags `json:"traceFlags"`
}

func (a *traceBlockArgs) bind(cmd *cobra.Command, client *rpcClient) {
	a.TraceFlags.bind(cmd, client)
	a.blockIDArgs.bind(cmd, client)
}

func traceBlockTransactionsSampler(
	input samplerInput[traceBlockArgs],
) (traceBlockParams, error) {
	params, err := blockIDSampler(input.rebindArgs(&input.args.blockIDArgs))
	if err != nil {
		return traceBlockParams{}, err
	}
	return traceBlockParams{BlockID: params.BlockID, TraceFlags: input.args.TraceFlags}, nil
}
