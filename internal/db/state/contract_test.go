package state

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
)

var contractStates = []*state.ContractState{
	{
		ContractHash: new(felt.Felt).SetBytes([]byte{1, 2, 3}),
		StorageRoot:  new(felt.Felt).SetBytes([]byte{4, 5, 6}),
	},
}

func TestManager_PutContractState(t *testing.T) {
	manager := newTestManager(t)
	defer manager.Close()
	for i, contractState := range contractStates {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			if err := manager.PutContractState(contractState); err != nil {
				t.Error(err)
			}
		},
		)
	}
}

func TestManager_GetContractState(t *testing.T) {
	manager := newTestManager(t)
	defer manager.Close()
	var tests []struct {
		name string
		hash *felt.Felt
		want struct {
			*state.ContractState
			error
		}
	}
	// Build found tests
	for i, contractState := range contractStates {
		tests = append(tests, struct {
			name string
			hash *felt.Felt
			want struct {
				*state.ContractState
				error
			}
		}{
			name: fmt.Sprintf("test found %d", i),
			hash: contractState.Hash(),
			want: struct {
				*state.ContractState
				error
			}{
				ContractState: contractState,
				error:         nil,
			},
		})
	}
	// Build not found tests
	for i := 0; i < len(contractStates); i++ {
		tests = append(tests, struct {
			name string
			hash *felt.Felt
			want struct {
				*state.ContractState
				error
			}
		}{
			name: fmt.Sprintf("test not found %d", i),
			hash: new(felt.Felt).SetBytes([]byte{1, 2, 3}),
			want: struct {
				*state.ContractState
				error
			}{
				ContractState: nil,
				error:         db.ErrNotFound,
			},
		})
	}
	// Store contract states
	for _, contractState := range contractStates {
		if err := manager.PutContractState(contractState); err != nil {
			t.Error(err)
		}
	}
	// Run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := manager.GetContractState(tt.hash)
			if tt.want.error != nil && !errors.Is(err, tt.want.error) {
				t.Errorf("GetContractState() error = %v, want %v", err, tt.want.error)
			}
			if !reflect.DeepEqual(got, tt.want.ContractState) {
				t.Errorf("GetContractState() = %v, want %v", got, tt.want.ContractState)
			}
		})
	}
}

func newTestManager(t *testing.T) *Manager {
	if err := db.InitializeMDBXEnv(t.TempDir(), 2, 0); err != nil {
		t.Fatal(err)
	}
	env, err := db.GetMDBXEnv()
	if err != nil {
		t.Fatal(err)
	}
	stateDb, err := db.NewMDBXDatabase(env, "STATE")
	if err != nil {
		t.Fatal(err)
	}
	contractDefDb, err := db.NewMDBXDatabase(env, "CONTRACT_DEF")
	if err != nil {
		t.Fatal(err)
	}
	return NewManager(stateDb, contractDefDb)
}
