package main

import "github.com/spf13/cobra"

type simulateTxsArgs struct {
	broadcastedTxsArgs
	SimulateFlags simulateFlags `json:"simulateFlags"`
}

func (a *simulateTxsArgs) bind(cmd *cobra.Command, client *rpcClient) {
	a.SimulateFlags.bind(cmd, client)
	a.broadcastedTxsArgs.bind(cmd, client)
}

func simulateTxsSampler(input samplerInput[simulateTxsArgs]) (simulateTxsParams, error) {
	txs, id, err := sampleBroadcastedTxs(input.rebindArgs(&input.args.broadcastedTxsArgs))
	if err != nil {
		return simulateTxsParams{}, err
	}

	params := simulateTxsParams{
		BlockID:         id,
		Transactions:    txs,
		SimulationFlags: input.args.SimulateFlags,
	}
	if input.args.Verify {
		if err := verify(input.ctx, input.client, input.method, params); err != nil {
			return simulateTxsParams{}, err
		}
	}
	return params, nil
}
