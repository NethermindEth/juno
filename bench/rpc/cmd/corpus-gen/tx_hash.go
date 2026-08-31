package main

import "github.com/spf13/cobra"

type txHashWithProofFactsArgs struct {
	blockRangeFlags
	ResponseFlags txnFlags `json:"responseFlags"`
}

func (a *txHashWithProofFactsArgs) bind(cmd *cobra.Command, client *rpcClient) {
	a.ResponseFlags.bind(cmd, client)
	a.blockRangeFlags.bind(cmd, client)
}

func txHashSampler(input samplerInput[blockRangeFlags]) (txHashParams, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	block, err := input.client.blockWithTxHashes(input.ctx, blockNumber)
	if err != nil {
		return txHashParams{}, err
	}
	hash, err := pickRandom(input.rng, block.Transactions)
	if err != nil {
		return txHashParams{}, err
	}
	return txHashParams{TransactionHash: hash}, nil
}

func txHashWithProofFactsSampler(
	input samplerInput[txHashWithProofFactsArgs],
) (txHashParams, error) {
	params, err := txHashSampler(input.rebindArgs(&input.args.blockRangeFlags))
	if err != nil {
		return txHashParams{}, err
	}
	params.ResponseFlags = input.args.ResponseFlags
	return params, nil
}
