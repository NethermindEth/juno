package pedersen

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/pkg/felt"
)

// BenchmarkDigest runs a benchmark on the Digest function by hashing a
// *felt.Felt with a value of 0 N times.
func BenchmarkDigest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Digest(new(felt.Felt))
	}
}

func ExampleDigest() {
	a := new(felt.Felt).SetHex("3d937c035c878245caf64531a5756109c53068da139362728feb561405371cb")
	b := new(felt.Felt).SetHex("208a0a10250e382e1e4bbe2880906c2791bf6275695e02fbbc6aeff9cd8b31a")
	fmt.Printf("%s\n", Digest(a, b).Hex())

	// Output:
	// 30e480bed5fe53fa909cc0f8c4d99b8f9f2c016be4c41e13a4848797979c662
}

// TestDigest does a basic test of the Pedersen hash function where the
// test cases chosen are the canonical ones that appear in the Python
// implementation of the same function by Starkware.
func TestDigest(t *testing.T) {
	// See https://github.com/starkware-libs/starkex-resources/blob/44a15c7d1bdafda15766ea0fc2e0866e970e39c1/crypto/starkware/crypto/signature/signature_test_data.json#L85-L96.
	tests := [...]struct {
		input1, input2, want string
	}{
		{
			"3d937c035c878245caf64531a5756109c53068da139362728feb561405371cb",
			"208a0a10250e382e1e4bbe2880906c2791bf6275695e02fbbc6aeff9cd8b31a",
			"30e480bed5fe53fa909cc0f8c4d99b8f9f2c016be4c41e13a4848797979c662",
		},
		{
			"58f580910a6ca59b28927c08fe6c43e2e303ca384badc365795fc645d479d45",
			"78734f65a067be9bdb39de18434d71e79f7b6466a4b66bbd979ab9e7515fe0b",
			"68cc0b76cddd1dd4ed2301ada9b7c872b23875d5ff837b3a87993e0d9996b87",
		},
	}
	for _, test := range tests {
		a := new(felt.Felt).SetHex(test.input1)
		b := new(felt.Felt).SetHex(test.input2)
		want := new(felt.Felt).SetHex(test.want)
		got := Digest(a, b)
		if got.Cmp(want) != 0 {
			t.Errorf("Digest(%x, %x) = %x, want %x", a.Hex(), b.Hex(), got.Hex(), want.Hex())
		}
	}
}

func BenchmarkArrayDigest(b *testing.B) {
	n := 20
	data := make([]*felt.Felt, n)
	for i := range data {
		data[i] = new(felt.Felt)
		if _, err := data[i].SetRandom(); err != nil {
			b.Fatalf("error while generating random felt: %x", err)
		}
	}

	b.Run(fmt.Sprintf("Benchmark pedersen.ArrayDigest over %d felt.Felts", n), func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ArrayDigest(data...)
		}
	})
}

func TestArrayDigest(t *testing.T) {
	tests := [...]struct {
		input []string
		want  string
	}{
		// Contract address calculation. See the following links for how the
		// it is carried out and the result referenced.
		//
		//	- https://docs.starknet.io/docs/Contracts/contract-address/
		//	- https://alpha-goerli.starknet.io/feeder_gateway/get_transaction?transactionHash=0x1b50380d45ebd70876518203f131a12428b2ac1a3a75f1a74241a4abdd614e8
		{
			input: []string{
				// Hex representation of []byte("STARKNET_CONTRACT_ADDRESS").
				"535441524b4e45545f434f4e54524143545f41444452455353",
				// caller_address.
				"0",
				// salt.
				"5bebda1b28ba6daa824126577b9fbc984033e8b18360f5e1ef694cb172c7aa5",
				// contract_hash. See the following for reference https://alpha4.starknet.io/feeder_gateway/get_block?blockHash=0x53e61cb9a53136ecb782e7396f7330e6bb3d069763d866612da3cf93cdf55b5.
				"0439218681f9108b470d2379cf589ef47e60dc5888ee49ec70071671d74ca9c6",
				// calldata_hash. (here h(0, 0) where h is the Pedersen hash
				// function).
				"49ee3eba8c1600700ee1b87eb599f16716b0b1022947733551fde4050ca6804",
			},
			// contract_address.
			want: "43c6817e70b3fd99a4f120790b2e82c6843df62b573fdadf9e2d677b60ac5eb",
		},
		// Transaction hash calculation. See the following for reference.
		//
		// 	- https://alpha-mainnet.starknet.io/feeder_gateway/get_transaction?transactionHash=e0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75.
		{
			input: []string{
				// Hex representation of []byte("deploy").
				"6465706c6f79",
				// contract_address.
				"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6",
				// Hex representation of keccak.Digest250([]byte("constructor")).
				"28ffe4ff0f226a9107253e17a904099aa4f63a02a5621de0576e5aa71bc5194",
				// calldata_hash.
				"7885ba4f628b6cdcd0b5e6282d2a1b17fe7cd4dd536230c5db3eac890528b4d",
				// chain_id. Hex representation of []byte("SN_MAIN").
				"534e5f4d41494e",
			},
			want: "e0a2e45a80bb827967e096bcf58874f6c01c191e0a0530624cba66a508ae75",
		},
	}
	for _, test := range tests {
		data := []*felt.Felt{}
		for _, item := range test.input {
			data = append(data, new(felt.Felt).SetHex(item))
		}
		want := new(felt.Felt).SetHex(test.want)
		got := ArrayDigest(data...)
		if got.Cmp(want) != 0 {
			t.Errorf("ArrayDigest(%x) = %x, want %x", data, got, want)
		}
	}
}
