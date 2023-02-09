package crypto_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
)

func TestStarknetKeccak(t *testing.T) {
	tests := [...]struct {
		input, want string
	}{
		{"", "01d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"},
		{"abc", "0203657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45"},
		{"test", "0022ff5f21f0b81b113e63f7db6da94fedef11b2119b4088b89664fb9a3cb658"},
		{"starknet", "014909ac0d4a034239ea4f7265fac97d189ff7430fec65bce3879ab4b5a8d058"},
		{"keccak", "0335a135a69c769066bbb4d17b2fa3ec922c028d4e4bf9d0402e6f7c12b31813"},
	}
	for _, test := range tests {
		d, err := crypto.StarknetKeccak([]byte(test.input))
		if err != nil {
			t.Fatalf("expected no error but got %s", err)
		}
		got := fmt.Sprintf("%x", d.Bytes())
		if test.want != got {
			t.Errorf("expected hash for \"%s\" = %q but got %q", test.input, test.want, got)
		}
	}
}
