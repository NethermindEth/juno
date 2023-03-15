package feeder_test

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/require"
)

func TestProgramHash(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.GOERLI)
	defer closeFn()
	hexToFelt := func(t *testing.T, hex string) *felt.Felt {
		f, err := new(felt.Felt).SetString(hex)
		require.NoError(t, err)
		return f
	}
	tests := []struct {
		classHash string
		name      string
		want      *felt.Felt
	}{
		{ // https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8
			classHash: "0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
			name:      "Genesis contract",
			want: hexToFelt(
				t, "0x1e87d79be8c8146494b5c54318f7d194481c3959752659a1e1bce158649a670"),
		},
		{ // https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3
			classHash: "0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3",
			name:      "Cairo 0.8 contract",
			want: hexToFelt(
				t, "0x359145fc6207854bfbbeadae4c6e289024400a5af87090ed18073200fef6213"),
		},
		{ // https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118
			classHash: "0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118",
			name:      "Cairo 0.10 contract",
			want: hexToFelt(
				t, "0x88562ac88adfc7760ff452d048d39d72978bcc0f8d7b0fcfb34f33970b3df3"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classDefinition, err := client.ClassDefinition(context.Background(),
				hexToFelt(t, tt.classHash))
			if err != nil {
				t.Fatal(err)
			}

			programHash, err := feeder.ProgramHash(classDefinition)
			if err != nil {
				t.Fatalf("unexpected error while computing program hash: %s", err)
			}

			if !programHash.Equal(tt.want) {
				t.Errorf("wrong hash: got %s, want %s", programHash.String(), tt.want.String())
			}
		})
	}
}
