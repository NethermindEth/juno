package tendermint

import (
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	name          string
	nodeAddr      *starknet.Address
	expectedState state[starknet.Value, starknet.Hash]
	actions       func(*testStateMachine) [][]starknet.Action
}

func getPrevote(idx int, value *starknet.Value) starknet.Prevote {
	return starknet.Prevote{
		MessageHeader: starknet.MessageHeader{
			Height: types.Height(0),
			Round:  types.Round(0),
			Sender: *getVal(idx),
		},
		ID: utils.HeapPtr(value.Hash()),
	}
}

func getPrecommit(idx int, value *starknet.Value) starknet.Precommit {
	return starknet.Precommit{
		MessageHeader: starknet.MessageHeader{
			Height: types.Height(0),
			Round:  types.Round(0),
			Sender: *getVal(idx),
		},
		ID: utils.HeapPtr(value.Hash()),
	}
}

func newStateMachine(nodeAddr *starknet.Address, vals *validators) *testStateMachine {
	return New(utils.NewNopZapLogger(), *nodeAddr, newApp(), vals, types.Height(0)).(*testStateMachine)
}

func TestReplayWAL(t *testing.T) {
	proposerAddr := getVal(0)
	nonProposerAddr := getVal(3)
	proposedValue := value(1)

	testCases := []testCase{
		{
			name:     "ReplayWAL: proposer crashes right after proposing",
			nodeAddr: proposerAddr,
			expectedState: state[starknet.Value, starknet.Hash]{
				height:      0,
				round:       0,
				step:        types.StepPrevote,
				lockedRound: -1,
				validRound:  -1,
			},
			actions: func(s *testStateMachine) [][]starknet.Action {
				return [][]starknet.Action{
					s.ProcessStart(0),
				}
			},
		},
		{
			name:     "ReplayWAL: non proposer crashes right before commit",
			nodeAddr: nonProposerAddr,
			expectedState: state[starknet.Value, starknet.Hash]{
				height:                        0,
				round:                         0,
				step:                          types.StepPrecommit,
				validValue:                    &proposedValue,
				lockedValue:                   &proposedValue,
				timeoutPrevoteScheduled:       true,
				lockedValueAndOrValidValueSet: true,
			},
			actions: func(s *testStateMachine) [][]starknet.Action {
				prevote0 := getPrevote(0, &proposedValue)
				prevote1 := getPrevote(1, &proposedValue)
				prevote2 := getPrevote(2, &proposedValue)
				precommit0 := getPrecommit(0, &proposedValue)
				proposalMessage := starknet.Proposal{
					MessageHeader: starknet.MessageHeader{
						Height: types.Height(0),
						Round:  types.Round(0),
						Sender: *proposerAddr,
					},
					ValidRound: types.Round(-1),
					Value:      &proposedValue,
				}

				return [][]starknet.Action{
					s.ProcessStart(0),
					s.ProcessProposal(&proposalMessage),
					s.ProcessPrevote(&prevote0),
					s.ProcessPrevote(&prevote1),
					s.ProcessPrevote(&prevote2),
					s.ProcessPrecommit(&precommit0),
				}
			},
		},
		{
			name:     "ReplayWAL: a test that requires timeouts",
			nodeAddr: nonProposerAddr,
			expectedState: state[starknet.Value, starknet.Hash]{
				height:      0,
				round:       0,
				step:        types.StepPrevote,
				validRound:  -1,
				lockedRound: -1,
			},
			actions: func(s *testStateMachine) [][]starknet.Action {
				timeout := types.Timeout{
					Step:   types.StepPropose,
					Height: 0,
					Round:  0,
				}
				return [][]starknet.Action{
					s.ProcessStart(0),
					s.ProcessTimeout(timeout),
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vals := newVals()
			for i := range 4 {
				vals.addValidator(*getVal(i))
			}

			before := newStateMachine(tc.nodeAddr, vals)
			walEntries := []starknet.WALEntry{}
			for _, actions := range tc.actions(before) {
				for _, action := range actions {
					if action, ok := action.(*starknet.WriteWAL); ok {
						walEntries = append(walEntries, action.Entry)
					}
				}
			}
			assert.Equal(t, tc.expectedState, before.state)

			after := newStateMachine(tc.nodeAddr, vals)
			for _, walEntry := range walEntries {
				after.ProcessWAL(walEntry)
			}
			assert.Equal(t, tc.expectedState, after.state)

			assert.Equal(t, before.voteCounter, after.voteCounter)
		})
	}
}
