package l1data_test

import (
	"errors"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1data"
	"github.com/stretchr/testify/require"
)

func TestDecodePre011(t *testing.T) {
	want := &core.StateDiff{
		StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
			*new(felt.Felt).SetUint64(3): {
				*new(felt.Felt).SetUint64(1): new(felt.Felt).SetUint64(2),
			},
		},
		Nonces: map[felt.Felt]*felt.Felt{
			*new(felt.Felt).SetUint64(3): new(felt.Felt).SetUint64(1),
		},
		DeployedContracts: map[felt.Felt]*felt.Felt{
			*new(felt.Felt).SetUint64(3): new(felt.Felt).SetUint64(4),
		},
	}

	tests := map[string]struct {
		constructorArgs bool
		encodedDiff     []*big.Int
		want            *core.StateDiff
	}{
		"with constructor args": {
			constructorArgs: true,
			encodedDiff: []*big.Int{
				// Deployed contracts
				big.NewInt(4), // Number of elements for deployed contracts
				big.NewInt(3), // Address
				big.NewInt(4), // Class hash
				// Constructor args
				big.NewInt(1), // Number of args
				big.NewInt(1), // The single arg
				// Storage diff
				big.NewInt(1), // Number of diffs
				big.NewInt(3), // Address
				new(big.Int).SetBit(big.NewInt(1), 64, 1), // Number of updates and nonce
				big.NewInt(1), // Key
				big.NewInt(2), // Value
			},
			want: want,
		},
		"without constructor args": {
			encodedDiff: []*big.Int{
				// Deployed contracts
				big.NewInt(2), // Number of elements for deployed contracts
				big.NewInt(3), // Address
				big.NewInt(4), // Class hash
				// Storage diff
				big.NewInt(1), // Number of diffs
				big.NewInt(3), // Address
				new(big.Int).SetBit(big.NewInt(1), 64, 1), // Number of updates and nonce
				big.NewInt(1), // Key
				big.NewInt(2), // Value
			},
			want: want,
		},
		"empty diff": {
			encodedDiff: []*big.Int{
				new(big.Int), // Zero contract deployments
				new(big.Int), // Zero storage diffs
			},
			want: &core.StateDiff{
				DeployedContracts: make(map[felt.Felt]*felt.Felt),
				StorageDiffs:      make(map[felt.Felt]map[felt.Felt]*felt.Felt),
				Nonces:            make(map[felt.Felt]*felt.Felt),
			},
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			got := l1data.DecodePre011Diff(test.encodedDiff, test.constructorArgs)
			require.Equal(t, test.want, got)
		})
	}
}

type fakeState struct {
	isDeployed bool
	stateErr   error
}

func (s *fakeState) ContractIsDeployed(address *felt.Felt) (bool, error) {
	return s.isDeployed, s.stateErr
}

func TestDecode011(t *testing.T) {
	tests := map[string]struct {
		encodedDiff     []*big.Int
		alreadyDeployed bool
		want            *core.StateDiff
	}{
		"empty diff": {
			encodedDiff: []*big.Int{
				// Deployed/Replaced contracts
				big.NewInt(0), // Number of elements for deployed/replaced contracts
				// Declared Classes
				big.NewInt(0), // Number of declared classes
			},
			want: &core.StateDiff{
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{},
				ReplacedClasses:   map[felt.Felt]*felt.Felt{},
				Nonces:            map[felt.Felt]*felt.Felt{},
				StorageDiffs:      map[felt.Felt]map[felt.Felt]*felt.Felt{},
				DeployedContracts: map[felt.Felt]*felt.Felt{},
			},
		},
		"replaced contract with storage diff": {
			encodedDiff: []*big.Int{
				big.NewInt(1), // Number of deployed/replaced contracts
				big.NewInt(2), // Address
				// Nonce is one, one storage update, class info flag is set
				func() *big.Int {
					summary := big.NewInt(1)
					summary.Lsh(summary, 64)
					summary.Add(summary, big.NewInt(1))
					summary.Lsh(summary, 64)
					summary.Add(summary, big.NewInt(1))
					return summary
				}(),
				big.NewInt(3), // Class hash
				big.NewInt(4), // Key
				big.NewInt(5), // Value
				big.NewInt(0), // No declared classes
			},
			alreadyDeployed: true,
			want: &core.StateDiff{
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{},
				ReplacedClasses: map[felt.Felt]*felt.Felt{
					*new(felt.Felt).SetUint64(2): new(felt.Felt).SetUint64(3),
				},
				Nonces: map[felt.Felt]*felt.Felt{
					*new(felt.Felt).SetUint64(2): new(felt.Felt).SetUint64(1),
				},
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
					*new(felt.Felt).SetUint64(2): {
						*new(felt.Felt).SetUint64(4): new(felt.Felt).SetUint64(5),
					},
				},
				DeployedContracts: map[felt.Felt]*felt.Felt{},
			},
		},
		"deployed contract": {
			encodedDiff: []*big.Int{
				big.NewInt(1), // Number of deployed/replaced contracts
				big.NewInt(2), // Address
				// Nonce is zero, no storage updates, class info flag is set
				new(big.Int).SetBit(new(big.Int), 128, 1),
				big.NewInt(3), // Class hash
				big.NewInt(0), // No declared classes
			},
			want: &core.StateDiff{
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{},
				ReplacedClasses:   map[felt.Felt]*felt.Felt{},
				Nonces: map[felt.Felt]*felt.Felt{
					*new(felt.Felt).SetUint64(2): new(felt.Felt).SetUint64(0),
				},
				StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{},
				DeployedContracts: map[felt.Felt]*felt.Felt{
					*new(felt.Felt).SetUint64(2): new(felt.Felt).SetUint64(3),
				},
			},
		},
		"declared class": {
			encodedDiff: []*big.Int{
				big.NewInt(0), // No deployed/replaced contracts
				big.NewInt(1), // One declared class
				big.NewInt(2), // Class hash
				big.NewInt(3), // Compiled class hash
			},
			want: &core.StateDiff{
				DeclaredV1Classes: map[felt.Felt]*felt.Felt{
					*new(felt.Felt).SetUint64(2): new(felt.Felt).SetUint64(3),
				},
				ReplacedClasses:   map[felt.Felt]*felt.Felt{},
				Nonces:            map[felt.Felt]*felt.Felt{},
				StorageDiffs:      map[felt.Felt]map[felt.Felt]*felt.Felt{},
				DeployedContracts: map[felt.Felt]*felt.Felt{},
			},
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			state := &fakeState{
				isDeployed: test.alreadyDeployed,
			}
			got, err := l1data.Decode011Diff(test.encodedDiff, state)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}

	t.Run("state error", func(t *testing.T) {
		testErr := errors.New("test error")
		state := &fakeState{
			stateErr: testErr,
		}
		encodedDiff := []*big.Int{
			big.NewInt(1), // Number of deployed/replaced contracts
			big.NewInt(2), // Address
			// Nonce is zero, no storage updates, class info flag is set
			new(big.Int).SetBit(new(big.Int), 128, 1),
			big.NewInt(3), // Class hash
			big.NewInt(0), // No declared classes
		}
		_, err := l1data.Decode011Diff(encodedDiff, state)
		require.ErrorIs(t, err, testErr)
	})
}
