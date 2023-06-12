package p2p

import (
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	db, _ := pebble.NewMem()
	bc := blockchain.New(db, utils.MAINNET, utils.NewNopZapLogger())
	c := converter{
		blockchain: bc,
	}

	globed, err := filepath.Glob("converter_tests/*dat")
	if err != nil {
		panic(err)
	}

	for i, filename := range globed {
		f, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		decoder := encoder.NewDecoder(f)
		block := core.Block{}
		err = decoder.Decode(&block)
		if err != nil {
			panic(err)
		}

		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			runEncodeDecodeTest(t, c, &block)
		})
	}
}

func runEncodeDecodeTest(t *testing.T, c converter, originalBlock *core.Block) {
	// Convert original struct to protobuf struct
	header, err := c.coreBlockToProtobufHeader(originalBlock)
	if err != nil {
		t.Fatalf("to protobuf failed %v", err)
	}

	body, err := c.coreBlockToProtobufBody(originalBlock)
	if err != nil {
		t.Fatalf("to protobuf failed %v", err)
	}

	// Convert protobuf struct back to original struct
	convertedBlock, err := protobufHeaderAndBodyToCoreBlock(header, body, utils.MAINNET)
	if err != nil {
		t.Fatalf("back to core failed %v", err)
	}

	// Check if the final struct is equal to the original struct
	assert.Equal(t, originalBlock, convertedBlock)
}
