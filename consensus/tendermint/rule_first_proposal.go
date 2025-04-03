package tendermint

/*
Check the upon condition on line 22:

	22: upon {PROPOSAL, h_p, round_p, v, nil} from proposer(h_p, round_p) while step_p = propose do
	23: 	if valid(v) ∧ (lockedRound_p = −1 ∨ lockedValue_p = v) then
	24: 		broadcast {PREVOTE, h_p, round_p, id(v)}
	25: 	else
	26: 		broadcast {PREVOTE, h_p, round_p, nil}
	27:		step_p ← prevote

The implementation uses nil as -1 to avoid using int type.

Since the value's id is expected to be unique the id can be used to compare the values.
*/
func (t *Tendermint[V, H, A]) line22(vr round, proposalFromProposer, validProposal bool, vID H) {
	if vr == -1 && proposalFromProposer && t.state.s == propose {
		var votedID *H
		if validProposal && (t.state.lockedRound == -1 || (*t.state.lockedValue).Hash() == vID) {
			votedID = &vID
		}
		t.sendPrevote(votedID)
	}
}
