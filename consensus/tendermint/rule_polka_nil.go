package tendermint

import "github.com/NethermindEth/juno/consensus/types"

/*
Check the upon condition on line 44:

	44: upon 2f + 1 {PREVOTE, h_p, round_p, nil} while step_p = prevote do
	45: broadcast {PRECOMMIT, hp, roundp, nil}
	46: step_p ← precommit

Line 36 and 44 for a round are mutually exclusive.
*/
func (t *stateMachine[V, H, A]) uponPolkaNil() bool {
	prevotes := t.messages.Prevotes[t.state.height][t.state.round]

	var vals []A
	for addr, v := range prevotes {
		if v.ID == nil {
			vals = append(vals, addr)
		}
	}

	// TODO: refactor this
	hasQuorum := t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(t.state.height))

	return hasQuorum && t.state.step == types.StepPrevote
}

func (t *stateMachine[V, H, A]) doPolkaNil() Action[V, H, A] {
	return t.setStepAndSendPrecommit(nil)
}
