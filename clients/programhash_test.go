package clients_test

import (
	_ "embed"
	"testing"

	"github.com/NethermindEth/juno/testsource"
	"github.com/NethermindEth/juno/utils"

	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core/felt"
)

func TestProgramHash(t *testing.T) {
	client, closer := testsource.NewTestClient(utils.GOERLI)
	defer closer.Close()
	hexToFelt := func(hex string) *felt.Felt {
		f, _ := new(felt.Felt).SetString(hex)
		return f
	}
	tests := []struct {
		class string
		name  string
		want  *felt.Felt
	}{
		{ // https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8
			class: "0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
			name:  "Genesis contract",
			want:  hexToFelt("0x1e87d79be8c8146494b5c54318f7d194481c3959752659a1e1bce158649a670"),
		},
		{ // https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3
			class: "0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3",
			name:  "Cairo 0.8 contract",
			want:  hexToFelt("0x359145fc6207854bfbbeadae4c6e289024400a5af87090ed18073200fef6213"),
		},
		{ // https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118
			class: "0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118",
			name:  "Cairo 0.10 contract",
			want:  hexToFelt("0x88562ac88adfc7760ff452d048d39d72978bcc0f8d7b0fcfb34f33970b3df3"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classDefinition, err := client.GetClassDefinition(hexToFelt(tt.class))
			if err != nil {
				t.Fatal(err)
			}

			programHash, err := clients.ProgramHash(classDefinition)
			if err != nil {
				t.Fatalf("unexpected error while computing program hash: %s", err)
			}

			if !programHash.Equal(tt.want) {
				t.Errorf("wrong hash: got %s, want %s", programHash.Text(16), tt.want.Text(16))
			}
		})
	}
}
