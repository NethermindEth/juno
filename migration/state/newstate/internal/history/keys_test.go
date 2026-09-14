package history

import (
	"encoding/binary"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/state"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

// TestReadHeadMatchesGetContract pins the Contract record layout blockScratch
// reads to the one core/state writes. The ingestors copy the head values out of
// the record's bytes instead of unmarshalling it, so this is what fails if that
// encoding ever changes.
func TestReadHeadMatchesGetContract(t *testing.T) {
	addr := felt.Address(felt.FromUint64[felt.Felt](0xC0FFEE))
	addrFelt := (*felt.Felt)(&addr)

	tests := map[string]struct {
		nonce        felt.Felt
		classHash    felt.Felt
		deployHeight uint64
	}{
		"typical":        {felt.FromUint64[felt.Felt](7), felt.FromUint64[felt.Felt](0xABCD), 12345},
		"zero nonce":     {felt.Zero, felt.FromUint64[felt.Felt](1), 1},
		"zero height":    {felt.FromUint64[felt.Felt](3), felt.FromUint64[felt.Felt](4), 0},
		"maximum height": {felt.FromUint64[felt.Felt](3), felt.FromUint64[felt.Felt](4), ^uint64(0)},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			memDB := memory.New()
			require.NoError(t, state.WriteContract(
				memDB, addrFelt, test.nonce, test.classHash, test.deployHeight,
			))

			expected, err := state.GetContract(memDB, addrFelt)
			require.NoError(t, err)

			var buf [addressKeyLen]byte
			contractKey := fillAddressKey(buf[:], db.Contract, &addr)
			gotNonce, err := readHeadNonce(memDB, contractKey)
			require.NoError(t, err)
			gotClassHash, gotHeight, err := readHeadClassHash(memDB, contractKey)
			require.NoError(t, err)

			expectedNonce := expected.Nonce.Bytes()
			expectedClassHash := expected.ClassHash.Bytes()
			require.Equal(t, expectedNonce[:], gotNonce[:], "nonce")
			require.Equal(t, expectedClassHash[:], gotClassHash[:], "class hash")
			require.Equal(
				t,
				expected.DeployedHeight,
				binary.BigEndian.Uint64(gotHeight[:]),
				"deploy height",
			)
		})
	}
}

// TestReadHeadFullRecord covers the record form that carries a storage root,
// which state.WriteContract never produces but the running node does. The head
// fields must still be found, since only the deploy height's offset moves.
func TestReadHeadFullRecord(t *testing.T) {
	addr := felt.Address(felt.FromUint64[felt.Felt](0xBEEF))
	addrFelt := (*felt.Felt)(&addr)

	nonce := felt.FromUint64[felt.Felt](11)
	classHash := felt.FromUint64[felt.Felt](22)
	storageRoot := felt.FromUint64[felt.Felt](33)
	const deployHeight = 4242

	record := make([]byte, contractRecordFullLen)
	nonceBytes := nonce.Bytes()
	classHashBytes := classHash.Bytes()
	storageRootBytes := storageRoot.Bytes()
	copy(record[0:], nonceBytes[:])
	copy(record[felt.Bytes:], classHashBytes[:])
	copy(record[2*felt.Bytes:], storageRootBytes[:])
	binary.BigEndian.PutUint64(record[3*felt.Bytes:], deployHeight)

	memDB := memory.New()
	require.NoError(t, memDB.Put(db.ContractKey(addrFelt), record))

	// core/state must agree this is a valid record, or the fixture is wrong.
	decoded, err := state.GetContract(memDB, addrFelt)
	require.NoError(t, err)
	require.Equal(t, uint64(deployHeight), decoded.DeployedHeight)

	var buf [addressKeyLen]byte
	contractKey := fillAddressKey(buf[:], db.Contract, &addr)
	gotNonce, err := readHeadNonce(memDB, contractKey)
	require.NoError(t, err)
	gotClassHash, gotHeight, err := readHeadClassHash(memDB, contractKey)
	require.NoError(t, err)

	require.Equal(t, nonceBytes[:], gotNonce[:])
	require.Equal(t, classHashBytes[:], gotClassHash[:])
	require.Equal(t, uint64(deployHeight), binary.BigEndian.Uint64(gotHeight[:]))
}

// TestReadHeadRejectsMalformedRecord guards the offsets: a record of any other
// length would have its deploy height read from the wrong place.
func TestReadHeadRejectsMalformedRecord(t *testing.T) {
	addr := felt.Address(felt.FromUint64[felt.Felt](1))
	addrFelt := (*felt.Felt)(&addr)

	memDB := memory.New()
	require.NoError(t, memDB.Put(db.ContractKey(addrFelt), make([]byte, felt.Bytes)))

	var buf [addressKeyLen]byte
	contractKey := fillAddressKey(buf[:], db.Contract, &addr)
	_, err := readHeadNonce(memDB, contractKey)
	require.ErrorContains(t, err, "malformed contract record")
}

// TestFillHistoryKeyFrom pins the one-byte bucket move and the length guard it
// depends on: a short key must be rejected rather than half-copied over the
// previous row.
func TestFillHistoryKeyFrom(t *testing.T) {
	addr := felt.Address(felt.FromUint64[felt.Felt](0xC0FFEE))
	addrFelt := (*felt.Felt)(&addr)
	deprecated := db.DeprecatedContractNonceHistoryAtBlockKey(addrFelt, 42)
	require.Len(t, deprecated, blockKeyLen)

	var buf [blockKeyLen]byte
	require.NoError(t, fillHistoryKeyFrom(buf[:], db.ContractNonceHistory, deprecated))
	require.Equal(t, db.ContractNonceHistoryAtBlockKey(addrFelt, 42), buf[:])

	// Poison the buffer, then feed a short key: nothing may change.
	before := buf
	err := fillHistoryKeyFrom(buf[:], db.ContractNonceHistory, deprecated[:blockKeyLen-1])
	require.ErrorContains(t, err, "malformed deprecated history key")
	require.Equal(t, before, buf)
}
