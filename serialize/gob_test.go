package serialize

import (
	"bytes"
	"encoding/gob"
	"reflect"
	"testing"
)

func TestGob(t *testing.T) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}, map[uint64]string{
		0: "zero",
		1: "one",
		2: "two",
	}, []string{"hello", "world"}}
	encoded := MarshalGob(test)
	unmarshalTest := UnMarshalGob[TestMarshal](encoded)

	if !reflect.DeepEqual(test, unmarshalTest) {
		t.Fatalf("Gob marshalling incorrect")
	}
}

func BenchmarkGob(b *testing.B) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}, map[uint64]string{
		0: "zero",
		1: "one",
		2: "two",
	}, []string{"hello", "world"}}
	encoded := MarshalGob(test)

	b.Run("Marshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			MarshalGob(test)
		}
	})
	b.Run("Unmarshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			UnMarshalGob[TestMarshal](encoded)
		}
	})
}

func BenchmarkGob2(b *testing.B) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}, map[uint64]string{
		0: "zero",
		1: "one",
		2: "two",
	}, []string{"hello", "world"}}

	var gobBuffer bytes.Buffer
	encoder := gob.NewEncoder(&gobBuffer)
	decoder := gob.NewDecoder(&gobBuffer)

	b.Run("Marshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			MarshalGob2(encoder, test)
		}
	})
	b.Run("Unmarshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			UnMarshalGob2[TestMarshal](decoder)
		}
	})
}

// go test -benchmem -run=^$ -bench ^Benchmark github.com/NethermindEth/juno/serialize
