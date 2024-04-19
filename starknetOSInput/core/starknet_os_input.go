package osinput

import (
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/vm"
)

func GetDeclaredClasses(su *core.StateUpdate, oldState core.StateReader) ([]core.Class, error) {
	var classesToGet []*felt.Felt
	for classHash := range su.StateDiff.DeclaredV1Classes {
		classesToGet = append(classesToGet, &classHash)
	}
	classesToGet = append(classesToGet, su.StateDiff.DeclaredV0Classes...)

	var declaredClasses []core.Class
	for _, classHash := range classesToGet {
		decClass, err := oldState.Class(classHash)
		if err != nil {
			return nil, err
		}
		declaredClasses = append(declaredClasses, decClass.Class)
	}
	return declaredClasses, nil
}

// GenerateStarknetOSInput generates the starknet OS input, given a block, state and classes.
func GenerateStarknetOSInput(oldState core.StateReader, newState core.StateReader, oldBlock core.Block, vm vm.VM, vmInput VMParameters) (*StarknetOsInput, error) {
	txnExecInfo, err := TxnExecInfo(vm, &vmInput)
	if err != nil {
		return nil, err
	}

	osInput, err := calculateOSInput(oldBlock, oldState, newState, *txnExecInfo)
	if err != nil {
		return nil, err
	}
	return osInput, nil
}

// Todo: update to use vm.TransactionTrace instead of TransactionExecutionInfo?
func calculateOSInput(block core.Block, oldstate core.StateReader, newstate core.StateReader, execInfo []TransactionExecutionInfo) (*StarknetOsInput, error) {
	osinput := &StarknetOsInput{}

	// Todo: complete
	stateSelector, err := get_os_state_selector(osinput.Transactions, execInfo, &osinput.GeneralConfig)
	if err != nil {
		return nil, err
	}

	classHashToCompiledClassHash, err := getClassHashToCompiledClassHash(oldstate, stateSelector.ClassHashes)
	if err != nil {
		return nil, err
	}
	classHashKeys := []felt.Felt{}
	for _, key := range classHashToCompiledClassHash {
		classHashKeys = append(classHashKeys, key)
	}

	contractStateCommitmentInfo, err := getTrieCommitmentInfo(oldstate, newstate, stateSelector.ContractAddresses)
	if err != nil {
		return nil, err
	}
	osinput.ContractStateCommitmentInfo = *contractStateCommitmentInfo

	classStateCommitmentInfo, err := getTrieCommitmentInfo(oldstate, newstate, classHashKeys)
	if err != nil {
		return nil, err
	}
	osinput.ContractClassCommitmentInfo = *classStateCommitmentInfo

	for _, tx := range block.Transactions {
		osinput.Transactions = append(osinput.Transactions, tx)

		switch decTxn := tx.(type) {
		case *core.DeclareTransaction:
			decClass, err := oldstate.Class(decTxn.ClassHash)
			if err != nil {
				return nil, err
			}
			switch class := decClass.Class.(type) {
			case *core.Cairo0Class:
				osinput.DeprecatedCompiledClasses[*decTxn.ClassHash] = *class
			case *core.Cairo1Class:
				osinput.CompiledClasses[*decTxn.ClassHash] = *class.Compiled
			default:
				return nil, errors.New("unknown class type")
			}
		}
	}

	initClassHashToCompiledClassHash, err := getInitialClassHashToCompiledClassHash(oldstate, classHashKeys)
	if err != nil {
		return nil, err
	}
	osinput.ClassHashToCompiledClassHash = initClassHashToCompiledClassHash

	// Todo: CompiledClassVisitedPcs??

	contractStates, err := getContracts(oldstate, stateSelector.ContractAddresses)
	if err != nil {
		return nil, err
	}
	osinput.Contracts = contractStates

	osinput.GeneralConfig = LoadExampleStarknetOSConfig()

	osinput.BlockHash = *block.Hash
	return osinput, nil
}

func LoadExampleStarknetOSConfig() StarknetGeneralConfig {
	seqAddr, _ := new(felt.Felt).SetString("0xfb3d5e8dec6dfef87f2b48dd3fbe9a455ed188636f6638a8f8bce4555f7938")
	feeTokenAddr, _ := new(felt.Felt).SetString("0x3400a86fdc294a70fac1cf84f81a2127419359096b846be9814786d4fc056b8")
	return StarknetGeneralConfig{
		StarknetOsConfig: StarknetOsConfig{
			ChainID:                   *new(felt.Felt).SetBytes([]byte{0x05, 0x9b, 0x01, 0xba, 0x26, 0x2c, 0x99, 0x9f, 0x26, 0x17, 0x41, 0x2f, 0xfb, 0xba, 0x78, 0x0f, 0x80, 0xb0, 0x10, 0x3d, 0x92, 0x8c, 0xbc, 0xe1, 0xae, 0xcb, 0xaa, 0x50, 0xde, 0x90, 0xab, 0xda}),
			DeprecatedFeeTokenAddress: felt.Felt{},
			FeeTokenAddress:           *feeTokenAddr,
		},
		GasPriceBounds:       GasPriceBounds{},
		InvokeTxMaxNSteps:    3000000,
		ValidateMaxNSteps:    1000000,
		DefaultEthPriceInFri: *new(felt.Felt),
		ConstantGasPrice:     false,
		SequencerAddress:     seqAddr,
		CairoResourceFeeWeights: map[string]float64{
			"n_steps":             1.0,
			"output_builtin":      0.0,
			"pedersen_builtin":    0.0,
			"range_check_builtin": 0.0,
			"ecdsa_builtin":       0.0,
			"bitwise_builtin":     0.0,
			"ec_op_builtin":       0.0,
			"keccak_builtin":      0.0,
			"poseidon_builtin":    0.0,
		},
		EnforceL1HandlerFee: true,
		UseKzgDa:            true,
	}
}

func getClassHashToCompiledClassHash(reader core.StateReader, classHashes []felt.Felt) (map[felt.Felt]felt.Felt, error) {
	classHashToCompiledClassHash := map[felt.Felt]felt.Felt{}
	for _, classHash := range classHashes {
		decClass, err := reader.Class(&classHash)
		if err != nil {
			return nil, err
		}
		switch t := decClass.Class.(type) {
		case *core.Cairo1Class:
			classHashToCompiledClassHash[classHash] = *t.Compiled.Hash()
		}
	}
	return classHashToCompiledClassHash, nil

}

func getInitialClassHashToCompiledClassHash(oldstate core.StateReader, classHashKeys []felt.Felt) (map[felt.Felt]felt.Felt, error) {
	classHashToCompiledClassHash := make(map[felt.Felt]felt.Felt)
	for _, key := range classHashKeys {
		class, err := oldstate.Class(&key)
		if err != nil {
			return nil, err
		}
		switch c := (class.Class).(type) {
		case *core.Cairo1Class:
			classHashToCompiledClassHash[key] = *c.Compiled.Hash()
		}
	}
	return classHashToCompiledClassHash, nil
}

// getTrieCommitmentInfo returns the CommitmentInfo (effectievely the set of modified nodes) that results from a Trie update.
func getTrieCommitmentInfo(oldstate core.StateReader, newstate core.StateReader, keys []felt.Felt) (*CommitmentInfo, error) {
	commitmentFacts := map[felt.Felt][]felt.Felt{}

	getStorageNodes := func(state core.StateReader, address felt.Felt) ([]trie.StorageNode, error) {
		sTrie, _, err := state.StorageTrie()
		if err != nil {
			return nil, err
		}
		addrBytes := address.Bytes()
		key := trie.NewKey(251, addrBytes[:])
		addressNodes, err := sTrie.NodesFromRoot(&key)
		if err != nil {
			return nil, err
		}
		return addressNodes, nil
	}

	for _, key := range keys {
		if key.Equal(&felt.Zero) || key.Equal(new(felt.Felt).SetUint64(1)) { // Todo: Hack to make empty state work for initial tests.
			continue
		}
		oldStorageNodes, err := getStorageNodes(oldstate, key)
		if err != nil {
			return nil, err
		}
		newStorageNodes, err := getStorageNodes(newstate, key)
		if err != nil {
			return nil, err
		}

		// Todo: check this. Assumes the nodes are ordered.
		iterFirstNew := 0
		for range oldStorageNodes {
			oldKeyFelt := oldStorageNodes[iterFirstNew].Key().Felt()
			newKeyFelt := newStorageNodes[iterFirstNew].Key().Felt()
			if oldKeyFelt.Equal(&newKeyFelt) {
				iterFirstNew++
				continue
			} else {
				break
			}
		}
		modifiedNodes := newStorageNodes[iterFirstNew:]
		for _, node := range modifiedNodes {
			key := node.Key()
			value := node.Node().Value
			keyLen := key.Len()
			path := new(felt.Felt) // Todo: export trie path function
			commitmentFacts[key.Felt()] = []felt.Felt{*new(felt.Felt).SetUint64(uint64(keyLen)), *value, *path}
		}
	}

	oldStateTrie, _, err := oldstate.StorageTrie()
	if err != nil {
		return nil, err
	}
	prevRoot, err := oldStateTrie.Root()
	if err != nil {
		return nil, err
	}

	newStateTrie, _, err := newstate.StorageTrie()
	if err != nil {
		return nil, err
	}
	newRoot, err := newStateTrie.Root()
	if err != nil {
		return nil, err
	}

	return &CommitmentInfo{
		PreviousRoot:    *prevRoot,
		UpdatedRoot:     *newRoot,
		TreeHeight:      251, // Todo : leave hardcoded
		CommitmentFacts: commitmentFacts,
	}, nil
}

func getContracts(reader core.StateReader, contractAddresses []felt.Felt) (map[felt.Felt]ContractState, error) {
	contractState := map[felt.Felt]ContractState{}
	for _, addr := range contractAddresses {
		root, err := reader.ContractStorageRoot(&addr)
		if err != nil {
			return nil, err
		}
		nonce, err := reader.ContractNonce(&addr)
		if err != nil {
			// run_os.py returns zeros for undeployed contracts
			if err == db.ErrKeyNotFound {
				contractState[addr] = ContractState{
					ContractHash: *new(felt.Felt).SetUint64(0),
					StorageCommitmentTree: PatriciaTree{
						Root:   *new(felt.Felt).SetUint64(0),
						Height: 251,
					},
					Nonce: *new(felt.Felt).SetUint64(0)}
				continue
			}
			return nil, err
		}
		hash, err := reader.ContractClassHash(&addr)
		if err != nil {
			return nil, err
		}
		contractState[addr] = ContractState{
			ContractHash: *hash,
			StorageCommitmentTree: PatriciaTree{
				Root:   *root,
				Height: 251,
			},
			Nonce: *nonce,
		}
	}
	return contractState, nil
}