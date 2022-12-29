package serialize

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"testing"

	pb "github.com/NethermindEth/juno/serialize/protobuf"
)

func TestCompression(t *testing.T) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}, map[uint64]string{
		0: "zero",
		1: "one",
		2: "two",
	}, []string{"hello", "world"}}

	cborEncoded, _ := MarshalCbor(test)
	cborSize := binary.Size(cborEncoded)
	t.Logf("Cbor size: %d", cborSize)

	jsonEncoded, _ := MarshalJson(test)
	jsonSize := binary.Size(jsonEncoded)
	t.Logf("Json size: %d", jsonSize)

	var gobBuffer bytes.Buffer
	encoder := gob.NewEncoder(&gobBuffer)
	MarshalGob2(encoder, test)
	// gobSize := gobBuffer.Len()
	gobSize := binary.Size(gobBuffer.Bytes())
	t.Logf("Gob size: %d", gobSize)

	testPb := &pb.TestMarshal{}
	testInnerPb := &pb.TestInner{}
	testInnerPb.InnerA = "Test Inner"

	testPb.A = 5
	testPb.B = "Test"
	testPb.C = true
	testPb.D = testInnerPb
	testPb.E = map[uint64]string{
		0: "zero",
		1: "one",
		2: "two",
	}
	testPb.F = []string{"hello", "world"}

	protobufEncoded, _ := testPb.Marshal()
	protobufSize := binary.Size(protobufEncoded)
	t.Logf("Protobuf size: %d", protobufSize)
}
