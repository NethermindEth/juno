package serialize

import (
	"reflect"
	"testing"
)

func TestJson(t *testing.T) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}, map[uint64]string{
		0: "zero",
		1: "one",
		2: "two",
	}, []string{"hello", "world"}}
	encoded, _ := MarshalJson(test)
	decodedTest, _ := UnMarshalJson[TestMarshal](encoded)

	if !reflect.DeepEqual(test, decodedTest) {
		t.Fatalf("JSON marshalling incorrect")
	}
}

func BenchmarkJson(b *testing.B) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}, map[uint64]string{
		0: "zero",
		1: "one",
		2: "two",
	}, []string{"hello", "world"}}
	encoded, _ := MarshalJson(test)

	b.Run("Marshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			MarshalJson(test)
		}
	})

	b.Run("UnMarshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			UnMarshalJson[TestMarshal](encoded)
		}
	})
}
