package starknet

import (
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/pkg/felt"
	gocmp "github.com/google/go-cmp/cmp"

	"gotest.tools/assert"
)

func TestBlockId_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want BlockId
		err  error
	}{
		{
			name: "empty data",
			data: []byte{},
			want: BlockId{},
			err:  io.EOF,
		},
		{
			name: "empty string",
			data: []byte(`""`),
			want: BlockId{},
			err:  ErrInvalidBlockId,
		},
		{
			name: "negative number",
			data: []byte(`-1`),
			want: BlockId{},
			err:  ErrInvalidBlockId,
		},
		{
			name: "float number",
			data: []byte(`1.1`),
			want: BlockId{},
			err:  ErrInvalidBlockId,
		},
		{
			name: "zero string",
			data: []byte(`"0"`),
			want: BlockId{},
			err:  ErrInvalidBlockId,
		},
		{
			name: "zero number",
			data: []byte(`0`),
			want: BlockId{
				value:  uint64(0),
				idType: blockIdNumber,
			},
			err: nil,
		},
		{
			name: "positive number",
			data: []byte(`1`),
			want: BlockId{
				value:  uint64(1),
				idType: blockIdNumber,
			},
			err: nil,
		},
		{
			name: "pending tag",
			data: []byte(`"pending"`),
			want: BlockId{
				idType: blockIdTag,
				value:  "pending",
			},
			err: nil,
		},
		{
			name: "latest tag",
			data: []byte(`"latest"`),
			want: BlockId{
				idType: blockIdTag,
				value:  "latest",
			},
			err: nil,
		},
		{
			name: "block hash",
			data: []byte(`"0x07d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b"`),
			want: BlockId{
				idType: blockIdHash,
				value:  new(felt.Felt).SetHex("0x07d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b"),
			},
			err: nil,
		},
		{
			name: "invalid block hash",
			data: []byte(`"0x7d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b"`),
			want: BlockId{},
			err:  ErrInvalidBlockId,
		},
		{
			name: "unexpected JSON type",
			data: []byte(`{}`),
			want: BlockId{},
			err:  ErrInvalidBlockId,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got BlockId
			if err := got.UnmarshalJSON(tt.data); !errors.Is(err, tt.err) {
				t.Errorf("BlockId.UnmarshalJSON() = %v, want %v", err, tt.err)
			}
			assert.DeepEqual(t, got, tt.want, gocmp.Comparer(func(x, y BlockId) bool {
				xv := reflect.ValueOf(x)
				yv := reflect.ValueOf(y)
				if xv.IsZero() || yv.IsZero() {
					return xv.IsZero() && yv.IsZero()
				}
				if x.idType != y.idType {
					return false
				}
				switch x.idType {
				case blockIdHash:
					return x.value.(*felt.Felt).Equal(y.value.(*felt.Felt))
				case blockIdTag:
					return x.value.(string) == y.value.(string)
				case blockIdNumber:
					return x.value.(uint64) == y.value.(uint64)
				default:
					return false
				}
			}))
		})
	}
}

func TestStorageKey_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want StorageKey
		err  bool
	}{
		{
			name: "empty data",
			data: []byte{},
			want: "",
			err:  true,
		},
		{
			// NOTE: This storage key is invalid because it is greater than prime P on the Stark
			// curve and also does not match the storage key regular expression provided by the RPC
			// specification.
			name: "invalid storage key",
			data: []byte(`"0x0ad328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b"`),
			want: "",
			err:  true,
		},
		{
			name: "valid storage key",
			data: []byte(`"0x01d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b"`),
			want: "0x01d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b",
			err:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got StorageKey
			if err := got.UnmarshalJSON(tt.data); (err != nil) != tt.err {
				t.Errorf("StorageKey.UnmarshalJSON(), unexpected error: %v", err)
			}
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestStorageKeyToFelt(t *testing.T) {
	tests := []struct {
		storageKey StorageKey
		want       *felt.Felt
	}{
		{
			storageKey: "0x07352d792298a2578d6ef20e80bce32473fff67b9b12b1bc431982287190291a",
			want:       new(felt.Felt).SetHex("0x07352d792298a2578d6ef20e80bce32473fff67b9b12b1bc431982287190291a"),
		},
		{
			storageKey: "0x00898cca7dbf84c2213f3a00e84775013bc991bc104d3a00952ce4bc166a4a1a",
			want:       new(felt.Felt).SetHex("0x00898cca7dbf84c2213f3a00e84775013bc991bc104d3a00952ce4bc166a4a1a"),
		},
		{
			storageKey: "0x0000000000000000000000000000000000000000000000000000000000000005",
			want:       new(felt.Felt).SetHex("0x0000000000000000000000000000000000000000000000000000000000000005"),
		},
	}
	for _, tt := range tests {
		assert.Check(t, tt.storageKey.Felt().Equal(tt.want))
	}
}

func TestRpcFelt_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want RpcFelt
		err  bool
	}{
		{
			name: "empty data",
			data: []byte{},
			want: "",
			err:  true,
		},
		{
			name: "invalid felt",
			data: []byte(`"0xad328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b"`),
			want: "",
			err:  true,
		},
		{
			name: "valid felt",
			data: []byte(`"0x02c2bb91714f8448ed814bdac274ab6fcdbafc22d835f9e847e5bee8c2e5444e"`),
			want: "0x02c2bb91714f8448ed814bdac274ab6fcdbafc22d835f9e847e5bee8c2e5444e",
			err:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got RpcFelt
			if err := got.UnmarshalJSON(tt.data); (err != nil) != tt.err {
				t.Errorf("RpcFelt.UnmarshalJSON(), unexpected error: %v", err)
			}
			assert.DeepEqual(t, got, tt.want)
		})
	}
}

func TestRpcFeltToFelt(t *testing.T) {
	tests := []struct {
		rpcFelt RpcFelt
		want    *felt.Felt
	}{
		{
			rpcFelt: "0x045c61314be4da85f0e13df53d18062e002c04803218f08061e4b274d4b38537",
			want:    new(felt.Felt).SetHex("0x045c61314be4da85f0e13df53d18062e002c04803218f08061e4b274d4b38537"),
		},
		{
			rpcFelt: "0x0320e37cf7c972458a3edf08ab51f2ab7596857706af174a3d5be4e46f16c63e",
			want:    new(felt.Felt).SetHex("0x0320e37cf7c972458a3edf08ab51f2ab7596857706af174a3d5be4e46f16c63e"),
		},
		{
			rpcFelt: "0x048636a22c6c74f5631b00c66ac3c8ab4714aa308ddd214af4089ccdfcee0f81",
			want:    new(felt.Felt).SetHex("0x048636a22c6c74f5631b00c66ac3c8ab4714aa308ddd214af4089ccdfcee0f81"),
		},
	}
	for _, tt := range tests {
		assert.Check(t, tt.rpcFelt.Felt().Equal(tt.want))
	}
}
