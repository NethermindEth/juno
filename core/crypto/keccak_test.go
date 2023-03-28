package crypto_test

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	t.Parallel()

	for _, test := range tests {
		test := test
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()
			d, err := crypto.StarknetKeccak([]byte(test.input))
			require.NoError(t, err)

			got := fmt.Sprintf("%x", d.Bytes())
			assert.Equal(t, test.want, got)
		})
	}
}
