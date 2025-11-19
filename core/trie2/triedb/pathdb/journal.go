package pathdb

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

const (
	journalVersion = 1
)

var stateVersion = new(felt.Felt).SetBytes([]byte(`STARKNET_STATE_V0`))

type journalType byte

const (
	diffJournal journalType = iota + 1
	diskJournal
)

// DiffJournal represents a single state transition layer containing changes to the trie.
// It stores the new state root, block number, and the encoded set of modified nodes.
type DiffJournal struct {
	Root       felt.Felt
	Block      uint64
	EncNodeset []byte // encoded bytes of nodeset
}

// DiskJournal represents a persisted state of the trie.
// It contains the state root, state ID, and the encoded set of all nodes at this state.
type DiskJournal struct {
	Root       felt.Felt
	ID         uint64
	EncNodeset []byte
}

// DBJournal represents the entire journal of the database.
// It contains the version of the journal format and the encoded sequence of layers
// that make up the state history.
type DBJournal struct {
	// TODO(weiihann): handle this, by right we should store the state root and verify when loading
	// root felt.Felt
	Version   uint8
	EncLayers []byte // encoded bytes of layers
}

const encSize = 8

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
	encLen := make([]byte, encSize)
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

	encLen := make([]byte, encSize)
	binary.BigEndian.PutUint64(encLen, uint64(len(enc)))
	_, err = w.Write(encLen)
	if err != nil {
		return err
	}

	_, err = w.Write(enc)
	return err
}

func (d *Database) Journal(root *felt.Felt) error {
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

func (d *Database) loadJournal() (layer, error) {
	enc, err := trieutils.ReadTrieJournal(d.disk)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return nil, err
		}

		latestID, err := trieutils.ReadPersistedStateID(d.disk)
		if err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return nil, err
			}
		}

		diskRoot := d.getStateRoot()
		disk := newDiskLayer(&diskRoot, latestID, d, nil, newBuffer(d.config.WriteBufferSize, nil, 0))
		return disk, nil
	}

	var journal DBJournal
	if err := encoder.Unmarshal(enc, &journal); err != nil {
		return nil, err
	}

	if journal.Version != journalVersion {
		return nil, fmt.Errorf("unsupported journal version: %d", journal.Version)
	}

	head, err := d.loadLayers(journal.EncLayers)
	if err != nil {
		return nil, err
	}

	return head, nil
}

func (d *Database) loadLayers(enc []byte) (layer, error) {
	const layerMetadataSize = 9

	var head layer
	var parent layer
	for len(enc) > 0 {
		if len(enc) < layerMetadataSize {
			return nil, ErrJournalCorrupt
		}
		layerType := journalType(enc[0])
		encLen := binary.BigEndian.Uint64(enc[1:layerMetadataSize])

		if len(enc) < int(encLen) {
			return nil, ErrJournalCorrupt
		}

		encLayer := enc[layerMetadataSize : layerMetadataSize+encLen]
		switch layerType {
		case diffJournal:
			var diffJn DiffJournal
			if err := encoder.Unmarshal(encLayer, &diffJn); err != nil {
				return nil, err
			}
			nodes := new(nodeSet)
			if err := nodes.decode(diffJn.EncNodeset); err != nil {
				return nil, err
			}
			head = &diffLayer{
				root:   diffJn.Root,
				block:  diffJn.Block,
				nodes:  nodes,
				parent: parent,
				id:     parent.stateID() + 1,
			}
			parent = head
		case diskJournal:
			var diskJn DiskJournal
			if err := encoder.Unmarshal(encLayer, &diskJn); err != nil {
				return nil, err
			}

			latestPersistedID, err := trieutils.ReadPersistedStateID(d.disk)
			if err != nil {
				if !errors.Is(err, db.ErrKeyNotFound) {
					return nil, err
				}
			}
			nodes := new(nodeSet)
			if err := nodes.decode(diskJn.EncNodeset); err != nil {
				return nil, err
			}
			head = newDiskLayer(
				&diskJn.Root,
				diskJn.ID,
				d,
				nil,
				newBuffer(d.config.WriteBufferSize, nodes, diskJn.ID-latestPersistedID),
			)
			parent = head
		default:
			return nil, fmt.Errorf("unknown journal type: %d", layerType)
		}
		enc = enc[layerMetadataSize+encLen:]
	}

	return head, nil
}

func (d *Database) getStateRoot() felt.Felt {
	encContractRoot, err := trieutils.GetNodeByPath(
		d.disk,
		db.ContractTrieContract,
		&felt.Zero,
		&trieutils.Path{},
		false,
	)
	if err != nil {
		return felt.Zero
	}

	encStorageRoot, err := trieutils.GetNodeByPath(

		d.disk,
		db.ClassTrie,
		&felt.Zero,
		&trieutils.Path{},
		false,
	)
	if err != nil {
		return felt.Zero
	}

	contractRootNode, err := trienode.DecodeNode(encContractRoot, &felt.Zero, 0, contractClassTrieHeight)
	if err != nil {
		return felt.Zero
	}
	contractRootHash := contractRootNode.Hash(crypto.Pedersen)

	classRootNode, err := trienode.DecodeNode(encStorageRoot, &felt.Zero, 0, contractClassTrieHeight)
	if err != nil {
		return felt.Zero
	}
	classRootHash := classRootNode.Hash(crypto.Pedersen)

	return crypto.PoseidonArray(stateVersion, &contractRootHash, &classRootHash)
}
