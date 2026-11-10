package registry

import (
	"reflect"
	"sync"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/core"
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

		var transactionTags core.CBORTransactionTags
		for _, t := range types {
			err := cbor.RegisterType(t)
			if err != nil {
				panic(err)
			}
			// fastcbor reads the tag that was just given, which a value of the type starts with.
			encoded, err := cbor.Marshal(reflect.New(t).Elem().Interface())
			if err != nil {
				panic(err)
			}
			tag, err := fastcbor.TagOf(encoded)
			if err != nil {
				panic(err)
			}
			fastcbor.RegisterType(t, tag)
			switch t {
			case reflect.TypeFor[core.DeclareTransaction]():
				transactionTags.DeclareTransaction = tag
			case reflect.TypeFor[core.DeployTransaction]():
				transactionTags.DeployTransaction = tag
			case reflect.TypeFor[core.InvokeTransaction]():
				transactionTags.InvokeTransaction = tag
			case reflect.TypeFor[core.L1HandlerTransaction]():
				transactionTags.L1HandlerTransaction = tag
			case reflect.TypeFor[core.DeployAccountTransaction]():
				transactionTags.DeployAccountTransaction = tag
			}
		}
		core.RegisterTypedCBORReaders()
		core.RegisterTypedTransactionReader(transactionTags)
	})
}
