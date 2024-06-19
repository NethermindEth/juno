package sync

import (
	"context"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"math/big"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func hexToFelt(str string) *felt.Felt {
	flt := felt.Zero.Clone()

	bint, ok := big.NewInt(0).SetString(str, 16)
	if !ok {
		panic("set fail")
	}

	return flt.SetBigInt(bint)
}

func TestSnapOfflineCopy(t *testing.T) {

	scenarios := []struct {
		name     string
		scenario func(t *testing.T, bc *blockchain.Blockchain) error
	}{
		{
			name: "basic contract",
			scenario: func(t *testing.T, bc *blockchain.Blockchain) error {
				key := new(felt.Felt).SetUint64(uint64(12))
				nonce := new(felt.Felt).SetUint64(uint64(1))
				classHash := new(felt.Felt).SetUint64(uint64(100))

				return bc.Store(
					&core.Block{
						Header: &core.Header{
							Hash:            &felt.Zero,
							ParentHash:      &felt.Zero,
							GlobalStateRoot: hexToFelt("023af75284f387faf318d9a314a5ccf7e792c45bc1981bff5a81ede56f725219"),
						},
					},
					nil,
					&core.StateUpdate{
						NewRoot: hexToFelt("023af75284f387faf318d9a314a5ccf7e792c45bc1981bff5a81ede56f725219"),
						OldRoot: &felt.Zero,
						StateDiff: &core.StateDiff{
							StorageDiffs: nil,
							Nonces: map[felt.Felt]*felt.Felt{
								*key: nonce,
							},
							DeployedContracts: map[felt.Felt]*felt.Felt{
								*key: classHash,
							},
							DeclaredV0Classes: nil,
							DeclaredV1Classes: nil,
							ReplacedClasses:   nil,
						},
					},
					nil)
			},
		},
		{
			name: "basic with storage",
			scenario: func(t *testing.T, bc *blockchain.Blockchain) error {
				key := new(felt.Felt).SetUint64(uint64(12))
				nonce := new(felt.Felt).SetUint64(uint64(1))
				classHash := new(felt.Felt).SetUint64(uint64(100))

				stateDiff := map[felt.Felt]*felt.Felt{}

				for i := 0; i < 10; i++ {
					stateDiff[*new(felt.Felt).SetUint64(uint64(i*10 + 1))] = new(felt.Felt).SetUint64(uint64(i*10 + 2))
				}

				return bc.Store(
					&core.Block{
						Header: &core.Header{
							Hash:            &felt.Zero,
							ParentHash:      &felt.Zero,
							GlobalStateRoot: hexToFelt("0177e2c05d4df22318f5026408b9e50d156a2f90fac81edaea36f88a03593239"),
						},
					},
					nil,
					&core.StateUpdate{
						NewRoot: hexToFelt("0177e2c05d4df22318f5026408b9e50d156a2f90fac81edaea36f88a03593239"),
						OldRoot: &felt.Zero,
						StateDiff: &core.StateDiff{
							Nonces: map[felt.Felt]*felt.Felt{
								*key: nonce,
							},
							DeployedContracts: map[felt.Felt]*felt.Felt{
								*key: classHash,
							},
							StorageDiffs: map[felt.Felt]map[felt.Felt]*felt.Felt{
								*key: stateDiff,
							},
							DeclaredV0Classes: nil,
							DeclaredV1Classes: nil,
							ReplacedClasses:   nil,
						},
					},
					nil)
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			d, err := pebble.NewMem()
			assert.NoError(t, err)
			bc := blockchain.New(d, &utils.Sepolia)

			err = scenario.scenario(t, bc)
			assert.NoError(t, err)

			d2, err := pebble.NewMem()
			assert.NoError(t, err)
			bc2 := blockchain.New(d2, &utils.Sepolia)

			logger, err := utils.NewZapLogger(utils.DEBUG, false)

			syncer := NewSnapSyncer(
				&NoopService{
					blockchain: bc,
				},
				&localStarknetData{bc},
				&snapServer{
					blockchain: bc,
				},
				bc2,
				logger,
			)

			err = syncer.Run(context.Background())
			assert.NoError(t, err)

			state1, closer, err := bc.HeadStateFreakingState()
			assert.NoError(t, err)
			defer closer()

			state2, closer, err := bc2.HeadStateFreakingState()
			assert.NoError(t, err)
			defer closer()

			sr, cr, err := state1.StateAndClassRoot()
			assert.NoError(t, err)

			sr2, cr2, err := state2.StateAndClassRoot()
			assert.NoError(t, err)

			assert.Equal(t, sr, sr2)
			assert.Equal(t, cr, cr2)
		})
	}
}

func TestSnapCopyTrie(t *testing.T) {
	var d db.DB
	d, err := pebble.New("/home/amirul/fastworkscratch3/juno_sepolia/juno-sepolia/", 128000000, 128, utils.NewNopZapLogger())
	assert.NoError(t, err)

	bc := blockchain.New(d, &utils.Sepolia) // Needed because class loader need encoder to be registered
	bc.DoneSnapSync()

	targetdir := "/home/amirul/fastworkscratch3/targetjuno"
	os.RemoveAll(targetdir)

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":9201", nil)
	}()

	var d2 db.DB
	d2, _ = pebble.New(targetdir, 128000000, 128, utils.NewNopZapLogger())
	bc2 := blockchain.New(d2, &utils.Mainnet) // Needed because class loader need encoder to be registered
	// bc2.DoneSnapSync()

	/*
		state, _, err := bc.HeadStateFreakingState()
		assert.NoError(t, err)

		state2, _, err := bc2.HeadStateFreakingState()
		assert.NoError(t, err)

		strie, _, err := state.StorageTrie()
		assert.NoError(t, err)

		strie2, _, err := state2.StorageTrie()
		assert.NoError(t, err)

		counter := 0
		_, err = strie.Iterate(&felt.Zero, func(key, value *felt.Felt) (bool, error) {

			stri1, err := state.StorageTrieForAddr(key)
			if err != nil {
				return false, err
			}

			r1, err := stri1.Root()
			if err != nil {
				return false, err
			}

			stri2, err := state2.StorageTrieForAddr(key)
			if err != nil {
				return false, err
			}

			r2, err := stri2.Root()
			if err != nil {
				return false, err
			}

			ch1, err := state.ContractClassHash(key)
			if err != nil {
				return false, err
			}

			nc1, err := state.ContractNonce(key)
			if err != nil {
				return false, err
			}

			ch2, err := state2.ContractClassHash(key)
			if err != nil {
				return false, err
			}

			nc2, err := state2.ContractNonce(key)
			if err != nil {
				return false, err
			}

			correctRoot := calculateContractCommitment(r2, ch2, nc2)
			storedRoot, err := strie2.Get(key)
			if err != nil {
				return false, err
			}

			fmt.Printf("== %s %s %s\n", key, r1, r2)
			fmt.Printf("=== %s %s %s\n", value, storedRoot, correctRoot)
			fmt.Printf("=== %s %s vs %s %s\n", ch1, nc1, ch2, nc2)

			idx := 0
			_, err = stri1.Iterate(&felt.Zero, func(key, value *felt.Felt) (bool, error) {
				v2, err := stri1.Get(key)
				if err != nil {
					return false, err
				}

				fmt.Printf(" %d %s %s %s\n", idx, key, value, v2)
				if !value.Equal(v2) {
					return false, errors.New("not equal")
				}

				idx++
				return true, nil
			})
			if err != nil {
				return false, err
			}

			counter++
			return counter < 100, nil
		})
		assert.NoError(t, err)
	*/

	logger, err := utils.NewZapLogger(utils.DEBUG, false)
	assert.NoError(t, err)

	syncer := NewSnapSyncer(
		&NoopService{
			blockchain: bc,
		},
		&localStarknetData{bc},
		&snapServer{
			blockchain: bc,
		},
		bc2,
		logger,
	)

	err = syncer.Run(context.Background())
	assert.NoError(t, err)
}

type NoopService struct {
	blockchain *blockchain.Blockchain
}

func (n NoopService) Run(ctx context.Context) error {
	return nil
}

type localStarknetData struct {
	blockchain *blockchain.Blockchain
}

func (l localStarknetData) BlockByNumber(ctx context.Context, blockNumber uint64) (*core.Block, error) {
	return l.blockchain.BlockByNumber(blockNumber)
}

func (l localStarknetData) BlockLatest(ctx context.Context) (*core.Block, error) {
	time.Sleep(time.Second)
	return l.blockchain.Head()
}

func (l localStarknetData) BlockPending(ctx context.Context) (*core.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) Class(ctx context.Context, classHash *felt.Felt) (core.Class, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) StateUpdatePending(ctx context.Context) (*core.StateUpdate, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) StateUpdateWithBlock(ctx context.Context, blockNumber uint64) (*core.StateUpdate, *core.Block, error) {
	//TODO implement me
	panic("implement me")
}

func (l localStarknetData) StateUpdatePendingWithBlock(ctx context.Context) (*core.StateUpdate, *core.Block, error) {
	//TODO implement me
	panic("implement me")
}

var _ starknetdata.StarknetData = &localStarknetData{}
