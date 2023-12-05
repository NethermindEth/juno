package builder

import (
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type State struct {
	reader      core.StateReader
	sd          *StateDiff
	blockNumber uint64
}

func init() { //nolint:gochecknoinits
	blockchain.RegisterCoreTypesToEncoder()
}

func NewState(r core.StateReader, blockNumber uint64) *State {
	return &State{
		reader: r,
		sd: &StateDiff{
			StorageDiffs:      map[felt.Felt]map[felt.Felt]*felt.Felt{},
			Nonces:            map[felt.Felt]*felt.Felt{},
			DeployedContracts: map[felt.Felt]*felt.Felt{},
			DeclaredV0Classes: []*felt.Felt{},
			DeclaredV1Classes: map[felt.Felt]*felt.Felt{},
			ReplacedClasses:   map[felt.Felt]*felt.Felt{},
			Classes:           map[felt.Felt]core.Class{},
		},
		blockNumber: blockNumber,
	}
}

func (s *State) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	if classHash, ok := s.sd.DeployedContracts[*addr]; ok {
		return classHash, nil
	}
	if classHash, ok := s.sd.ReplacedClasses[*addr]; ok {
		return classHash, nil
	}
	return s.reader.ContractClassHash(addr)
}

func (s *State) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	if nonce, ok := s.sd.Nonces[*addr]; ok {
		return nonce, nil
	} else if _, ok := s.sd.DeployedContracts[*addr]; ok {
		return &felt.Felt{}, nil
	}
	return s.reader.ContractNonce(addr)
}

func (s *State) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	if diff, ok := s.sd.StorageDiffs[*addr]; ok {
		if value, ok := diff[*key]; ok {
			return value, nil
		}
	}
	return s.reader.ContractStorage(addr, key)
}

func (s *State) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	if class, ok := s.sd.Classes[*classHash]; ok {
		return &core.DeclaredClass{
			Class: class,
			At:    s.blockNumber,
		}, nil
	}
	return s.reader.Class(classHash)
}

func (s *State) SetStorage(addr, key, value *felt.Felt) {
	if _, ok := s.sd.StorageDiffs[*addr]; !ok {
		s.sd.StorageDiffs[*addr] = map[felt.Felt]*felt.Felt{}
	}
	s.sd.StorageDiffs[*addr][*key] = value
}

var one = new(felt.Felt).SetUint64(1)

func (s *State) IncrementNonce(addr *felt.Felt) error {
	nonce, ok := s.sd.Nonces[*addr]
	if !ok {
		var err error
		nonce, err = s.reader.ContractNonce(addr)
		if err != nil {
			return err
		}
	}
	s.sd.Nonces[*addr] = nonce.Add(nonce, one)
	return nil
}

func (s *State) DeclareClass(classHash, compiledClassHash *felt.Felt, class core.Class) {
	s.sd.DeclaredV1Classes[*classHash] = compiledClassHash
	s.sd.Classes[*classHash] = class
}

func (s *State) ReplaceContractClass(addr, classHash *felt.Felt) {
	if _, ok := s.sd.DeployedContracts[*addr]; ok {
		s.sd.DeployedContracts[*addr] = classHash
	} else {
		s.sd.ReplacedClasses[*addr] = classHash
	}
}

func (s *State) DeployContract(addr, classHash *felt.Felt) {
	s.sd.DeployedContracts[*addr] = classHash
}

func (s *State) Diff() *StateDiff {
	return s.sd
}

func (s *State) Update(sd *StateDiff) {
	// Update is a temporary function that we can use
	// until we either have a Starknet runtime in Go or
	// fulfil blockifier's State interface using the Go<>C<>Rust ffi.

	for addr, diff := range sd.StorageDiffs {
		for key, value := range diff {
			s.SetStorage(&addr, &key, value)
		}
	}

	for addr, nonce := range sd.Nonces {
		s.sd.Nonces[addr] = nonce
	}

	for addr, classHash := range sd.DeployedContracts {
		s.sd.DeployedContracts[addr] = classHash
	}

	s.sd.DeclaredV0Classes = append(s.sd.DeclaredV0Classes, sd.DeclaredV0Classes...)

	for classHash, compiledClassHash := range sd.DeclaredV1Classes {
		s.sd.DeclaredV1Classes[classHash] = compiledClassHash
	}

	for addr, classHash := range sd.ReplacedClasses {
		s.ReplaceContractClass(&addr, classHash)
	}

	for classHash, class := range sd.Classes {
		s.sd.Classes[classHash] = class
	}
}
