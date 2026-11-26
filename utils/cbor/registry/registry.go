package registry

import (
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fastcbor"
)

var once sync.Once

//nolint:gochecknoinits
func init() {
	once.Do(func() {
		types := []reflect.Type{
			reflect.TypeOf(core.DeclareTransaction{}),
			reflect.TypeOf(core.DeployTransaction{}),
			reflect.TypeOf(core.InvokeTransaction{}),
			reflect.TypeOf(core.L1HandlerTransaction{}),
			reflect.TypeOf(core.DeployAccountTransaction{}),
			reflect.TypeOf(core.DeprecatedCairoClass{}),
			reflect.TypeOf(core.SierraClass{}),
			reflect.TypeOf(trienode.DeletedNode{}),
			reflect.TypeOf(trienode.LeafNode{}),
			reflect.TypeOf(trienode.NonLeafNode{}),
			reflect.TypeOf(pathdb.JournalNodeSet{}),
			reflect.TypeOf(pathdb.DiffJournal{}),
			reflect.TypeOf(pathdb.DiskJournal{}),
			reflect.TypeOf(pathdb.DBJournal{}),
			// Consensus WAL types
			reflect.TypeOf(starknet.WALProposal{}),
			reflect.TypeOf(starknet.WALPrevote{}),
			reflect.TypeOf(starknet.WALPrecommit{}),
			reflect.TypeOf(starknet.WALTimeout{}),
		}

		for _, t := range types {
			err := cbor.RegisterType(t)
			if err != nil {
				panic(err)
			}
		}

		for _, t := range []reflect.Type{
			reflect.TypeFor[felt.Felt](),
			reflect.TypeFor[felt.Address](),
			reflect.TypeFor[felt.Hash](),
			reflect.TypeFor[felt.ClassHash](),
			reflect.TypeFor[felt.SierraClassHash](),
			reflect.TypeFor[felt.CasmClassHash](),
			reflect.TypeFor[felt.TransactionHash](),
			reflect.TypeFor[felt.StateRootHash](),
			reflect.TypeFor[felt.Slice[felt.Felt]](),
			reflect.TypeFor[core.TransactionVersion](),
		} {
			if err := fastcbor.RegisterFelt(t); err != nil {
				panic(err)
			}
		}
		fastcbor.RegisterFeltMap[felt.Felt, felt.Felt]()
		fastcbor.RegisterFeltStorageMap[felt.Felt, felt.Felt]()
		fastcbor.RegisterFeltValueMap[felt.SierraClassHash, felt.CasmClassHash]()
	})
}
