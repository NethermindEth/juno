//go:build !cgo || no_rust_crypto

package crypto

import "github.com/NethermindEth/juno/core/felt"

// hadesPermutationRounds runs the Hades rounds in place (pure-Go fallback).
// The hashing steps are intentionally inlined for performance reasons.
// The sparse-MDS optimization was measured ~53% slower here (our MDS is mul-free), don't try it.
func hadesPermutationRounds(state *[3]felt.Felt) {
	var squared, stateSum, triple felt.Felt
	for i := range totalRounds {
		full := (i < fullRounds/2) || (totalRounds-i <= fullRounds/2)

		// add round keys
		state[0].Add(&state[0], &roundKeys[i][0])
		state[1].Add(&state[1], &roundKeys[i][1])
		state[2].Add(&state[2], &roundKeys[i][2])

		// S-box: cube state[2] every round, state[0] and state[1] only in full rounds
		state[2].Mul(&state[2], squared.Mul(&state[2], &state[2]))
		if full {
			state[0].Mul(&state[0], squared.Mul(&state[0], &state[0]))
			state[1].Mul(&state[1], squared.Mul(&state[1], &state[1]))
		}

		// mix layer: state = M * state
		stateSum.Add(&state[0], &state[1])
		stateSum.Add(&stateSum, &state[2])
		state[0].Double(&state[0]).Add(&state[0], &stateSum)
		state[1].Sub(&stateSum, state[1].Double(&state[1]))
		state[2].Sub(&stateSum, triple.Double(&state[2]).Add(&triple, &state[2]))
	}
}
