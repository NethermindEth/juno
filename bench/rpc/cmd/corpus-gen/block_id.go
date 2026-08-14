package main

func blockIDSampler(input samplerInput[blockRangeFlags]) (any, error) {
	return blockIDParams{blockNumberID{input.args.sampleBlockNumber(input.rng)}}, nil
}
