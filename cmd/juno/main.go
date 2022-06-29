package main

import (
	"errors"
	feeder2 "github.com/NethermindEth/juno/pkg/feeder"
)

func main() {
	//cli.Execute()
	feeder := feeder2.NewClient("https://alpha-mainnet.starknet.io/", "feeder_gateway", nil)
	val, err := feeder.GetBlock("", "2903")
	if err != nil {
		if errors.Is(err, feeder2.ErrorBlockNotFound) {
			print("Block not found")
			return
		}
		print(err)
	}
	print(val)

}
