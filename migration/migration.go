package migration

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"sync"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/starknet"
	"github.com/NethermindEth/juno/utils"
	"github.com/bits-and-blooms/bitset"
	"github.com/fxamacker/cbor/v2"
	"github.com/sourcegraph/conc/pool"
)

type schemaMetadata struct {
	Version           uint64
	IntermediateState []byte
}

type Migration interface {
	Before(intermediateState []byte) error
	// Migration should return intermediate state whenever it requests new txn or detects cancelled ctx.
	Migrate(context.Context, db.Transaction, *utils.Network, utils.SimpleLogger) ([]byte, error)
}

type MigrationFunc func(db.Transaction, *utils.Network) error

// Migrate returns f(txn).
func (f MigrationFunc) Migrate(_ context.Context, txn db.Transaction, network *utils.Network, _ utils.SimpleLogger) ([]byte, error) {
	return nil, f(txn, network)
}

// Before is a no-op.
func (f MigrationFunc) Before(_ []byte) error { return nil }

// defaultMigrations contains a set of migrations that can be applied to a database.
// After making breaking changes to the DB layout, add new migrations to this list.
var defaultMigrations = []Migration{
	MigrationFunc(migration0000),
	MigrationFunc(relocateContractStorageRootKeys),
	MigrationFunc(recalculateBloomFilters),
	new(changeTrieNodeEncoding),
	MigrationFunc(calculateBlockCommitments),
	NewBucketMigrator(db.ClassesTrie, migrateTrieRootKeysFromBitsetToTrieKeys).WithKeyFilter(rootKeysFilter(db.ClassesTrie)),
	NewBucketMigrator(db.StateTrie, migrateTrieRootKeysFromBitsetToTrieKeys).WithKeyFilter(rootKeysFilter(db.StateTrie)),
	NewBucketMigrator(db.ContractStorage, migrateTrieRootKeysFromBitsetToTrieKeys).WithKeyFilter(rootKeysFilter(db.ContractStorage)),
	NewBucketMigrator(db.ClassesTrie, migrateTrieNodesFromBitsetToTrieKey(db.ClassesTrie)).WithKeyFilter(nodesFilter(db.ClassesTrie)),
	NewBucketMover(db.Temporary, db.ClassesTrie),
	NewBucketMigrator(db.StateTrie, migrateTrieNodesFromBitsetToTrieKey(db.StateTrie)).WithKeyFilter(nodesFilter(db.StateTrie)),
	NewBucketMover(db.Temporary, db.StateTrie),
	NewBucketMigrator(db.ContractStorage, migrateTrieNodesFromBitsetToTrieKey(db.ContractStorage)).
		WithKeyFilter(nodesFilter(db.ContractStorage)),
	NewBucketMover(db.Temporary, db.ContractStorage),
	NewBucketMigrator(db.StateUpdatesByBlockNumber, changeStateDiffStruct).WithBatchSize(100), //nolint:mnd
	NewBucketMigrator(db.Class, migrateCairo1CompiledClass).WithBatchSize(1_000),              //nolint:mnd
	MigrationFunc(calculateL1MsgHashes),
}

var ErrCallWithNewTransaction = errors.New("call with new transaction")

func MigrateIfNeeded(ctx context.Context, targetDB db.DB, network *utils.Network, log utils.SimpleLogger) error {
	return migrateIfNeeded(ctx, targetDB, network, log, defaultMigrations)
}

func migrateIfNeeded(ctx context.Context, targetDB db.DB, network *utils.Network, log utils.SimpleLogger, migrations []Migration) error {
	/*
		Schema metadata of the targetDB determines which set of migrations need to be applied to the database.
		After a migration is successfully executed, which may update the database, the schema version is incremented
		by 1 by this loop.

		For example;

		Assume an empty DB, which has a schema version of 0. So we start applying the migrations from the
		very first one in the list and increment the schema version as we go. After the loop exists we
		end up with a database which's schema version is len(migrations).

		After running that Juno version for a while, if the user updates its Juno version which adds new
		migrations to the list, MigrateIfNeeded will skip the already applied migrations and only apply the
		new ones. It will be able to do this since the schema version it reads from the database will be
		non-zero and that is what we use to initialise the i loop variable.
	*/
	metadata, err := SchemaMetadata(targetDB)
	if err != nil {
		return err
	}

	currentVersion := uint64(len(migrations))
	if metadata.Version > currentVersion {
		return errors.New("db is from a newer, incompatible version of Juno; upgrade to use this database")
	}

	for i := metadata.Version; i < currentVersion; i++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		log.Infow("Applying database migration", "stage", fmt.Sprintf("%d/%d", i+1, len(migrations)))
		migration := migrations[i]
		if err = migration.Before(metadata.IntermediateState); err != nil {
			return err
		}
		for {
			callWithNewTransaction := false
			if dbErr := targetDB.Update(func(txn db.Transaction) error {
				metadata.IntermediateState, err = migration.Migrate(ctx, txn, network, log)
				switch {
				case err == nil || errors.Is(err, ctx.Err()):
					if metadata.IntermediateState == nil {
						metadata.Version++
					}
					return updateSchemaMetadata(txn, metadata)
				case errors.Is(err, ErrCallWithNewTransaction):
					callWithNewTransaction = true
					return nil
				default:
					return err
				}
			}); dbErr != nil {
				return dbErr
			} else if !callWithNewTransaction {
				break
			}
		}
	}

	return nil
}

// SchemaMetadata retrieves metadata about a database schema from the given database.
func SchemaMetadata(targetDB db.DB) (schemaMetadata, error) {
	metadata := schemaMetadata{}
	txn, err := targetDB.NewTransaction(false)
	if err != nil {
		return metadata, err
	}
	if err := txn.Get(db.SchemaVersion.Key(), func(b []byte) error {
		metadata.Version = binary.BigEndian.Uint64(b)
		return nil
	}); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return metadata, utils.RunAndWrapOnError(txn.Discard, err)
	}

	if err := txn.Get(db.SchemaIntermediateState.Key(), func(b []byte) error {
		return cbor.Unmarshal(b, &metadata.IntermediateState)
	}); err != nil && !errors.Is(err, db.ErrKeyNotFound) {
		return metadata, utils.RunAndWrapOnError(txn.Discard, err)
	}

	return metadata, txn.Discard()
}

// updateSchemaMetadata updates the schema in given database.
func updateSchemaMetadata(txn db.Transaction, schema schemaMetadata) error {
	var (
		version [8]byte
		state   []byte
		err     error
	)
	binary.BigEndian.PutUint64(version[:], schema.Version)
	state, err = cbor.Marshal(schema.IntermediateState)
	if err != nil {
		return err
	}

	if err := txn.Set(db.SchemaVersion.Key(), version[:]); err != nil {
		return err
	}
	return txn.Set(db.SchemaIntermediateState.Key(), state)
}

// migration0000 makes sure the targetDB is empty
func migration0000(txn db.Transaction, _ *utils.Network) error {
	it, err := txn.NewIterator()
	if err != nil {
		return err
	}

	// not empty if valid
	if it.Next() {
		return utils.RunAndWrapOnError(it.Close, errors.New("initial DB should be empty"))
	}
	return it.Close()
}

// relocateContractStorageRootKeys moves contract root keys to new locations and deletes the previous locations.
//
// Before: the key to the root in a contract storage trie was stored at 1+<contractAddress>.
// After: the key to the root of the contract storage trie is stored at 3+<contractAddress>.
//
// This enables us to remove the db.ContractRootKey prefix.
func relocateContractStorageRootKeys(txn db.Transaction, _ *utils.Network) error {
	it, err := txn.NewIterator()
	if err != nil {
		return err
	}

	// Modifying the db with txn.Set/Delete while iterating can cause consistency issues,
	// so we do them separately.

	// Iterate over all entries in the old bucket, copying each into memory.
	// Even with millions of contracts, this shouldn't be too expensive.
	oldEntries := make(map[string][]byte)
	// Previously ContractStorageRoot were stored in the Peer bucket.
	oldPrefix := db.Peer.Key()
	var value []byte
	for it.Seek(oldPrefix); it.Valid(); it.Next() {
		// Stop iterating once we're out of the old bucket.
		if !bytes.Equal(it.Key()[:len(oldPrefix)], oldPrefix) {
			break
		}

		oldKey := it.Key()
		value, err = it.Value()
		if err != nil {
			return utils.RunAndWrapOnError(it.Close, err)
		}
		oldEntries[string(oldKey)] = value
	}

	if err = it.Close(); err != nil {
		return err
	}

	// Move the entries to their new locations.
	for oldKey, value := range oldEntries {
		oldKeyBytes := []byte(oldKey)
		contractAddress, found := bytes.CutPrefix(oldKeyBytes, oldPrefix)
		if !found {
			// Should not happen.
			return errors.New("prefix not found")
		}

		if err := txn.Set(db.ContractStorage.Key(contractAddress), value); err != nil {
			return err
		}
		if err := txn.Delete(oldKeyBytes); err != nil {
			return err
		}
	}

	return nil
}

// recalculateBloomFilters updates bloom filters in block headers to match what the most recent implementation expects
func recalculateBloomFilters(txn db.Transaction, _ *utils.Network) error {
	blockchain.RegisterCoreTypesToEncoder()
	for blockNumber := uint64(0); ; blockNumber++ {
		block, err := blockchain.BlockByNumber(txn, blockNumber)
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				return nil
			}
			return err
		}
		block.EventsBloom = core.EventsBloom(block.Receipts)
		if err = blockchain.StoreBlockHeader(txn, block.Header); err != nil {
			return err
		}
	}
}

// changeTrieNodeEncoding migrates to using a custom encoding for trie nodes
// that minimises memory allocations. Always use new(changeTrieNodeEncoding)
// before calling Before(), otherwise it will panic.
type changeTrieNodeEncoding struct {
	trieNodeBuckets map[db.Bucket]*struct {
		seekTo  []byte
		skipLen int
	}
}

func (m *changeTrieNodeEncoding) Before(_ []byte) error {
	m.trieNodeBuckets = map[db.Bucket]*struct {
		seekTo  []byte
		skipLen int
	}{
		db.ClassesTrie: {
			seekTo:  db.ClassesTrie.Key(),
			skipLen: 1,
		},
		db.StateTrie: {
			seekTo:  db.StateTrie.Key(),
			skipLen: 1,
		},
		db.ContractStorage: {
			seekTo:  db.ContractStorage.Key(),
			skipLen: 1 + felt.Bytes,
		},
	}
	return nil
}

type node struct {
	Value *felt.Felt
	Left  *bitset.BitSet
	Right *bitset.BitSet
}

func (n *node) _WriteTo(buf *bytes.Buffer) (int64, error) {
	if n.Value == nil {
		return 0, errors.New("cannot marshal node with nil value")
	}

	totalBytes := int64(0)

	valueB := n.Value.Bytes()
	wrote, err := buf.Write(valueB[:])
	totalBytes += int64(wrote)
	if err != nil {
		return totalBytes, err
	}

	if n.Left != nil {
		wrote, err := n.Left.WriteTo(buf)
		totalBytes += wrote
		if err != nil {
			return totalBytes, err
		}
		wrote, err = n.Right.WriteTo(buf) // n.Right is non-nil by design
		totalBytes += wrote
		if err != nil {
			return totalBytes, err
		}
	}

	return totalBytes, nil
}

func (n *node) _UnmarshalBinary(data []byte) error {
	if len(data) < felt.Bytes {
		return errors.New("size of input data is less than felt size")
	}
	if n.Value == nil {
		n.Value = new(felt.Felt)
	}
	n.Value.SetBytes(data[:felt.Bytes])
	data = data[felt.Bytes:]

	if len(data) == 0 {
		n.Left = nil
		n.Right = nil
		return nil
	}

	stream := bytes.NewReader(data)

	if n.Left == nil {
		n.Left = new(bitset.BitSet)
		n.Right = new(bitset.BitSet)
	}

	_, err := n.Left.ReadFrom(stream)
	if err != nil {
		return err
	}
	_, err = n.Right.ReadFrom(stream)
	return err
}

func (m *changeTrieNodeEncoding) Migrate(_ context.Context, txn db.Transaction, _ *utils.Network, _ utils.SimpleLogger) ([]byte, error) {
	// If we made n a trie.Node, the encoder would fall back to the custom encoding methods.
	// We instead define a cutom struct to force the encoder to use the default encoding.
	var n node
	var buf bytes.Buffer
	var updatedNodes uint64

	migrateF := func(it db.Iterator, bucket db.Bucket, seekTo []byte, skipLen int) error {
		bucketPrefix := bucket.Key()
		for it.Seek(seekTo); it.Valid(); it.Next() {
			key := it.Key()
			if !bytes.HasPrefix(key, bucketPrefix) {
				delete(m.trieNodeBuckets, bucket)
				break
			}

			if len(key) == skipLen {
				// this entry is not a TrieNode, it is the root key
				continue
			}

			// We cant fit the entire migration in a single transaction but
			// the actual limit is much higher than 1M. But the more updates
			// you queue on a transaction the more memory it uses. 1M is just
			// an arbitrary number that we can both fit in a transaction and
			// dont use too much memory.
			const updatedNodesBatch = 1_000_000
			if updatedNodes >= updatedNodesBatch {
				m.trieNodeBuckets[bucket].seekTo = key
				return ErrCallWithNewTransaction
			}

			v, err := it.Value()
			if err != nil {
				return err
			}

			if err = encoder.Unmarshal(v, &n); err != nil {
				return err
			}

			if _, err = n._WriteTo(&buf); err != nil {
				return err
			}

			if err = txn.Set(key, buf.Bytes()); err != nil {
				return err
			}
			buf.Reset()

			updatedNodes++
		}
		return nil
	}

	iterator, err := txn.NewIterator()
	if err != nil {
		return nil, err
	}

	for bucket, info := range m.trieNodeBuckets {
		if err := migrateF(iterator, bucket, info.seekTo, info.skipLen); err != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, err)
		}
	}
	return nil, iterator.Close()
}

// calculateBlockCommitments calculates the txn and event commitments for each block and stores them separately
func calculateBlockCommitments(txn db.Transaction, network *utils.Network) error {
	var txnLock sync.RWMutex
	workerPool := pool.New().WithErrors().WithMaxGoroutines(runtime.GOMAXPROCS(0))

	for blockNumber := 0; ; blockNumber++ {
		txnLock.RLock()
		block, err := blockchain.BlockByNumber(txn, uint64(blockNumber))
		txnLock.RUnlock()

		if errors.Is(err, db.ErrKeyNotFound) {
			break
		}

		workerPool.Go(func() error {
			commitments, err := core.VerifyBlockHash(block, network, nil)
			if err != nil {
				return err
			}
			txnLock.Lock()
			defer txnLock.Unlock()
			return blockchain.StoreBlockCommitments(txn, block.Number, commitments)
		})
	}

	return workerPool.Wait()
}

func calculateL1MsgHashes(txn db.Transaction, n *utils.Network) error {
	numOfWorkers := runtime.GOMAXPROCS(0)
	workerPool := pool.New().WithErrors().WithMaxGoroutines(numOfWorkers)

	chainHeight, err := blockchain.ChainHeight(txn)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		return err
	}
	blockNumbers := make(chan uint64, 1024) //nolint:mnd
	go func() {
		for bNumber := range chainHeight {
			blockNumbers <- bNumber
		}
		close(blockNumbers)
	}()
	for range numOfWorkers {
		workerPool.Go(func() error {
			for bNumber := range blockNumbers {
				txns, err := blockchain.TransactionsByBlockNumber(txn, bNumber)
				if err != nil {
					return err
				}
				err = blockchain.StoreL1HandlerMsgHashes(txn, txns)
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	return workerPool.Wait()
}

func bitset2Key(bs *bitset.BitSet) *trie.Key {
	bsWords := bs.Bytes()
	if len(bsWords) > felt.Limbs {
		panic("key too long to fit in Felt")
	}

	var bsBytes [felt.Bytes]byte
	for idx, word := range bsWords {
		startBytes := 24 - (idx * 8)
		binary.BigEndian.PutUint64(bsBytes[startBytes:startBytes+8], word)
	}

	f := new(felt.Felt).SetBytes(bsBytes[:])
	fBytes := f.Bytes()
	k := trie.NewKey(uint8(bs.Len()), fBytes[:])
	return &k
}

func migrateTrieRootKeysFromBitsetToTrieKeys(txn db.Transaction, key, value []byte, _ *utils.Network) error {
	var bs bitset.BitSet
	var tempBuf bytes.Buffer
	if err := bs.UnmarshalBinary(value); err != nil {
		return err
	}
	trieKey := bitset2Key(&bs)
	_, err := trieKey.WriteTo(&tempBuf)
	if err != nil {
		return err
	}

	return txn.Set(key, tempBuf.Bytes())
}

var rootKeysLen = map[db.Bucket]int{
	db.ClassesTrie:     1,
	db.StateTrie:       1,
	db.ContractStorage: 1 + felt.Bytes,
}

func rootKeysFilter(target db.Bucket) BucketMigratorKeyFilter {
	return func(key []byte) (bool, error) {
		return len(key) == rootKeysLen[target], nil
	}
}

func nodesFilter(target db.Bucket) BucketMigratorKeyFilter {
	return func(key []byte) (bool, error) {
		return len(key) != rootKeysLen[target], nil
	}
}

func migrateTrieNodesFromBitsetToTrieKey(target db.Bucket) BucketMigratorDoFunc {
	return func(txn db.Transaction, key, value []byte, _ *utils.Network) error {
		var n node
		var tempBuf bytes.Buffer
		if err := n._UnmarshalBinary(value); err != nil {
			return err
		}

		trieNode := trie.Node{
			Value: n.Value,
		}
		if n.Left != nil {
			trieNode.Left = bitset2Key(n.Left)
			trieNode.Right = bitset2Key(n.Right)
		}

		if _, err := trieNode.WriteTo(&tempBuf); err != nil {
			return err
		}

		if err := txn.Delete(key); err != nil {
			return err
		}

		orgKeyPrefix := key[:rootKeysLen[target]]
		bsBytes := key[rootKeysLen[target]:]
		var bs bitset.BitSet
		if err := bs.UnmarshalBinary(bsBytes); err != nil {
			return err
		}

		var keyBuffer bytes.Buffer
		if _, err := bitset2Key(&bs).WriteTo(&keyBuffer); err != nil {
			return err
		}

		orgKeyPrefix[0] = byte(db.Temporary) // move the node to temporary bucket
		return txn.Set(append(orgKeyPrefix, keyBuffer.Bytes()...), tempBuf.Bytes())
	}
}

type oldStorageDiff struct {
	Key   *felt.Felt
	Value *felt.Felt
}
type oldAddressClassHashPair struct {
	Address   *felt.Felt
	ClassHash *felt.Felt
}
type oldDeclaredV1Class struct {
	ClassHash         *felt.Felt
	CompiledClassHash *felt.Felt
}
type oldStateDiff struct {
	StorageDiffs      map[felt.Felt][]oldStorageDiff
	Nonces            map[felt.Felt]*felt.Felt
	DeployedContracts []oldAddressClassHashPair
	DeclaredV0Classes []*felt.Felt
	DeclaredV1Classes []oldDeclaredV1Class
	ReplacedClasses   []oldAddressClassHashPair
}
type oldStateUpdate struct {
	BlockHash *felt.Felt
	NewRoot   *felt.Felt
	OldRoot   *felt.Felt
	StateDiff *oldStateDiff
}

func changeStateDiffStruct(txn db.Transaction, key, value []byte, _ *utils.Network) error {
	old := new(oldStateUpdate)
	if err := encoder.Unmarshal(value, old); err != nil {
		return fmt.Errorf("unmarshal: %v", err)
	}

	storageDiffs := make(map[felt.Felt]map[felt.Felt]*felt.Felt, len(old.StateDiff.StorageDiffs))
	for addr, diff := range old.StateDiff.StorageDiffs {
		newStorageDiffs := make(map[felt.Felt]*felt.Felt, len(diff))
		for _, kv := range diff {
			newStorageDiffs[*kv.Key] = kv.Value
		}
		storageDiffs[addr] = newStorageDiffs
	}

	nonces := make(map[felt.Felt]*felt.Felt, len(old.StateDiff.Nonces))
	maps.Copy(nonces, old.StateDiff.Nonces)

	deployedContracts := make(map[felt.Felt]*felt.Felt, len(old.StateDiff.DeployedContracts))
	for _, deployedContract := range old.StateDiff.DeployedContracts {
		deployedContracts[*deployedContract.Address] = deployedContract.ClassHash
	}

	declaredV1Classes := make(map[felt.Felt]*felt.Felt, len(old.StateDiff.DeclaredV1Classes))
	for _, declaredV1Class := range old.StateDiff.DeclaredV1Classes {
		declaredV1Classes[*declaredV1Class.ClassHash] = declaredV1Class.CompiledClassHash
	}

	replacedClasses := make(map[felt.Felt]*felt.Felt, len(old.StateDiff.ReplacedClasses))
	for _, replacedClass := range old.StateDiff.ReplacedClasses {
		replacedClasses[*replacedClass.Address] = replacedClass.ClassHash
	}

	newValue, err := encoder.Marshal(&core.StateUpdate{
		BlockHash: old.BlockHash,
		NewRoot:   old.NewRoot,
		OldRoot:   old.OldRoot,
		StateDiff: &core.StateDiff{
			StorageDiffs:      storageDiffs,
			Nonces:            nonces,
			DeployedContracts: deployedContracts,
			DeclaredV0Classes: old.StateDiff.DeclaredV0Classes,
			DeclaredV1Classes: declaredV1Classes,
			ReplacedClasses:   replacedClasses,
		},
	})
	if err != nil {
		return fmt.Errorf("marshal state update: %v", err)
	}

	if err := txn.Set(key, newValue); err != nil {
		return fmt.Errorf("set new state update for key: %v", key)
	}

	return nil
}

type declaredClass struct {
	At    uint64
	Class oldCairo1Class
}

type oldCairo1Class struct {
	Abi         string
	AbiHash     *felt.Felt
	EntryPoints struct {
		Constructor []core.SierraEntryPoint
		External    []core.SierraEntryPoint
		L1Handler   []core.SierraEntryPoint
	}
	Program         []*felt.Felt
	ProgramHash     *felt.Felt
	SemanticVersion string
	Compiled        json.RawMessage
}

func (o *oldCairo1Class) Version() uint64 {
	return 1
}

func (o *oldCairo1Class) Hash() (*felt.Felt, error) {
	return nil, nil
}

func migrateCairo1CompiledClass(txn db.Transaction, key, value []byte, _ *utils.Network) error {
	var class declaredClass
	err := encoder.Unmarshal(value, &class)
	if err != nil {
		// assumption that only Cairo0 class causes this error
		targetErr := new(cbor.UnmarshalTypeError)
		if errors.As(err, &targetErr) {
			return nil
		}

		return err
	}

	var coreCompiledClass *core.CompiledClass
	if deprecated, _ := starknet.IsDeprecatedCompiledClassDefinition(class.Class.Compiled); !deprecated {
		var starknetCompiledClass starknet.CompiledClass
		err = json.Unmarshal(class.Class.Compiled, &starknetCompiledClass)
		if err != nil {
			return err
		}

		coreCompiledClass, err = sn2core.AdaptCompiledClass(&starknetCompiledClass)
		if err != nil {
			return err
		}
	}

	declaredClass := core.DeclaredClass{
		At: class.At,
		Class: &core.Cairo1Class{
			Abi:             class.Class.Abi,
			AbiHash:         class.Class.AbiHash,
			EntryPoints:     class.Class.EntryPoints,
			Program:         class.Class.Program,
			ProgramHash:     class.Class.ProgramHash,
			SemanticVersion: class.Class.SemanticVersion,
			Compiled:        coreCompiledClass,
		},
	}

	value, err = encoder.Marshal(declaredClass)
	if err != nil {
		return err
	}

	return txn.Set(key, value)
}
