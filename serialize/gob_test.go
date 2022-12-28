package serialize

import (
	"reflect"
	"testing"
)

func TestGob(t *testing.T) {

	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}}
	encoded := MarshalGob(test)
	unmarshalTest := UnMarshalGob[TestMarshal](encoded)

	if !reflect.DeepEqual(test, unmarshalTest) {
		t.Fatalf("Gob marshalling incorrect")
	}
}

func BenchmarkGob(b *testing.B) {

	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}}
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

// go test -benchmem -run=^$ -bench ^Benchmark github.com/NethermindEth/juno/serialize
