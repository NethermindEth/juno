package pebblev2

import (
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

// There are currently two possible cases: an old database created with version 1
// (FormatMostCompatible) or a new database created with version 13 (FormatFlushableIngest).
// Because both versions are supported by pebble v1, we use pebble v1 to peek at the database and
// then check the version. If the version is older than version 13, we upgrade the database to
// version 13 by opening it with pebble v1 using version 13, and then closing it.
func upgradeFormatIfNeeded(path string) {
	desc, err := pebble.Peek(path, vfs.Default)
	if err != nil {
		return
	}

	if desc.FormatMajorVersion >= pebble.FormatFlushableIngest {
		return
	}

	database, err := pebble.Open(path, &pebble.Options{
		FormatMajorVersion: pebble.FormatFlushableIngest,
	})
	if err != nil {
		return
	}
	database.Close()
}
