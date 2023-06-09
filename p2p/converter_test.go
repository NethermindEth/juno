package p2p

import (
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"testing"
)

var blocks = []*core.Block{
	{
		Header: unsafeDecodeJson[*core.Header]("{\"Hash\":\"0x27d1e625a4293d2ed162eb2a1ad031cf71eaa2bab3a3d10fe9bddc51b877ccd\",\"ParentHash\":\"0x16b00cd0ed871bccdfed41fac05ef64c025f6df61a8369d62db777bc0872f62\",\"Number\":192,\"GlobalStateRoot\":\"0x2e3dcf2a28f3c4d53fdc5970cb43ddb27f1ecab5ba6da9074c266a195b4598c\",\"SequencerAddress\":null,\"TransactionCount\":1,\"EventCount\":0,\"Timestamp\":1638440748,\"ProtocolVersion\":\"\",\"ExtraData\":null,\"EventsBloom\":{\"m\":8192,\"k\":6,\"b\":\"AAAAAAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\"}}"),
		Transactions: []core.Transaction{
			unsafeDecodeJson[*core.L1HandlerTransaction]("{\"TransactionHash\":\"0x5d50b7020f7cf8033fd7d913e489f47edf74fbf3c8ada85be512c7baa6a2eab\",\"ContractAddress\":\"0x58b43819bb12aba8ab3fb2e997523e507399a3f48a1e2aa20a5fb7734a0449f\",\"EntryPointSelector\":\"0xe3f5e9e1456ffa52a3fbc7e8c296631d4cc2120c0be1e2829301c0d8fa026b\",\"Nonce\":null,\"CallData\":[\"0x5474c49483aa09993090979ade8101ebb4cdce4a\",\"0xabf8dd8438d1c21e83a8b5e9c1f9b58aaf3ed360\",\"0x2\",\"0x4c04fac82913f01a8f01f6e15ff7e834ff2d9a9a1d8e9adffc7bd45692f4f9a\"],\"Version\":\"0x0\"}"),
		},
		Receipts: []*core.TransactionReceipt{
			unsafeDecodeJson[*core.TransactionReceipt]("{\n    \"Fee\": \"0x0\",\n    \"Events\": [],\n    \"ExecutionResources\": {\n        \"BuiltinInstanceCounter\": {\n            \"Bitwise\": 0,\n            \"EcOp\": 0,\n            \"Ecsda\": 0,\n            \"Output\": 0,\n            \"Pedersen\": 2,\n            \"RangeCheck\": 6\n        },\n        \"MemoryHoles\": 22,\n        \"Steps\": 193\n    },\n    \"L1ToL2Message\": {\n        \"From\": \"0x5474c49483aa09993090979ade8101ebb4cdce4a\",\n        \"Nonce\": null,\n        \"Payload\": [\n            \"0xabf8dd8438d1c21e83a8b5e9c1f9b58aaf3ed360\",\n            \"0x2\",\n            \"0x4c04fac82913f01a8f01f6e15ff7e834ff2d9a9a1d8e9adffc7bd45692f4f9a\"\n        ],\n        \"Selector\": \"0xe3f5e9e1456ffa52a3fbc7e8c296631d4cc2120c0be1e2829301c0d8fa026b\",\n        \"To\": \"0x58b43819bb12aba8ab3fb2e997523e507399a3f48a1e2aa20a5fb7734a0449f\"\n    },\n    \"L2ToL1Message\": [],\n    \"TransactionHash\": \"0x5d50b7020f7cf8033fd7d913e489f47edf74fbf3c8ada85be512c7baa6a2eab\"\n}"),
		},
	},
}

func unsafeDecodeJson[T any](jsonstr string) T {
	var obj T

	err := json.Unmarshal([]byte(jsonstr), &obj)
	if err != nil {
		panic(err)
	}

	return obj
}

func TestEncodeDecode(t *testing.T) {
	for i, block := range blocks {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			runEncodeDecodeTest(t, block)
		})
	}
}

func runEncodeDecodeTest(t *testing.T, originalBlock *core.Block) {
	// Convert original struct to protobuf struct
	header, err := coreBlockToProtobufHeader(originalBlock)
	if err != nil {
		t.Fatalf("to protobuf failed %v", err)
	}

	body := coreBlockToProtobufBody(originalBlock)

	// Convert protobuf struct back to original struct
	convertedBlock, err := protobufHeaderToCoreBlock(header, body, utils.MAINNET)
	if err != nil {
		t.Fatalf("back to core failed %v", err)
	}

	// Check if the final struct is equal to the original struct
	assert.Equal(t, originalBlock, convertedBlock)
}
