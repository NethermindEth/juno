package feeder

import "testing"

func TestIsPending(t *testing.T) {
	tests := [...]struct {
		blockNumber BlockNumber
		want        bool
	}{
		{
			blockNumber: BlockNumber(-1),
			want:        true,
		},
		{
			blockNumber: BlockNumber(0),
			want:        false,
		},
		{
			blockNumber: BlockNumber(10),
			want:        false,
		},
	}
	for _, test := range tests {
		result := test.blockNumber.IsPending()
		if result != test.want {
			t.Errorf("%d.IsPending()=%t, want %t", test.blockNumber, result, test.want)
		}
	}
}

func TestBlockNumberUnmarshalJSON(t *testing.T) {
	tests := [...]struct {
		data []byte
		want BlockNumber
		err  bool
	}{
		{
			data: []byte("\"pending\""),
			want: BlockNumber(-1),
			err:  false,
		},
		{
			data: []byte("-1"),
			want: *new(BlockNumber),
			err:  true,
		},
		{
			data: []byte("123"),
			want: BlockNumber(123),
			err:  false,
		},
		{
			data: []byte("123\""),
			want: *new(BlockNumber),
			err:  true,
		},
		{
			data: []byte("\"123"),
			want: *new(BlockNumber),
			err:  true,
		},
		{
			data: []byte(""),
			want: *new(BlockNumber),
			err:  true,
		},
		{
			data: []byte("\"invalid string\""),
			want: *new(BlockNumber),
			err:  true,
		},
	}
	for _, test := range tests {
		result := *new(BlockNumber)
		err := result.UnmarshalJSON(test.data)
		if err != nil && !test.err {
			t.Errorf("unexpected error: %s", err.Error())
		}
		if err == nil && test.err {
			t.Errorf("test must fail with error")
		}
		if result != test.want {
			t.Errorf("UnmarshalJSON(%v)=%d, want %d", test.data, result, test.want)
		}
	}
}
