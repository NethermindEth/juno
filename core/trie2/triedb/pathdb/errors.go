package pathdb

import "errors"

var (
	ErrDiskLayerStale = errors.New("pathdb disk layer is stale")
	ErrMissingJournal = errors.New("journal is missing")
	ErrJournalCorrupt = errors.New("journal is corrupt")
)
