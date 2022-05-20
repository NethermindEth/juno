package pedersen

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"
)

// BenchmarkDigest runs a benchmark on the Digest function by hashing a
// *big.Int with a value of 0 N times.
func BenchmarkDigest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Digest(new(big.Int))
	}
}

func ExampleDigest() {
	a, _ := new(big.Int).SetString("3d937c035c878245caf64531a5756109c53068da139362728feb561405371cb", 16)
	b, _ := new(big.Int).SetString("208a0a10250e382e1e4bbe2880906c2791bf6275695e02fbbc6aeff9cd8b31a", 16)
	fmt.Printf("%x\n", Digest(a, b))

	// Output:
	// 30e480bed5fe53fa909cc0f8c4d99b8f9f2c016be4c41e13a4848797979c662
}

// TestDigest does a basic test of the Pedersen hash function where the
// test cases chosen are the canonical ones that appear in the Python
// implementation of the same function by Starkware.
func TestDigest(t *testing.T) {
	// See https://github.com/starkware-libs/starkex-resources/blob/44a15c7d1bdafda15766ea0fc2e0866e970e39c1/crypto/starkware/crypto/signature/signature_test_data.json#L85-L96.
	var tests = [...]struct {
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
		a, _ := new(big.Int).SetString(test.input1, 16)
		b, _ := new(big.Int).SetString(test.input2, 16)
		want, _ := new(big.Int).SetString(test.want, 16)
		got := Digest(a, b)
		if got.Cmp(want) != 0 {
			t.Errorf("Digest(%x, %x) = %x, want %x", a, b, got, want)
		}
	}
}

func BenchmarkArrayDigest(b *testing.B) {
	n := 20
	data := make([]*big.Int, n)
	max := curve.Params().P
	seed := time.Now().UnixNano()
	for i := range data {
		data[i] = new(big.Int).Rand(rand.New(rand.NewSource(seed)), max)
	}

	b.Run(fmt.Sprintf("Benchmark pedersen.ArrayDigest over %d big.Ints", n), func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ArrayDigest(data...)
		}
	})
}

func TestArrayDigest(t *testing.T) {
	var tests = [...]struct {
		input []string
		want  string
	}{
		{
			input: []string{"1", "2", "3", "4", "5"},
			want:  "79c2de2c34baea4a6aa66288140b205e075dd05177c3e05222f48fb6808454a",
		},
		{
			input: []string{
				"3ca0cfe4b3bc6ddf346d49d06ea0ed34e621062c0e056c1d0405d266e10268a",
				"5668060aa49730b7be4801df46ec62de53ecd11abe43a32873000c36e8dc1f",
				"3b056f100f96fb21e889527d41f4e39940135dd7a6c94cc6ed0268ee89e5615",
				"7122e9063d239d89d4e336753845b76f2b33ca0d7f0c1acd4b9fe974994cc19",
				"109f720a79e2a41471f054ca885efd90c8cfbbec37991d1b6343991e0a3e740",
			},
			want: "3b4649f0914d7a85ae0bae94c33125bcbbe6a8a60091466b5d15b0c3d77c53e",
		},
		// Contract address computation (https://docs.starknet.io/docs/Contracts/contract-address/)
		// The data below comes from the following command:
		// `starknet get_transaction --hash 0x1b50380d45ebd70876518203f131a12428b2ac1a3a75f1a74241a4abdd614e8 --network=alpha-goerli`
		{
			input: []string{
				// big-endian hex representation of `STARKNET_CONTRACT_ADDRESS`
				"535441524b4e45545f434f4e54524143545f41444452455353",
				// caller_address (always zero in current starknet version)
				"0",
				// salt
				"5bebda1b28ba6daa824126577b9fbc984033e8b18360f5e1ef694cb172c7aa5",
				// contract_hash
				// see `starknet get_block --blockHash <hash> --network=alpha-goerli` where `<hash>` is from get_transaction output
				"0439218681f9108b470d2379cf589ef47e60dc5888ee49ec70071671d74ca9c6",
				// calldata_hash (empty calldata for this contract, hash is defined to be h(0, 0))
				// see https://docs.starknet.io/docs/Hashing/hash-functions/#array-hashing
				Digest(new(big.Int), new(big.Int)).Text(16),
			},
			// contract_address
			want: "43c6817e70b3fd99a4f120790b2e82c6843df62b573fdadf9e2d677b60ac5eb",
		},
	}
	for _, test := range tests {
		data := make([]*big.Int, len(test.input))
		for i, item := range test.input {
			data[i], _ = new(big.Int).SetString(item, 16)
		}
		want, _ := new(big.Int).SetString(test.want, 16)
		got := ArrayDigest(data...)
		if got.Cmp(want) != 0 {
			t.Errorf("ArrayDigest(%x) = %x, want %x", data, got, want)
		}
	}
}
