package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

// Implements Hashable interface
type value uint64

func (t value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(t))
}

// Implements Application[value, felt.Felt] interface
type app struct {
	cur value
}

func newApp() *app { return &app{} }

func (a *app) Value() value {
	a.cur = (a.cur + 1) % 100
	return a.cur
}

func (a *app) Valid(v value) bool {
	return v < 100
}

// Implements Blockchain[value, felt.Felt] interface
type chain struct {
	curHeight            height
	decision             map[height]value
	decisionCertificates map[height][]Precommit[felt.Felt, felt.Felt]
}

func newChain() *chain {
	return &chain{
		decision:             make(map[height]value),
		decisionCertificates: make(map[height][]Precommit[felt.Felt, felt.Felt]),
	}
}

func (c *chain) Height() height {
	return c.curHeight
}

func (c *chain) Commit(h height, v value, precommits []Precommit[felt.Felt, felt.Felt]) {
	c.decision[c.curHeight] = v
	c.decisionCertificates[c.curHeight] = precommits
	c.curHeight++
}

// Implements Validators[felt.Felt] interface
type validators struct {
	totalVotingPower votingPower
	vals             []felt.Felt
}

func newVals() *validators { return &validators{} }

func (v *validators) TotalVotingPower(h height) votingPower {
	return v.totalVotingPower
}

func (v *validators) ValidatorVotingPower(validatorAddr felt.Felt) votingPower {
	return 1
}

// Proposer is implements round robin
func (v *validators) Proposer(h height, r round) felt.Felt {
	i := (uint(h) + uint(r)) % uint(v.totalVotingPower)
	return v.vals[i]
}

func (v *validators) addValidator(addr felt.Felt) {
	v.vals = append(v.vals, addr)
	v.totalVotingPower++
}

func getVal(idx int) *felt.Felt {
	return new(felt.Felt).SetUint64(uint64(idx))
}

func setupStateMachine(
	t *testing.T,
	numValidators, thisValidator int, //nolint:unparam // This is because in all current tests numValidators is always 4.
) *Tendermint[value, felt.Felt, felt.Felt] {
	t.Helper()
	app, chain, vals := newApp(), newChain(), newVals()

	for i := range numValidators {
		vals.addValidator(*getVal(i))
	}

	thisNodeAddr := getVal(thisValidator)
	testDB := newTestTMDB(t)
	return New(testDB, utils.NewNopZapLogger(), *thisNodeAddr, app, chain, vals) // Todo: pass in a non-nil DB
}

func TestThresholds(t *testing.T) {
	tests := []struct {
		n votingPower
		q votingPower
		f votingPower
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
