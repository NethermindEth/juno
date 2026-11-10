package core

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/cursor"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
	"github.com/stretchr/testify/require"
)

type plannerTransaction Transaction
type plannerTransactionHash transactionHashProjection

func transactionTags(t testing.TB) CBORTransactionTags {
	t.Helper()
	tag := func(value any) uint64 {
		data, err := cbor.Marshal(value)
		require.NoError(t, err)
		n, err := fastcbor.TagOf(data)
		require.NoError(t, err)
		return n
	}
	return CBORTransactionTags{
		DeclareTransaction: tag(DeclareTransaction{}), DeployTransaction: tag(DeployTransaction{}),
		InvokeTransaction: tag(InvokeTransaction{}), L1HandlerTransaction: tag(L1HandlerTransaction{}),
		DeployAccountTransaction: tag(DeployAccountTransaction{}),
	}
}

func transactionTag(tag uint64, wide bool) []byte {
	if wide {
		return binary.BigEndian.AppendUint64([]byte{0xdb}, tag)
	}
	switch {
	case tag < 24:
		return []byte{0xc0 | byte(tag)}
	case tag <= math.MaxUint8:
		return []byte{0xd8, byte(tag)}
	case tag <= math.MaxUint16:
		return binary.BigEndian.AppendUint16([]byte{0xd9}, uint16(tag))
	case tag <= math.MaxUint32:
		return binary.BigEndian.AppendUint32([]byte{0xda}, uint32(tag))
	default:
		return binary.BigEndian.AppendUint64([]byte{0xdb}, tag)
	}
}

func checkTransactionPlanners(t *testing.T, data []byte, tags CBORTransactionTags) {
	t.Helper()
	checkProjectionPlanner(t, data, func(v plannerTransaction) Transaction { return Transaction(v) })
	checkProjectionPlanner(t, data, func(v plannerTransactionHash) transactionHashProjection { return transactionHashProjection(v) })
	var direct, routed Transaction
	rest, err := DecodeTransactionFirst(data, &direct, tags)
	wantRest, wantErr := fastcbor.UnmarshalFirst(data, &routed)
	require.Equal(t, wantErr, err)
	require.Equal(t, wantRest, rest)
	require.Equal(t, routed, direct)
}

func randomTransactions(rng *rand.Rand) []Transaction {
	f := func() *felt.Felt {
		if rng.IntN(3) == 0 {
			return nil
		}
		return felt.NewFromUint64[felt.Felt](rng.Uint64())
	}
	felts := func() felt.Slice[felt.Felt] {
		if rng.IntN(3) == 0 {
			return nil
		}
		out := make(felt.Slice[felt.Felt], rng.IntN(8))
		for i := range out {
			out[i] = felt.FromUint64[felt.Felt](rng.Uint64())
		}
		return out
	}
	version := func() *TransactionVersion {
		value := f()
		if value == nil {
			return nil
		}
		v := TransactionVersion(*value)
		return &v
	}
	bounds := func() map[Resource]ResourceBounds {
		if rng.IntN(3) == 0 {
			return nil
		}
		out := make(map[Resource]ResourceBounds)
		for n := rng.IntN(8); n > 0; n-- {
			out[Resource(rng.Uint32())] = ResourceBounds{MaxAmount: rng.Uint64(), MaxPricePerUnit: f()}
		}
		return out
	}
	deploy := func() DeployTransaction {
		return DeployTransaction{TransactionHash: f(), ContractAddressSalt: f(), ContractAddress: f(), ClassHash: f(), ConstructorCallData: felts(), Version: version()}
	}
	facts := []felt.Felt(felts())
	if len(facts) == 0 {
		facts = nil
	} // omitempty deliberately omits both nil and empty.
	return []Transaction{
		&DeclareTransaction{TransactionHash: f(), ClassHash: f(), SenderAddress: f(), MaxFee: f(), TransactionSignature: felts(), Nonce: f(), Version: version(), CompiledClassHash: f(), ResourceBounds: bounds(), Tip: rng.Uint64(), PaymasterData: felts(), AccountDeploymentData: felts(), NonceDAMode: DataAvailabilityMode(rng.Uint32()), FeeDAMode: DataAvailabilityMode(rng.Uint32())},
		new(deploy()),
		&InvokeTransaction{TransactionHash: f(), CallData: felts(), TransactionSignature: felts(), MaxFee: f(), ContractAddress: f(), Version: version(), EntryPointSelector: f(), Nonce: f(), SenderAddress: f(), ResourceBounds: bounds(), Tip: rng.Uint64(), PaymasterData: felts(), AccountDeploymentData: felts(), NonceDAMode: DataAvailabilityMode(rng.Uint32()), FeeDAMode: DataAvailabilityMode(rng.Uint32()), ProofFacts: facts},
		&L1HandlerTransaction{TransactionHash: f(), ContractAddress: f(), EntryPointSelector: f(), Nonce: f(), CallData: felts(), Version: version()},
		&DeployAccountTransaction{DeployTransaction: deploy(), MaxFee: f(), TransactionSignature: felts(), Nonce: f(), ResourceBounds: bounds(), Tip: rng.Uint64(), PaymasterData: felts(), NonceDAMode: DataAvailabilityMode(rng.Uint32()), FeeDAMode: DataAvailabilityMode(rng.Uint32())},
	}
}

func TestTypedTransactions(t *testing.T) {
	tags := transactionTags(t)
	rng := rand.New(rand.NewPCG(133, 739))
	for range 64 {
		for _, value := range randomTransactions(rng) {
			data, err := cbor.Marshal(value)
			require.NoError(t, err)
			checkTransactionPlanners(t, data, tags)
			checkReceiptOwned(t, data, value)
			var hash felt.Felt
			if value.Hash() != nil {
				hash = *value.Hash()
			}
			checkReceiptOwned(t, data, transactionHashProjection{TransactionHash: hash})
			var direct Transaction
			require.NoError(t, DecodeTransaction(data, &direct, tags))
			require.Equal(t, value, direct)
		}
	}
	for _, data := range [][]byte{{0xf6}, {0xf7}} {
		checkTransactionPlanners(t, data, tags)
		checkReceiptOwned[Transaction](t, data, nil)
	}
}

func TestTypedTransactionFailures(t *testing.T) {
	tags := transactionTags(t)
	for _, value := range randomTransactions(rand.New(rand.NewPCG(63, 271))) {
		data, err := cbor.Marshal(value)
		require.NoError(t, err)
		for end := range len(data) {
			input := data[:end]
			checkTransactionPlanners(t, input, tags)
			var got Transaction
			rest, err := DecodeTransactionFirst(input, &got, tags)
			require.Error(t, err)
			require.Nil(t, rest)
			require.Nil(t, got)
		}
		got := value
		rest, err := DecodeTransactionFirst([]byte{0xf6}, &got, tags)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Same(t, value, got)
		var trailing Transaction
		require.ErrorIs(t, DecodeTransaction(append(bytes.Clone(data), 7), &trailing, tags), fastcbor.ErrExtraneousData)
		require.Nil(t, trailing)
	}
	for _, value := range []Transaction{(*DeclareTransaction)(nil), (*DeployTransaction)(nil), (*InvokeTransaction)(nil), (*L1HandlerTransaction)(nil), (*DeployAccountTransaction)(nil)} {
		got := value
		rest, err := DecodeTransactionFirst([]byte{0xf6}, &got, tags)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Equal(t, value, got, "typed nil interfaces are nonzero destinations")
	}
	for _, limb := range []int{0, 1, 2, 3} {
		var got transactionHashProjection
		got.TransactionHash[limb] = 1
		before := got
		rest, err := fastcbor.UnmarshalFirst([]byte{0xa0}, &got)
		require.ErrorIs(t, err, fastcbor.ErrUnsupported)
		require.Nil(t, rest)
		require.Equal(t, before, got)
	}
}

func TestTypedTransactionWireCorners(t *testing.T) {
	tags := transactionTags(t)
	profiles := []uint64{tags.DeclareTransaction, tags.DeployTransaction, tags.InvokeTransaction, tags.L1HandlerTransaction, tags.DeployAccountTransaction}
	for _, tag := range profiles {
		for _, wide := range []bool{false, true} {
			for _, body := range [][]byte{{0xa0}, {0xf6}, {0xf7}, {0x80}, {0xa1, 0x67, 'V', 'e', 'r', 's', 'i', 'o', 'n', 0x80}} {
				checkTransactionPlanners(t, append(transactionTag(tag, wide), body...), tags)
			}
			duplicate := projectionMap(t, projectionPair{"TransactionHash", []byte{0x84, 1, 2, 3, 4}}, projectionPair{"TRANSACTIONHASH", []byte{0x61, 'x'}})
			wire := append(transactionTag(tag, wide), duplicate...)
			checkTransactionPlanners(t, wire, tags)
			var got Transaction
			require.ErrorIs(t, DecodeTransaction(wire, &got, tags), cursor.ErrUnsupported)
			var hash transactionHashProjection
			require.NoError(t, fastcbor.Unmarshal(wire, &hash))
			require.Equal(t, felt.Felt{1, 2, 3, 4}, hash.TransactionHash)
		}
	}
	for _, tag := range []uint64{0, 1, 2, 3, 4, 23, 24, 255, 256, 65535, 65541, math.MaxUint64} {
		wire := append(transactionTag(tag, false), 0xa0)
		checkTransactionPlanners(t, wire, tags)
		checkTransactionPlanners(t, append(transactionTag(tag, true), wire...), tags)
	}
	for _, head := range [][]byte{nil, {0xa0}, {0xd8}, {0xd9, 1}, {0xda, 0, 1}, {0xdb, 1}, {0xdf}} {
		checkTransactionPlanners(t, head, tags)
	}
	for _, key := range []string{"Version", "NonceDAMode", "FeeDAMode", "ProofFacts", "ResourceBounds"} {
		for _, value := range [][]byte{{0xf6}, {0xf7}, {0x80}, {0xa0}, {0x19, 0xff, 0xff}, {0x1a, 0xff, 0xff, 0xff, 0xff}, {0x1b, 0, 0, 0, 1, 0, 0, 0, 0}, {0x61, 0xff}, {0x9f, 0xff}} {
			checkTransactionPlanners(t, append(transactionTag(tags.InvokeTransaction, false), projectionMap(t, projectionPair{key, value})...), tags)
		}
	}
	// A resource map replaces a duplicate's whole struct, rather than merging it.
	for _, last := range [][]byte{{0xf6}, {0xf7}, {0xa0}, projectionMap(t, projectionPair{"MaxAmount", []byte{9}})} {
		initial := projectionMap(t, projectionPair{"MaxAmount", []byte{7}}, projectionPair{"MaxPricePerUnit", []byte{0x84, 1, 2, 3, 4}})
		body := projectionMap(t, projectionPair{"ResourceBounds", projectionMap(t, projectionPair{uint64(1), initial}, projectionPair{uint64(1), last})})
		wire := append(transactionTag(tags.InvokeTransaction, false), body...)
		checkTransactionPlanners(t, wire, tags)
		var got Transaction
		require.NoError(t, DecodeTransaction(wire, &got, tags))
		want := ResourceBounds{}
		if last[0] == 0xa1 {
			want.MaxAmount = 9
		}
		require.Equal(t, map[Resource]ResourceBounds{1: want}, got.(*InvokeTransaction).ResourceBounds)
	}
	for _, key := range []any{nil, uint64(0), uint64(63), uint64(64), uint64(math.MaxUint32), uint64(math.MaxUint32) + 1, int64(-1), "L1_GAS"} {
		body := projectionMap(t, projectionPair{"ResourceBounds", projectionMap(t, projectionPair{key, []byte{0xa0}})})
		checkTransactionPlanners(t, append(transactionTag(tags.InvokeTransaction, false), body...), tags)
	}
	for _, depth := range []int{30, 31, 32, 33} {
		body := projectionMap(t, projectionPair{"ResourceBounds", []byte{0xa1, 1, 0xa0}}, projectionPair{"unknown", append(bytes.Repeat([]byte{0x81}, depth), 0)})
		wire := append(transactionTag(tags.InvokeTransaction, false), body...)
		checkTransactionPlanners(t, wire, tags)
		var got Transaction
		err := DecodeTransaction(wire, &got, tags)
		if depth <= 31 {
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, err, cursor.ErrDepth)
			require.Nil(t, got)
		}
	}
}

func TestTypedTransactionTagProfile(t *testing.T) {
	// The direct API's mapping comes from its profile, independently of global
	// registration. Exercise all five variants using every CBOR tag head width.
	tags := CBORTransactionTags{DeclareTransaction: 4, DeployTransaction: 24, InvokeTransaction: 256, L1HandlerTransaction: 65536, DeployAccountTransaction: 1 << 40}
	cases := []struct {
		tag  uint64
		want Transaction
	}{{tags.DeclareTransaction, &DeclareTransaction{}}, {tags.DeployTransaction, &DeployTransaction{}}, {tags.InvokeTransaction, &InvokeTransaction{}}, {tags.L1HandlerTransaction, &L1HandlerTransaction{}}, {tags.DeployAccountTransaction, &DeployAccountTransaction{}}}
	for _, tc := range cases {
		for _, wide := range []bool{false, true} {
			var got Transaction
			input := append(transactionTag(tc.tag, wide), 0xa0, 7)
			before := bytes.Clone(input)
			rest, err := DecodeTransactionFirst(input, &got, tags)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			require.Equal(t, before, input)
			require.Equal(t, []byte{7}, rest)
		}
	}
	// Invalid profiles fail before altering global registration.
	require.PanicsWithValue(t, "core: duplicate typed transaction tags", func() { RegisterTypedTransactionReader(CBORTransactionTags{}) })
}

func FuzzTypedTransactions(f *testing.F) {
	tags := transactionTags(f)
	for _, tx := range randomTransactions(rand.New(rand.NewPCG(55, 634))) {
		data, err := cbor.Marshal(tx)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	for _, seed := range [][]byte{{0xa0}, {0xf6}, {0xf7}, append(transactionTag(tags.InvokeTransaction, false), 0xa0)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) { checkTransactionPlanners(t, data, tags) })
}
