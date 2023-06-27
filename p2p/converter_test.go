package p2p

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
)

type mapClassProvider struct {
	classes             map[felt.Felt]*core.DeclaredClass
	providerToIntercept ClassProvider
}

func (c *mapClassProvider) GetClass(hash *felt.Felt) (*core.DeclaredClass, error) {
	if c.providerToIntercept != nil {
		cls, err := c.providerToIntercept.GetClass(hash)
		if err == nil {
			c.classes[*hash] = cls
		}

		return cls, err
	}

	class, ok := c.classes[*hash]
	if !ok {
		return nil, fmt.Errorf("unable to get class of hash %s", hash.String())
	}
	return class, nil
}

func (c *mapClassProvider) Intercept(provider ClassProvider) {
	c.providerToIntercept = provider
}

func (c *mapClassProvider) Load() {
	bytes, err := os.ReadFile("converter_tests/classes.dat")
	if err != nil {
		panic(err)
	}

	err = encoder.Unmarshal(bytes, &c.classes)
	if err != nil {
		panic(err)
	}
}

func (c *mapClassProvider) Save() {
	bytes, err := encoder.Marshal(c.classes)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("converter_tests/classes.dat", bytes, 0o666)
	if err != nil {
		panic(err)
	}
}

// Used to dump blocks
//
//nolint:all
func dumpBlock(blockNum uint64, d db.DB) {
	bc := blockchain.New(d, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	block, err := bc.BlockByNumber(blockNum)
	if err != nil {
		panic(err)
	}

	asbyte, err := encoder.Marshal(block)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(fmt.Sprintf("converter_tests/blocks/%d.dat", blockNum), asbyte, 0o666)
	if err != nil {
		panic(err)
	}
}

func TestEncodeDecodeBlocks(t *testing.T) {
	interceptClassDB, _ := os.LookupEnv("P2P_TEST_SOURCE_DB")
	var d db.DB
	if interceptClassDB != "" {
		d, _ = pebble.New(interceptClassDB, utils.NewNopZapLogger())
	} else {
		d, _ = pebble.NewMem()
	}
	bc := blockchain.New(d, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	classProvider := &mapClassProvider{
		classes: map[felt.Felt]*core.DeclaredClass{},
	}

	if interceptClassDB != "" {
		classProvider.Intercept(&blockchainClassProvider{blockchain: bc})
	} else {
		classProvider.Load()
	}

	c := converter{
		classprovider: classProvider,
	}

	v := verifier{
		network: utils.MAINNET,
	}

	globed, err := filepath.Glob("converter_tests/blocks/*dat")
	if err != nil {
		panic(err)
	}

	for _, filename := range globed {
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

		t.Run(filename, func(t *testing.T) {
			runEncodeDecodeBlockTest(t, &c, &v, bc.Network(), &block)
		})
	}

	if interceptClassDB != "" {
		classProvider.Save()
	}
}

func runEncodeDecodeBlockTest(t *testing.T, c *converter, v Verifier, network utils.Network, originalBlock *core.Block) {
	err := testBlockEncoding(originalBlock, c, v, network, false)
	if err != nil {
		t.Fatalf("error on block encoding test %s", err)
	}
}

func TestEncodeDecodeStateUpdate(t *testing.T) {
	d, _ := pebble.NewMem()
	blockchain.New(d, utils.MAINNET, utils.NewNopZapLogger()) // So that the encoder get registered
	globed, err := filepath.Glob("converter_tests/state_updates/*dat")
	if err != nil {
		panic(err)
	}

	for i, filename := range globed {
		f, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		decoder := encoder.NewDecoder(f)
		stateUpdate := core.StateUpdate{}
		err = decoder.Decode(&stateUpdate)
		if err != nil {
			panic(err)
		}

		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			runEncodeDecodeStateUpdateTest(t, &stateUpdate)
		})
	}
}

func runEncodeDecodeStateUpdateTest(t *testing.T, stateUpdate *core.StateUpdate) {
	// Convert original struct to protobuf struct
	err := testStateDiff(stateUpdate)
	if err != nil {
		t.Fatalf("error on state encoding test %s", err)
	}
}
