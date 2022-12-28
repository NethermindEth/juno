package serialize

import (
	"reflect"
	"testing"
)

func TestCbor(t *testing.T) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}, map[uint64]string{
		0: "zero",
		1: "one",
		2: "two",
	}, []string{"hello", "world"}}
	encoded, _ := MarshalCbor(test)
	unmarshalTest, _ := UnMarshalCbor[TestMarshal](encoded)

	if !reflect.DeepEqual(test, unmarshalTest) {
		t.Fatalf("Gob marshalling incorrect")
	}
}

func BenchmarkCbor(b *testing.B) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}, map[uint64]string{
		0: "zero",
		1: "one",
		2: "two",
	}, []string{"hello", "world"}}
	encoded, _ := MarshalCbor(test)

	b.Run("Marshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			MarshalCbor(test)
		}
	})
	b.Run("UnMarshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			UnMarshalCbor[TestMarshal](encoded)
		}
	})
}
