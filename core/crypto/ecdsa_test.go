package crypto_test

import (
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/starknet.go/account"
	"github.com/NethermindEth/starknet.go/curve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/rand"
)

func TestVerify(t *testing.T) {
	tests := map[string]struct {
		key      string
		msg      string
		sigR     string
		sigS     string
		result   bool
		errorMsg string
	}{
		"success": {
			key:    "0x01ef15c18599971b7beced415a40f0c7deacfd9b0d1819e03d723d8bc943cfca",
			msg:    "0x0000000000000000000000000000000000000000000000000000000000000002",
			sigR:   "0x0411494b501a98abd8262b0da1351e17899a0c4ef23dd2f96fec5ba847310b20",
			sigS:   "0x0405c3191ab3883ef2b763af35bc5f5d15b3b4e99461d70e84c654a351a7c81b",
			result: true,
		},
		"fail": {
			key:  "0x077a4b314db07c45076d11f62b6f9e748a39790441823307743cf00d6597ea43",
			msg:  "0x0397e76d1667c4454bfb83514e120583af836f8e32a516765497823eabe16a3f",
			sigR: "0x0173fd03d8b008ee7432977ac27d1e9d1a1f6c98b1a2f05fa84a21c84c44e882",
			sigS: "0x01f2c44a7798f55192f153b4c48ea5c1241fbb69e6132cc8a0da9c5b62a4286e",
		},
		"invalid key": {
			key:      "0x03ee9bffffffffff26ffffffff60ffffffffffffffffffffffffffff004accff",
			msg:      "0x0000000000000000000000000000000000000000000000000000000000000002",
			sigR:     "0x0411494b501a98abd8262b0da1351e17899a0c4ef23dd2f96fec5ba847310b20",
			sigS:     "0x0405c3191ab3883ef2b763af35bc5f5d15b3b4e99461d70e84c654a351a7c81b",
			errorMsg: "not a valid public key",
		},
	}
	for desc, test := range tests {
		t.Run(desc, func(t *testing.T) {
			signature := crypto.Signature{
				R: *utils.HexToFelt(t, test.sigR),
				S: *utils.HexToFelt(t, test.sigS),
			}
			msg := utils.HexToFelt(t, test.msg)
			publicKey := crypto.NewPublicKey(utils.HexToFelt(t, test.key))

			res, err := publicKey.Verify(&signature, msg)
			assert.Equal(t, test.result, res)
			if test.errorMsg != "" {
				assert.ErrorContains(t, err, test.errorMsg)
			}
		})
	}
}

func BenchmarkVerify(b *testing.B) {
	signature := crypto.Signature{
		R: *utils.HexToFelt(b, "0x0411494b501a98abd8262b0da1351e17899a0c4ef23dd2f96fec5ba847310b20"),
		S: *utils.HexToFelt(b, "0x0405c3191ab3883ef2b763af35bc5f5d15b3b4e99461d70e84c654a351a7c81b"),
	}
	msg := utils.HexToFelt(b, "0x0000000000000000000000000000000000000000000000000000000000000002")
	publicKey := crypto.NewPublicKey(utils.HexToFelt(b, "0x01ef15c18599971b7beced415a40f0c7deacfd9b0d1819e03d723d8bc943cfca"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := publicKey.Verify(&signature, msg)
		require.NoError(b, err)
	}
}

func FuzzVerify(f *testing.F) {
	digits := "abcdefABCDEF0123456789"
	msgHash := big.NewInt(0)
	privKey := big.NewInt(0)

	msgHashFelt := &felt.Felt{}
	r := &felt.Felt{}
	s := &felt.Felt{}
	var err error
	f.Fuzz(func(t *testing.T, bytes []byte) {
		for i := range bytes {
			bytes[i] = digits[int(bytes[i])%len(digits)]
		}
		bytes = append(bytes, digits[rand.Intn(len(digits))])
		msgHashFelt, err = curve.Curve.StarknetKeccak(bytes)
		if err != nil {
			t.Fatal(err)
		}
		msgHashFelt.BigInt(msgHash)

		_, pubKeyFelt, privKeyFelt := account.GetRandomKeys()
		privKeyFelt.BigInt(privKey)

		x, y, err := curve.Curve.Sign(msgHash, privKey)
		if err != nil {
			t.Fatal(err)
		}
		r.SetBigInt(x)
		s.SetBigInt(y)

		sig := crypto.Signature{
			R: *r,
			S: *s,
		}

		publicKey := crypto.NewPublicKey(pubKeyFelt)
		ok, err := publicKey.Verify(&sig, msgHashFelt)

		assert.Equal(t, true, ok, msgHashFelt, err)
		assert.NoError(t, err, msgHashFelt, err)
		// TODO: add err cases
	})
}
