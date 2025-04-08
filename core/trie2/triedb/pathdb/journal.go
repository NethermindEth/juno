package pathdb

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/encoder"
)

const (
	journalVersion = 1
)

type journalType int

const (
	diffJournal journalType = iota + 1
	diskJournal
)

type DiffJournal struct {
	Root       felt.Felt
	Block      uint64
	EncNodeset []byte // encoded bytes of nodeset
}

type DiskJournal struct {
	Root       felt.Felt
	ID         uint64
	EncNodeset []byte
}

type DBJournal struct {
	// TODO(weiihann): handle this, by right we should store the state root and verify when loading
	// root felt.Felt
	Version   uint8
	EncLayers []byte // encoded bytes of layers
}

func (dl *diffLayer) journal(w io.Writer) error {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	if err := dl.parent.journal(w); err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := dl.nodes.encode(buf); err != nil {
		return err
	}

	diffJn := &DiffJournal{
		Root:       dl.root,
		Block:      dl.block,
		EncNodeset: buf.Bytes(),
	}

	enc, err := encoder.Marshal(diffJn)
	if err != nil {
		return err
	}

	// First write the journal type
	_, err = w.Write([]byte{byte(diffJournal)})
	if err != nil {
		return err
	}

	// Then write the length of the encoded journal
	var encLen []byte
	binary.BigEndian.PutUint64(encLen, uint64(len(enc)))
	_, err = w.Write(encLen)
	if err != nil {
		return err
	}

	// Finally write the encoded journal
	_, err = w.Write(enc)
	return err
}

func (dl *diskLayer) journal(w io.Writer) error {
	dl.lock.RLock()
	defer dl.lock.RUnlock()

	if dl.isStale() {
		return ErrDiskLayerStale
	}

	buf := new(bytes.Buffer)
	if err := dl.dirties.nodes.encode(buf); err != nil {
		return err
	}

	diskJn := &DiskJournal{
		Root:       dl.root,
		ID:         dl.id,
		EncNodeset: buf.Bytes(),
	}

	enc, err := encoder.Marshal(diskJn)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte{byte(diskJournal)})
	if err != nil {
		return err
	}

	var encLen []byte
	binary.BigEndian.PutUint64(encLen, uint64(len(enc)))
	_, err = w.Write(encLen)
	if err != nil {
		return err
	}

	_, err = w.Write(enc)
	return err
}

func (d *Database) Journal(root felt.Felt) error {
	l := d.tree.get(root)
	if l == nil {
		return fmt.Errorf("layer %v not found", root)
	}

	d.lock.Lock()
	defer d.lock.Unlock()

	dbJn := &DBJournal{
		Version: journalVersion,
	}

	buf := new(bytes.Buffer)
	if err := l.journal(buf); err != nil {
		return err
	}

	dbJn.EncLayers = buf.Bytes()

	enc, err := encoder.Marshal(dbJn)
	if err != nil {
		return err
	}

	return trieutils.WriteTrieJournal(d.disk, enc)
}
