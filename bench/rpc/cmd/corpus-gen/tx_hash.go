package main

func txHashSampler(input samplerInput[blockRangeFlags]) (any, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	block, err := input.client.blockWithTxHashes(input.ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	hash, err := pickRandom(input.rng, block.Transactions)
	if err != nil {
		return nil, err
	}
	return txHashParams{TransactionHash: hash}, nil
}
