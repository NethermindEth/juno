package main

func txByBlockIDAndIndexSampler(
	input samplerInput[blockRangeFlags],
) (txByBlockIDAndIndexParams, error) {
	blockNumber := input.args.sampleBlockNumber(input.rng)
	count, err := input.client.txCountInBlock(input.ctx, blockNumber)
	if err != nil {
		return txByBlockIDAndIndexParams{}, err
	}
	if count == 0 {
		return txByBlockIDAndIndexParams{}, errResample
	}
	return txByBlockIDAndIndexParams{
		BlockID: blockNumberID{blockNumber},
		Index:   input.rng.Uint64N(count),
	}, nil
}
