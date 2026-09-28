package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/require"
)

func unmarshalRPCStateDiff(t *testing.T, body string) *rpcStateDiff {
	t.Helper()
	var diff rpcStateDiff
	require.NoError(t, json.Unmarshal([]byte(body), &diff))
	return &diff
}

func TestToFGWStateDiff(t *testing.T) {
	empty := `{
		"storage_diffs": {}, "nonces": {}, "deployed_contracts": [], "old_declared_contracts": [],
		"declared_classes": [], "replaced_classes": [], "migrated_compiled_classes": []
	}`
	tests := []struct {
		name string
		diff *rpcStateDiff
		want string
	}{
		{"nil", nil, empty},
		{"empty", &rpcStateDiff{}, empty},
		{"every field", unmarshalRPCStateDiff(t, rpcStateDiffJSON), `{
			"storage_diffs": {"0xa1": [{"key": "0x1", "value": "0x10"}, {"key": "0x2", "value": "0x20"}]},
			"nonces": {"0xa1": "0x5"},
			"deployed_contracts": [{"address": "0xa2", "class_hash": "0xc1"}],
			"old_declared_contracts": ["0xc0"],
			"declared_classes": [{"class_hash": "0xc1", "compiled_class_hash": "0xd1"}],
			"replaced_classes": [{"address": "0xa3", "class_hash": "0xc2"}],
			"migrated_compiled_classes": [{"class_hash": "0xc3", "compiled_class_hash": "0xd3"}]
		}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := json.Marshal(toFGWStateDiff(test.diff))
			require.NoError(t, err)
			require.JSONEq(t, test.want, string(got))
		})
	}
}

func TestBuildRound(t *testing.T) {
	block := fixtureBlock(t, fixtureFrom)
	traces := decodeTraceResult(t, syntheticTraces(t, block))

	swapped := slices.Clone(traces)
	swapped[0], swapped[1] = traces[1], traces[0]

	malformed := *block
	malformed.Transactions = slices.Clone(block.Transactions)
	malformed.Transactions[0] = json.RawMessage("nope")

	var block3 confirmedBlock
	require.NoError(t, json.Unmarshal(readFixture(t, "block", "3.json"), &block3))
	traces3, err := os.ReadFile(traceFixture)
	require.NoError(t, err)

	tests := []struct {
		name    string
		number  uint64
		block   *confirmedBlock
		traces  []tracedTransaction
		wantErr string
	}{
		{"fixture block", fixtureFrom, block, traces, ""},
		{
			"fewer traces than transactions", fixtureFrom, block, traces[1:],
			"block 56377: trace has 44 transactions, block has 45",
		},
		{
			"trace for another transaction", fixtureFrom, block, swapped,
			fmt.Sprintf(
				"block 56377: trace tx 0 is %s, block has %s",
				traces[1].TransactionHash, traces[0].TransactionHash,
			),
		},
		{"malformed transaction", fixtureFrom, &malformed, traces, "block 56377: invalid character"},
		{
			"block before 0.13.1", 3, &block3, decodeTraceResult(t, traces3),
			"block 3: invalid pre_confirmed full block: l2_gas_price is required",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := buildRound(test.number, test.block, test.traces)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			plain, err := gunzip(got)
			require.NoError(t, err)
			checkRoundRetainsBlock(t, plain, test.block)
			checkRoundAdapts(t, plain, test.number, test.block)
		})
	}
}

// checkRoundRetainsBlock checks the round carries the block's transactions and receipts verbatim.
func checkRoundRetainsBlock(t *testing.T, round []byte, block *confirmedBlock) {
	t.Helper()
	var got struct {
		Changed      bool              `json:"changed"`
		BlockNumber  *uint64           `json:"block_number"`
		Transactions []json.RawMessage `json:"transactions"`
		Receipts     []json.RawMessage `json:"transaction_receipts"`
		StateDiffs   []json.RawMessage `json:"transaction_state_diffs"`
	}
	require.NoError(t, json.Unmarshal(round, &got))
	require.True(t, got.Changed)
	require.Nil(t, got.BlockNumber, "a round for an explicit block number carries none")
	require.Len(t, got.StateDiffs, len(block.Transactions))

	require.JSONEq(t, marshalJSON(t, block.Transactions), marshalJSON(t, got.Transactions))
	require.JSONEq(t, marshalJSON(t, block.Receipts), marshalJSON(t, got.Receipts))
}

func marshalJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return string(body)
}

// checkRoundAdapts decodes the round the way Juno's sync does and checks the header and the
// converted state diffs survive the trip.
func checkRoundAdapts(t *testing.T, round []byte, number uint64, block *confirmedBlock) {
	t.Helper()
	envelope, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(round))
	require.NoError(t, err)
	require.NoError(t, envelope.Validate())
	preConfirmed, ok := envelope.Update.(starknet.PreConfirmedBlock)
	require.True(t, ok, "round must decode as a full pre-confirmed block, got %T", envelope.Update)
	require.Equal(t, block.Hash, preConfirmed.BlockIdentifier)
	require.Equal(t, preConfirmedStatus, preConfirmed.Status)
	require.Equal(t, uint64(1712213818), preConfirmed.Timestamp)
	require.Equal(t, "0.13.1", preConfirmed.Version)
	require.Equal(t, starknet.Blob, preConfirmed.L1DAMode)
	require.Equal(
		t,
		"0x1176a1bd84444c89232ec27754698e5d2e7e1a7f1539f12027f28b23ec9f3d8",
		preConfirmed.SequencerAddress.String(),
	)

	adapted, err := sn2core.AdaptPreConfirmedBlock(&preConfirmed, number)
	require.NoError(t, err)
	require.Equal(t, number, adapted.Block.Number)
	require.Equal(t, uint64(len(block.Transactions)), adapted.Block.TransactionCount)
	require.Equal(t, block.Hash, adapted.BlockIdentifier)
	require.Len(t, adapted.TransactionStateDiffs, len(block.Transactions))
	require.Equal(t, uint64(0), adapted.TransactionStateDiffs[1].Length(), "empty diff")
	require.Equal(t, uint64(0), adapted.TransactionStateDiffs[2].Length(), "null diff")

	diff := adapted.StateUpdate.StateDiff
	require.Equal(t, "0x10", diff.StorageDiffs[asFelt("0xa1")][asFelt("0x1")].String())
	require.Equal(t, "0x20", diff.StorageDiffs[asFelt("0xa1")][asFelt("0x2")].String())
	require.Equal(t, "0x5", diff.Nonces[asFelt("0xa1")].String())
	require.Equal(t, "0xc1", diff.DeployedContracts[asFelt("0xa2")].String())
	require.Equal(t, []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0xc0")}, diff.DeclaredV0Classes)
	require.Equal(t, "0xd1", diff.DeclaredV1Classes[asFelt("0xc1")].String())
	require.Equal(t, "0xc2", diff.ReplacedClasses[asFelt("0xa3")].String())
	require.Equal(
		t,
		felt.UnsafeFromString[felt.CasmClassHash]("0xd3"),
		diff.MigratedClasses[felt.UnsafeFromString[felt.SierraClassHash]("0xc3")],
	)
}

func asFelt(hex string) felt.Felt {
	return felt.UnsafeFromString[felt.Felt](hex)
}

func TestDecodeRound(t *testing.T) {
	round := preConfirmedFixtures(t)[0].body
	tests := []struct {
		name             string
		gzipped          []byte
		wantTransactions int
		wantErr          string
	}{
		{"fixture round", gzipped(t, round), 45, ""},
		{"no transactions", gzipped(t, []byte(`{"changed": true, "block_identifier": "0x1"}`)), 0, ""},
		{"not gzip", round, 0, "gzip: invalid header"},
		{"not json", gzipped(t, []byte("<html>")), 0, "invalid character"},
		{
			"transactions without receipts and diffs", gzipped(t, []byte(`{"transactions": [{}]}`)), 0,
			"1 transactions, 0 receipts, 0 state diffs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodeRound(test.gzipped)
			if test.wantErr != "" {
				require.ErrorContains(t, err, test.wantErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, err)
			require.Len(t, got.Transactions, test.wantTransactions)
			require.NotNil(t, got.Transactions, "replies encode [], not null")
			require.NotNil(t, got.Receipts, "replies encode [], not null")
			require.NotNil(t, got.StateDiffs, "replies encode [], not null")
		})
	}
}

func TestRoundReply(t *testing.T) {
	stored := gzipped(t, preConfirmedFixtures(t)[0].body)
	decoded := decodedRounds(t)[fixtureFrom]
	number := fixtureFrom

	tests := []struct {
		name         string
		known, shown uint64
		blockNumber  *uint64
		want         replyKind
	}{
		{"whole block", 0, 45, nil, wantFull},
		{"revealed part with number", 0, 15, &number, wantFull},
		{"nothing revealed", 0, 0, nil, wantFull},
		{"delta", 15, 30, nil, wantDelta},
		{"delta with number", 30, 45, &number, wantDelta},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			round, err := decodeRound(stored)
			require.NoError(t, err)

			body, err := round.reply(test.known, test.shown, test.blockNumber)
			require.NoError(t, err)
			plain, err := gunzip(body)
			require.NoError(t, err)
			envelope, err := starknet.DecodePreConfirmedUpdate(bytes.NewReader(plain))
			require.NoError(t, err)
			require.NoError(t, envelope.Validate())
			require.Equal(t, wantUpdate(decoded, test.want, test.known, test.shown), envelope.Update)

			var wantNumber uint64
			if test.blockNumber != nil {
				wantNumber = *test.blockNumber
			}
			require.Equal(t, wantNumber, envelope.BlockNumber)

			untouched, err := decodeRound(stored)
			require.NoError(t, err)
			require.Equal(t, untouched, round, "the window reuses a round across replies")
		})
	}
}
