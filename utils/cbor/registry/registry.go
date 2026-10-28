package registry

import (
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/ugorji"
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
			reflect.TypeFor[core.Header](),
			reflect.TypeFor[core.StateUpdate](),
			reflect.TypeFor[core.TransactionReceipt](),
			reflect.TypeFor[core.DeclareTransaction](),
			reflect.TypeFor[core.DeployTransaction](),
			reflect.TypeFor[core.InvokeTransaction](),
			reflect.TypeFor[core.L1HandlerTransaction](),
			reflect.TypeFor[core.DeployAccountTransaction](),
			reflect.TypeFor[core.SierraClass](),
		} {
			cbor.RegisterDecoder(t, ugorji.Unmarshal)
		}
		cbor.RegisterInterface(reflect.TypeFor[core.Transaction]())
		cbor.RegisterInterface(reflect.TypeFor[core.ClassDefinition]())
	})
}
