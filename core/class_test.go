package core_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestClassHash(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.GOERLI)
	defer closeFn()

	gw := adaptfeeder.New(client)
	tests := []struct {
		classHash string
	}{
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8
			classHash: "0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
		},
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3
			classHash: "0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3",
		},
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118
			classHash: "0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118",
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			hash := hexToFelt(t, tt.classHash)
			class, err := gw.Class(context.Background(), hash)
			assert.NoError(t, err)
			got := class.Hash()
			if !hash.Equal(got) {
				t.Errorf("wrong hash: got %s, want %s", got.Text(16), hash.Text(16))
			}
		})
	}
}
