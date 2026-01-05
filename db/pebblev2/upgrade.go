package pebblev2

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	pebblev2 "github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/vfs"
)

const (
	TargetV1Version         = pebble.FormatFlushableIngest
	TargetUpgradedV1Version = pebblev2.FormatMajorVersion(TargetV1Version)
	TargetNewV2Version      = pebblev2.FormatFlushableIngest
)

// There are currently two possible cases: an old database created with version 1
// (FormatMostCompatible) or a new database created with version 13 (FormatFlushableIngest).
// Because both versions are supported by pebble v1, we use pebble v1 to peek at the database and
// then check the version. If the version is older than version 13, we upgrade the database to
// version 13 by opening it with pebble v1 using version 13, and then closing it.
func upgradeFormatIfNeeded(path string) (pebblev2.FormatMajorVersion, error) {
	desc, err := pebble.Peek(path, vfs.Default)
	if isNotV1 := err != nil || !desc.Exists; isNotV1 {
		return TargetNewV2Version, nil
	}

	if isV2Supported := desc.FormatMajorVersion >= TargetV1Version; isV2Supported {
		v2Format := pebblev2.FormatMajorVersion(desc.FormatMajorVersion)
		if v2Format < pebblev2.FormatMinSupported {
			return 0, fmt.Errorf("unknown pebble db older format %v", v2Format)
		}
		if v2Format > pebblev2.FormatNewest {
			return 0, fmt.Errorf("unknown pebble db newer format %v", v2Format)
		}
		return v2Format, nil
	}

	database, err := pebble.Open(path, &pebble.Options{
		FormatMajorVersion: TargetV1Version,
	})
	if err != nil {
		return 0, err
	}
	defer database.Close()

	return TargetUpgradedV1Version, nil
}
