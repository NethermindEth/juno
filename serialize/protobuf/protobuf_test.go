package protobuf

import "testing"

func BenchmarkProtobuf(b *testing.B) {
	test := &TestMarshal{}
	testInner := &TestInner{}
	testInner.InnerA = "Test Inner"

	test.A = 5
	test.B = "Test"
	test.C = true
	test.D = testInner

	encoded, _ := test.Marshal()

	b.Run("Marshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			test.Marshal()
		}
	})
	b.Run("UnMarshalling", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			newTest := &TestMarshal{}
			newTest.Unmarshal(encoded)
		}
	})
}
