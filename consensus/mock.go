package consensus

import (
	"bytes"
	"crypto/ed25519"
	"math/rand/v2"
	"time"

	"github.com/NethermindEth/juno/consensus/driver"
	"github.com/NethermindEth/juno/consensus/starknet"
	consensusSync "github.com/NethermindEth/juno/consensus/sync"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/libp2p/go-libp2p/core/crypto"
)

const (
	timeoutBase        = 500 * time.Microsecond
	timeoutRoundFactor = 100 * time.Millisecond
)

type MockServices struct {
	PrivateKey  crypto.PrivKey
	NodeAddress starknet.Address
	Validators  votecounter.Validators[starknet.Address]
	TimeoutFn   driver.TimeoutFn
}

func InitMockServices(hiSeed, loSeed uint64, nodeIndex, nodeCount int) MockServices {
	return MockServices{
		PrivateKey:  mockKey(nodeIndex),
		NodeAddress: mockNodeAddress(nodeIndex),
		Validators:  newMockValidators(hiSeed, loSeed, nodeCount),
		TimeoutFn:   MockTimeoutFn(nodeCount),
	}
}

type mockValidators []int

//nolint:gosec // This is for testing purpose only, so it's safe to use weak random.
func newMockValidators(hiSeed, loSeed uint64, nodeCount int) mockValidators {
	rand := rand.New(rand.NewPCG(hiSeed, loSeed))
	return mockValidators(rand.Perm(nodeCount))
}

func (n mockValidators) TotalVotingPower(height types.Height) types.VotingPower {
	return types.VotingPower(len(n))
}

// Currently we mock one voting power for all validators, to be removed once
// we can query the voting power from the staking contracts.
// The special case is for precommits from sync protocol, to be removed once
// we can extract precommits from the sync protocol messages.
func (n mockValidators) ValidatorVotingPower(height types.Height, addr *starknet.Address) types.VotingPower {
	if addr != nil && *addr == consensusSync.SyncProtocolPrecommitSender {
		return types.VotingPower(len(n))
	}
	return types.VotingPower(1)
}

// Randomised proposer selection, with prime coefficients so that for each height, the order of proposers is different.
func (n mockValidators) Proposer(height types.Height, round types.Round) starknet.Address {
	idx := (int(height)*31 + int(round)*17) % len(n)
	return mockNodeAddress(n[idx])
}

func mockNodeAddress(i int) starknet.Address {
	return felt.FromUint64[starknet.Address](uint64(i))
}

func MockTimeoutFn(nodeCount int) func(types.Step, types.Round) time.Duration {
	return func(step types.Step, round types.Round) time.Duration {
		// Total number of messages are N^2, so the load is roughly proportional to O(N^2)
		// Every round increases the timeout by timeoutRoundFactor. It also guarantees that the timeout will be at least timeoutRoundFactor
		delta := time.Duration(nodeCount*nodeCount)*timeoutBase + time.Duration(round+1)*timeoutRoundFactor

		// The formulae follow the lemma in the paper
		switch step {
		case types.StepPropose:
			prevDelta := delta - timeoutRoundFactor
			return (prevDelta + delta) * 2
		case types.StepPrevote:
			return delta * 2
		case types.StepPrecommit:
			return delta * 2
		}
		return 0
	}
}

func mockKey(nodeIndex int) crypto.PrivKey {
	seed := make([]byte, ed25519.SeedSize)
	seed[0] = byte(nodeIndex)
	reader := bytes.NewReader(seed)
	privKey, _, err := crypto.GenerateEd25519Key(reader)
	if err != nil {
		panic(err)
	}
	return privKey
}
