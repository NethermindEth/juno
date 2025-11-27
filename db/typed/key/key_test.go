package key_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/typed/key"
	"github.com/stretchr/testify/require"
)

func runLiteralTest[S key.Serializer[V], V any](
	t *testing.T,
	_ S,
	value V,
	expected string,
) {
	t.Helper()
	t.Run(fmt.Sprintf("%T", *new(S)), func(t *testing.T) {
		var vs S

		actualBytes := vs.Marshal(value)
		require.Equal(t, expected, hex.EncodeToString(actualBytes))
	})
}

func TestKeySerializer(t *testing.T) {
	runLiteralTest(t, key.Empty, struct{}{}, "")

	runLiteralTest(t, key.Bytes, []byte("hello"), "68656c6c6f")

	runLiteralTest(t, key.Uint64, 2025112020251120, "000731d42298f1f0")

	feltInput := felt.FromUint64[felt.Felt](2025112120251121)
	feltOutput := "000000000000000000000000000000000000000000000000000731d4288ed2f1"

	runLiteralTest(t, key.Felt, &feltInput, feltOutput)

	runLiteralTest(t, key.Address, (*felt.Address)(&feltInput), feltOutput)

	runLiteralTest(t, key.ClassHash, (*felt.ClassHash)(&feltInput), feltOutput)

	runLiteralTest(t, key.SierraClassHash, (*felt.SierraClassHash)(&feltInput), feltOutput)

	runLiteralTest(t, key.TransactionHash, (*felt.TransactionHash)(&feltInput), feltOutput)

	runLiteralTest(
		t,
		key.Marshal[db.BlockNumIndexKey](),
		db.BlockNumIndexKey{
			Number: 2025112220251122,
			Index:  2025112320251123,
		},
		"000731d42e84b3f2000731d4347a94f3",
	)
}
