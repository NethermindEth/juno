package tendermint_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/core/felt"
)

// Implements Hashable interface
type value uint64

func (t value) Hash() felt.Felt {
	return *new(felt.Felt).SetUint64(uint64(t))
}

// Implements Application[value, felt.Felt] interface
type app struct{}

func newApp() *app { return &app{} }

func (a *app) Value() value {
	return value(rand.Intn(100))
}

func (a *app) Valid(v value) bool {
	return v < 100
}

// Implements Blockchain[value, felt.Felt] interface
type chain struct {
	curHeight uint
	decision  map[uint]value
}

func newChain() *chain {
	return &chain{decision: make(map[uint]value)}
}

func (c *chain) Height() uint {
	return c.curHeight
}

func (c *chain) Commit(v value) error {
	c.decision[c.curHeight] = v
	c.curHeight++
	return nil
}

// Implements Validators[felt.Felt] interface
type validators struct {
	totalVotingPower uint
	vals             []*validator
}

type validator struct {
	addr felt.Felt
	// power is effective weight
	power         uint
	currentWeight int
}

func newVals() *validators { return &validators{} }

func (v *validators) TotalVotingPower(height uint) uint {
	return v.totalVotingPower
}

func (v *validators) ValidatorVotingPower(validatorAddr felt.Felt) uint {
	// since a validator mush stake therefore the voting power will always be more than zero, thus,
	// if an address is provided which is not part of the validator set then 0 will be returned.
	for _, val := range v.vals {
		if val.addr == validatorAddr {
			return val.power
		}
	}
	return 0
}

// Proposer is implements weighted round robin regardless of height and round
func (v *validators) Proposer(_, _ uint) felt.Felt {
	// Add effective wight to current weight
	for _, val := range v.vals {
		val.currentWeight += int(val.power)
	}

	// Find validator with the highest current weight
	valWithHighestWeight := new(validator)
	// valWithHighestWeight := validator{}
	for _, val := range v.vals {
		if val.currentWeight > valWithHighestWeight.currentWeight {
			valWithHighestWeight = val
			break
		}
	}

	// Decrease the chosen proposer's current weight by total voting power
	valWithHighestWeight.currentWeight -= int(v.totalVotingPower)
	return valWithHighestWeight.addr
}

func (v *validators) addValidator(addr felt.Felt, power uint) {
	if power < 1 {
		return
	}

	v.vals = append(v.vals, &validator{addr: addr, power: power})
	v.totalVotingPower += power
}

func TestTendermint(t *testing.T) {
	//t.Run("initial tendermint state", func(t *testing.T) {
	//	s := New()
	//})
	// Dzes nothing, for now, just here to easily check for compilation issues.
	tendermint.New[value, felt.Felt, felt.Felt](newApp(), newChain(), newVals())
	// s := state[value, felt.Felt]{}
	// assert.Nil(t, s.lockedRound)

	validatorSet := newVals()
	validatorSet.addValidator(*(new(felt.Felt).SetUint64(1)), 2)
	validatorSet.addValidator(*(new(felt.Felt).SetUint64(2)), 2)
	validatorSet.addValidator(*(new(felt.Felt).SetUint64(3)), 2)

	fmt.Println("Initialise validators")
	fmt.Println("Total voting power:", validatorSet.totalVotingPower)
	for _, val := range validatorSet.vals {
		fmt.Println("Validator Addr", val.addr.String(), "power", val.power, "currentWeight", val.currentWeight)
	}
	fmt.Println()

	for i := 0; i < 30; i++ {
		fmt.Println("i =", i)
		p := validatorSet.Proposer(0, 0)
		fmt.Println("Proposer is", (&p).String())
		for _, val := range validatorSet.vals {
			fmt.Println("Validator Addr", val.addr.String(), "power", val.power, "currentWeight", val.currentWeight)
		}
		fmt.Println()
	}
}
