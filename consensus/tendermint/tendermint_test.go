package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/mocks"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

// Implements Hashable interface
// Implements Application[value, felt.Felt] interface
type app struct {
	cur uint64
}

func newApp() *app { return &app{} }

func (a *app) Value() starknet.Value {
	a.cur = (a.cur + 1) % 100
	return starknet.Value(felt.FromUint64(a.cur))
}

func (a *app) Valid(v starknet.Value) bool {
	return v != starknet.Value(felt.Zero)
}

// Implements Validators[felt.Felt] interface
type validators struct {
	totalVotingPower types.VotingPower
	vals             []starknet.Address
}

func newVals() *validators { return &validators{} }

func (v *validators) TotalVotingPower(h types.Height) types.VotingPower {
	return v.totalVotingPower
}

func (v *validators) ValidatorVotingPower(validatorAddr starknet.Address) types.VotingPower {
	return 1
}

// Proposer is implements round robin
func (v *validators) Proposer(h types.Height, r types.Round) starknet.Address {
	i := (uint(h) + uint(r)) % uint(v.totalVotingPower)
	return v.vals[i]
}

func (v *validators) addValidator(addr starknet.Address) {
	v.vals = append(v.vals, addr)
	v.totalVotingPower++
}

func getVal(idx int) *starknet.Address {
	return (*starknet.Address)(new(felt.Felt).SetUint64(uint64(idx)))
}

func setupStateMachine(
	t *testing.T,
	numValidators, thisValidator int, //nolint:unparam // This is because in all current tests numValidators is always 4.
) *testStateMachine {
	t.Helper()
	app, vals := newApp(), newVals()

	for i := range numValidators {
		vals.addValidator(*getVal(i))
	}

	thisNodeAddr := getVal(thisValidator)
	ctrl := gomock.NewController(t)
	// Ignore WAL for tests that use this
	db := mocks.NewMockTendermintDB[starknet.Value, starknet.Hash, starknet.Address](ctrl)
	db.EXPECT().SetWALEntry(gomock.Any()).AnyTimes()
	db.EXPECT().Flush().AnyTimes()
	db.EXPECT().DeleteWALEntries(gomock.Any()).AnyTimes()
	return New(db, utils.NewNopZapLogger(), *thisNodeAddr, app, vals, types.Height(0)).(*testStateMachine)
}

func TestThresholds(t *testing.T) {
	tests := []struct {
		n types.VotingPower
		q types.VotingPower
		f types.VotingPower
	}{
		{1, 1, 0},
		{2, 2, 0},
		{3, 2, 0},
		{4, 3, 1},
		{5, 4, 1},
		{6, 4, 1},
		{7, 5, 2},
		{11, 8, 3},
		{15, 10, 4},
		{20, 14, 6},
		{100, 67, 33},
		{150, 100, 49},
		{2000, 1334, 666},
		{2509, 1673, 836},
		{3045, 2030, 1014},
		{7689, 5126, 2562},
		{10032, 6688, 3343},
		{12932, 8622, 4310},
		{15982, 10655, 5327},
		{301234, 200823, 100411},
		{301235, 200824, 100411},
		{301236, 200824, 100411},
	}

	for _, test := range tests {
		assert.Equal(t, test.q, q(test.n))
		assert.Equal(t, test.f, f(test.n))

		assert.True(t, 2*q(test.n) > test.n+f(test.n))
		assert.True(t, 2*(q(test.n)-1) <= test.n+f(test.n))
	}
}

// Todo: Add tests for round change where existing messages are processed
// Todo: Add malicious test
