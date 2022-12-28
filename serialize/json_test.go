package serialize

import (
	"reflect"
	"testing"
)

func TestJson(t *testing.T) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}}
	encoded, _ := MarshalJson(test)
	decodedTest, _ := UnMarshalJson[TestMarshal](encoded)

	if !reflect.DeepEqual(test, decodedTest) {
		t.Fatalf("JSON marshalling incorrect")
	}
}

func BenchmarkJson(b *testing.B) {
	test := TestMarshal{5, "Test", true, TestInner{"Test Inner"}}
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
