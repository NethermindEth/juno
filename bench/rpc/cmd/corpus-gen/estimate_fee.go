package main

import "github.com/spf13/cobra"

type estimateFeeArgs struct {
	broadcastedTxsArgs
	EstimateFlags estimateFlags `json:"estimateFlags"`
}

func (a *estimateFeeArgs) bind(cmd *cobra.Command, client *rpcClient) {
	a.EstimateFlags.bind(cmd, client)
	a.broadcastedTxsArgs.bind(cmd, client)
}

func estimateFeeSampler(input samplerInput[estimateFeeArgs]) (estimateFeeParams, error) {
	txs, id, err := sampleBroadcastedTxs(input.rebindArgs(&input.args.broadcastedTxsArgs))
	if err != nil {
		return estimateFeeParams{}, err
	}

	params := estimateFeeParams{
		Request:         txs,
		SimulationFlags: input.args.EstimateFlags,
		BlockID:         id,
	}
	if input.args.Verify {
		if err := verify(input.ctx, input.client, input.method, params); err != nil {
			return estimateFeeParams{}, err
		}
	}
	return params, nil
}
