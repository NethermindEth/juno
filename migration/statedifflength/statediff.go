package statedifflength

import (
	"errors"
	"fmt"
)

// Field keys as the encoder writes them: cbor.CanonicalEncOptions keys a struct by
// its Go field names. They must stay in step with core.StateUpdate and
// core.StateDiff — TestStateDiffLengthMatchesDecoder fails if a counted field is
// renamed, added or dropped.
const (
	keyStateDiff = "StateDiff"

	keyStorageDiffs      = "StorageDiffs"
	keyNonces            = "Nonces"
	keyDeployedContracts = "DeployedContracts"
	keyDeclaredV0Classes = "DeclaredV0Classes"
	keyDeclaredV1Classes = "DeclaredV1Classes"
	keyReplacedClasses   = "ReplacedClasses"
	keyMigratedClasses   = "MigratedClasses"
)

var (
	errNoStateDiff    = errors.New("state update has no state diff")
	errNilStateUpdate = errors.New("state update is null")
	errNilStateDiff   = errors.New("state diff is null")
)

// The counted fields, in core.StateDiff declaration order, which is the order the
// decoder resolves a key in. They mirror core.StateDiff.Length() one for one.
const (
	fieldStorageDiffs = iota
	fieldNonces
	fieldDeployedContracts
	fieldDeclaredV0Classes
	fieldDeclaredV1Classes
	fieldReplacedClasses
	fieldMigratedClasses

	countedFields
)

// stateDiffCounts holds one entry count per counted core.StateDiff field.
type stateDiffCounts struct {
	entries [countedFields]uint64
	// seen marks the fields already read, so a repeated key can be ignored the way the
	// decoder ignores it. Seven fields fit in a byte.
	seen uint8
}

func (c *stateDiffCounts) sum() uint64 {
	var total uint64
	for _, count := range c.entries {
		total += count
	}
	return total
}

// stateDiffLength returns core.StateDiff.Length() for the CBOR-encoded
// core.StateUpdate in data, without decoding the state diff itself. It walks the
// encoding and reads collection sizes off their heads, so it allocates nothing and
// costs one pass over data however large the diff is.
func stateDiffLength(data []byte) (uint64, error) {
	walker := cursor{data: data}
	if walker.nextIsNull() {
		return 0, errNilStateUpdate
	}

	found := false
	var counts stateDiffCounts
	err := walker.eachMapEntry(func() error {
		key, err := walker.textString()
		if err != nil {
			return fmt.Errorf("reading state update field name: %w", err)
		}
		if !matchesFieldName(key, keyStateDiff) {
			return walker.skip()
		}
		found = true
		return counts.read(&walker)
	})
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, errNoStateDiff
	}
	return counts.sum(), nil
}

// read fills the counts from the state diff map at the cursor. A key the walker does
// not know is skipped, as an unknown field is by the decoder.
func (c *stateDiffCounts) read(walker *cursor) error {
	if walker.nextIsNull() {
		return errNilStateDiff
	}
	return walker.eachMapEntry(func() error {
		key, err := walker.textString()
		if err != nil {
			return fmt.Errorf("reading state diff field name: %w", err)
		}

		field := fieldIndex(key)
		if field < 0 {
			return walker.skip()
		}
		if c.seen&(1<<field) != 0 {
			// The decoder keeps the first occurrence of a repeated key and skips the
			// rest, so a second one must not overwrite the count.
			return walker.skip()
		}
		c.seen |= 1 << field

		if field == fieldStorageDiffs {
			c.entries[field], err = storageDiffsLength(walker)
		} else {
			c.entries[field], err = walker.entries()
		}
		if err != nil {
			return fmt.Errorf("counting %s: %w", key, err)
		}
		return nil
	})
}

// fieldIndex returns the counted field an encoded key names, or -1 if the key is not
// counted.
func fieldIndex(key []byte) int {
	switch {
	case matchesFieldName(key, keyStorageDiffs):
		return fieldStorageDiffs
	case matchesFieldName(key, keyNonces):
		return fieldNonces
	case matchesFieldName(key, keyDeployedContracts):
		return fieldDeployedContracts
	case matchesFieldName(key, keyDeclaredV0Classes):
		return fieldDeclaredV0Classes
	case matchesFieldName(key, keyDeclaredV1Classes):
		return fieldDeclaredV1Classes
	case matchesFieldName(key, keyReplacedClasses):
		return fieldReplacedClasses
	case matchesFieldName(key, keyMigratedClasses):
		return fieldMigratedClasses
	default:
		return -1
	}
}

// matchesFieldName reports whether an encoded map key resolves to the struct field
// called name, the way the decoder resolves it: an exact match, else a
// strings.EqualFold match against a name of the same byte length. Every field name
// here is ASCII, and a same-length fold match against an ASCII name can only be
// ASCII itself — any non-ASCII rune would take more bytes than the ASCII byte it
// folds to — so folding ASCII covers both. The names are also distinct under
// folding, which makes each key resolve to at most one field.
func matchesFieldName(key []byte, name string) bool {
	if len(key) != len(name) {
		return false
	}
	for i := range key {
		if lowerASCII(key[i]) != lowerASCII(name[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(letter byte) byte {
	if letter >= 'A' && letter <= 'Z' {
		return letter + ('a' - 'A')
	}
	return letter
}

// storageDiffsLength sums the sizes of the per-address storage maps, which is what
// core.StateDiff.Length() counts for the nested StorageDiffs field.
func storageDiffsLength(walker *cursor) (uint64, error) {
	var total uint64
	err := walker.eachMapEntry(func() error {
		if err := walker.skip(); err != nil { // the contract address
			return err
		}
		perAddress, err := walker.entries()
		if err != nil {
			return err
		}
		total += perAddress
		return nil
	})
	return total, err
}
