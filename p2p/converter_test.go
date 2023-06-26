package p2p

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
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

//nolint:all
func dumpBlock(blockNum uint64) {
	db, _ := pebble.New("/home/amirul/fastworkscratch/largejuno_bak", utils.NewNopZapLogger())
	bc := blockchain.New(db, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

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
	//nolint:all
	// db, _ := pebble.New("/home/amirul/fastworkscratch/largejuno_bak", utils.NewNopZapLogger())
	db, _ := pebble.NewMem()
	_ = blockchain.New(db, utils.MAINNET, utils.NewNopZapLogger()) // Needed because class loader need encoder to be registered

	classProvider := &mapClassProvider{
		classes: map[felt.Felt]*core.DeclaredClass{},
	}

	//nolint:all
	// classProvider.Intercept(&blockchainClassProvider{blockchain: bc})
	classProvider.Load()

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
			runEncodeDecodeBlockTest(t, &c, &v, &block)
		})
	}

	//nolint:all
	// classProvider.Save()
}

func runEncodeDecodeBlockTest(t *testing.T, c *converter, v *verifier, originalBlock *core.Block) {
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
	convertedBlock, includedClasses, err := c.protobufHeaderAndBodyToCoreBlock(header, body)
	if err != nil {
		t.Fatalf("back to core failed %v", err)
	}

	err = v.VerifyBlock(convertedBlock)
	if err != nil {
		t.Fatalf("block verification failed failed %v", err)
	}

	for k, class := range includedClasses {
		err := v.VerifyClass(class, &k)
		if err != nil {
			t.Fatalf("class verification failed failed %v", err)
		}

		declaredClass, err := c.classprovider.GetClass(&k)
		if err != nil {
			t.Fatalf("error loading class %s", err)
		}

		currentClass := declaredClass.Class
		if v, ok := currentClass.(*core.Cairo1Class); ok {
			v.Compiled = nil
		}

		assert.Equal(t, currentClass, class)
	}

	originalBlock.ProtocolVersion = ""
	originalBlock.ExtraData = nil
	convertedBlock.ProtocolVersion = ""
	convertedBlock.ExtraData = nil

	// Check if the final struct is equal to the original struct
	assert.Equal(t, originalBlock, convertedBlock)
}

func TestEncodeDecodeStateUpdate(t *testing.T) {
	db, _ := pebble.NewMem()
	blockchain.New(db, utils.MAINNET, utils.NewNopZapLogger()) // So that the encoder get registered
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
	protoBuff := coreStateUpdateToProtobufStateUpdate(stateUpdate)
	reencodedStateUpdate := protobufStateUpdateToCoreStateUpdate(protoBuff)

	type storageKV struct {
		key   felt.Felt
		value []core.StorageDiff
	}

	odarray := make([]storageKV, 0)
	for k, diffs := range stateUpdate.StateDiff.StorageDiffs {
		odarray = append(odarray, storageKV{
			key:   k,
			value: diffs,
		})
	}

	rdarray := make([]storageKV, 0)
	for k, diffs := range reencodedStateUpdate.StateDiff.StorageDiffs {
		rdarray = append(rdarray, storageKV{
			key:   k,
			value: diffs,
		})
	}

	slices.SortFunc(odarray, func(a, b storageKV) bool {
		return a.key.Cmp(&b.key) > 0
	})
	slices.SortFunc(rdarray, func(a, b storageKV) bool {
		return a.key.Cmp(&b.key) > 0
	})

	// Check if the final struct is equal to the original struct
	assert.Equal(t, odarray, rdarray)
	assert.True(t, reflect.DeepEqual(stateUpdate.StateDiff.Nonces, reencodedStateUpdate.StateDiff.Nonces))
	assert.Equal(t, stateUpdate.StateDiff.DeclaredV0Classes, reencodedStateUpdate.StateDiff.DeclaredV0Classes)
	assert.Equal(t, stateUpdate.StateDiff.DeclaredV1Classes, reencodedStateUpdate.StateDiff.DeclaredV1Classes)
	assert.Equal(t, stateUpdate.StateDiff.ReplacedClasses, reencodedStateUpdate.StateDiff.ReplacedClasses)
	assert.Equal(t, stateUpdate.StateDiff.DeployedContracts, reencodedStateUpdate.StateDiff.DeployedContracts)
}
