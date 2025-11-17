package registry

import (
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/trie2/triedb/pathdb"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/encoder"
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
			err := encoder.RegisterType(t)
			if err != nil {
				panic(err)
			}
		}
	})
}
