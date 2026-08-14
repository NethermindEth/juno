package main

func txByBlockIDAndIndexSampler(input samplerInput[blockRangeFlags]) (any, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	count, err := input.client.txCountInBlock(input.ctx, blockNumber)
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, errResample
	}
	return txByBlockIDAndIndexParams{
		BlockID: blockNumberID{blockNumber},
		Index:   input.rng.Uint64N(count),
	}, nil
}
