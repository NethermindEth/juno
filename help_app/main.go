package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
)

func main() {
	database, err := pebble.NewWithOptions("../snapshots/juno_sepolia", 8, 1, false)
	if err != nil {
		panic(err)
	}

	file, err := os.Create("output.csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var buffer bytes.Buffer
	_, err = buffer.WriteTo(file)
	if err != nil {
		panic(err)
	}

	chain := blockchain.New(database, &utils.Sepolia)
	for blockNumber := range uint64(86312) {
		block, err := chain.BlockByNumber(blockNumber)
		if err != nil {
			panic(err)
		}
		stateUpdate, err := chain.StateUpdateByNumber(0)
		if err != nil {
			panic(err)
		}
		hash, _, err := core.Post0132Hash(block, stateUpdate.StateDiff)
		if err != nil {
			panic(err)
		}
		_, err = buffer.WriteString(strconv.FormatUint(blockNumber, 10))
		if err != nil {
			panic(err)
		}
		err = buffer.WriteByte(',')
		if err != nil {
			panic(err)
		}
		_, err = buffer.WriteString(hash.String())
		if err != nil {
			panic(err)
		}
		err = buffer.WriteByte('\n')
		if err != nil {
			panic(err)
		}
		_, err = buffer.WriteTo(file)
		if err != nil {
			panic(err)
		}
		fmt.Println("Block number:", blockNumber)
	}
}
