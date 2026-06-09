package db

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

func (e walRecordEnvelope[V, H, A]) entry() (wal.Entry[V, H, A], error) {
	switch e.EntryKind {
	case walEntryStart:
		start := wal.Start(e.StartHeight)
		return &start, nil
	case walEntryProposal:
		if e.ProposalEntry == nil {
			return nil, errors.New("missing proposal WAL entry payload")
		}
		return e.ProposalEntry, nil
	case walEntryPrevote:
		if e.PrevoteEntry == nil {
			return nil, errors.New("missing prevote WAL entry payload")
		}
		return e.PrevoteEntry, nil
	case walEntryPrecommit:
		if e.PrecommitEntry == nil {
			return nil, errors.New("missing precommit WAL entry payload")
		}
		return e.PrecommitEntry, nil
	case walEntryTimeout:
		if e.TimeoutEntry == nil {
			return nil, errors.New("missing timeout WAL entry payload")
		}
		return e.TimeoutEntry, nil
	default:
		return nil, fmt.Errorf("unknown WAL entry kind %d", e.EntryKind)
	}
}
