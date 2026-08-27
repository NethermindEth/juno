package main

func blockIDSampler(input samplerInput[blockRangeFlags]) (blockIDParams, error) {
	return blockIDParams{blockNumberID{input.args.sampleBlockNumber(input.rng)}}, nil
}
