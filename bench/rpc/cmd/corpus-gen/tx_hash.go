package main

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
