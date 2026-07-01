package walstore

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
)

type walRecordKind uint8

const (
	walRecordEntry walRecordKind = iota + 1
	walRecordPruneUpToHeight
)

type walEntryKind uint8

const (
	walEntryStart walEntryKind = iota + 1
	walEntryProposal
	walEntryPrevote
	walEntryPrecommit
	walEntryTimeout
)

type walRecordEnvelope[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	Kind           walRecordKind
	EntryKind      walEntryKind
	StartHeight    types.Height
	ProposalEntry  *wal.Proposal[V, H, A]
	PrevoteEntry   *wal.Prevote[H, A]
	PrecommitEntry *wal.Precommit[H, A]
	TimeoutEntry   *wal.Timeout
	Height         types.Height
}

func (e *walRecordEnvelope[V, H, A]) setEntry(entry wal.Entry[V, H, A]) error {
	switch entry := entry.(type) {
	case *wal.Start:
		if entry == nil {
			return errors.New("nil start WAL entry")
		}
		e.EntryKind = walEntryStart
		e.StartHeight = types.Height(*entry)
	case *wal.Proposal[V, H, A]:
		if entry == nil {
			return errors.New("nil proposal WAL entry")
		}
		e.EntryKind = walEntryProposal
		proposal := *entry
		e.ProposalEntry = &proposal
	case *wal.Prevote[H, A]:
		if entry == nil {
			return errors.New("nil prevote WAL entry")
		}
		e.EntryKind = walEntryPrevote
		prevote := *entry
		e.PrevoteEntry = &prevote
	case *wal.Precommit[H, A]:
		if entry == nil {
			return errors.New("nil precommit WAL entry")
		}
		e.EntryKind = walEntryPrecommit
		precommit := *entry
		e.PrecommitEntry = &precommit
	case *wal.Timeout:
		if entry == nil {
			return errors.New("nil timeout WAL entry")
		}
		e.EntryKind = walEntryTimeout
		timeout := *entry
		e.TimeoutEntry = &timeout
	default:
		return fmt.Errorf("unsupported WAL entry type %T", entry)
	}

	return nil
}

func (e walRecordEnvelope[V, H, A]) entry() wal.Entry[V, H, A] {
	switch e.EntryKind {
	case walEntryStart:
		start := wal.Start(e.StartHeight)
		return &start
	case walEntryProposal:
		return e.ProposalEntry
	case walEntryPrevote:
		return e.PrevoteEntry
	case walEntryPrecommit:
		return e.PrecommitEntry
	case walEntryTimeout:
		return e.TimeoutEntry
	default:
		panic(fmt.Sprintf("unknown WAL entry kind %d", e.EntryKind))
	}
}
