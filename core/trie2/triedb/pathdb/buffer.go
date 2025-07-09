package pathdb

import (
	"fmt"
	"os"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

// Stores the pending nodes in memory to be committed later
type buffer struct {
	layers uint64 // number of layers merged into the buffer
	limit  uint64 // maximum number of nodes the buffer can hold
	nodes  *nodeSet
}

func newBuffer(limit int, nodes *nodeSet, layer uint64) *buffer {
	if nodes == nil {
		nodes = newNodeSet(nil, nil, nil)
	}
	return &buffer{
		limit:  uint64(limit),
		nodes:  nodes,
		layers: layer,
	}
}

func (b *buffer) node(owner *felt.Felt, path *trieutils.Path, isClass bool) (trienode.TrieNode, bool) {
	return b.nodes.node(owner, path, isClass)
}

func (b *buffer) commit(nodes *nodeSet) *buffer {
	b.layers++
	b.nodes.merge(nodes)
	return b
}

func (b *buffer) reset() {
	b.layers = 0
	b.limit = 0
	b.nodes.reset()
}

func (b *buffer) isFull() bool {
	return b.nodes.size > b.limit
}

func (b *buffer) flush(kvs db.KeyValueStore, cleans *cleanCache, id uint64) error {
	logEntry := fmt.Sprintf("FLUSH_START: state_id=%d buffer_layers=%d buffer_size=%d\n", id, b.layers, b.nodes.size)
	if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		f.WriteString(logEntry)
		f.Close()
	}

	latestPersistedID, err := trieutils.ReadPersistedStateID(kvs)
	if err != nil {
		logEntry := fmt.Sprintf("FLUSH_ERROR: Failed to read persisted state ID: %v\n", err)
		if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString(logEntry)
			f.Close()
		}
		return fmt.Errorf("failed to read persisted state ID: %w", err)
	}

	if latestPersistedID+b.layers != id {
		logEntry := fmt.Sprintf("FLUSH_ERROR: State ID mismatch - latest_persisted=%d buffer_layers=%d target_id=%d\n", latestPersistedID, b.layers, id)
		if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString(logEntry)
			f.Close()
		}
		return fmt.Errorf(
			"mismatch buffer layers applied: latest state id (%d) + buffer layers (%d) != target state id (%d)",
			latestPersistedID,
			b.layers,
			id,
		)
	}

	// Log database size calculation
	dbSize := b.nodes.dbSize()
	logEntry = fmt.Sprintf("FLUSH_INFO: Database size calculation - dbSize=%d nodeCount=%d totalSize=%d\n", dbSize, b.nodes.dbSize(), b.nodes.size)
	if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		f.WriteString(logEntry)
		f.Close()
	}

	batch := kvs.NewBatchWithSize(dbSize)
	if batch == nil {
		logEntry := fmt.Sprintf("FLUSH_ERROR: Failed to create batch with size %d\n", dbSize)
		if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString(logEntry)
			f.Close()
		}
		return fmt.Errorf("failed to create batch")
	}

	// Log node counts
	classNodeCount := len(b.nodes.classNodes)
	contractNodeCount := len(b.nodes.contractNodes)
	storageNodeCount := 0
	for _, nodes := range b.nodes.contractStorageNodes {
		storageNodeCount += len(nodes)
	}
	logEntry = fmt.Sprintf("FLUSH_INFO: Node counts - class=%d contract=%d storage=%d total=%d\n",
		classNodeCount, contractNodeCount, storageNodeCount, classNodeCount+contractNodeCount+storageNodeCount)
	if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		f.WriteString(logEntry)
		f.Close()
	}

	if err := b.nodes.write(batch, cleans); err != nil {
		logEntry := fmt.Sprintf("FLUSH_ERROR: Failed to write nodes to batch: %v\n", err)
		if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString(logEntry)
			f.Close()
		}
		return err
	}

	if err := trieutils.WritePersistedStateID(batch, id); err != nil {
		logEntry := fmt.Sprintf("FLUSH_ERROR: Failed to write persisted state ID %d: %v\n", id, err)
		if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString(logEntry)
			f.Close()
		}
		return err
	}

	if err := batch.Write(); err != nil {
		logEntry := fmt.Sprintf("FLUSH_ERROR: Failed to commit batch: %v\n", err)
		if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString(logEntry)
			f.Close()
		}
		return err
	}

	// Log successful flush
	logEntry = fmt.Sprintf("FLUSH_SUCCESS: state_id=%d nodes_written=%d total_size=%d\n",
		id, classNodeCount+contractNodeCount+storageNodeCount, b.nodes.size)
	if f, err := os.OpenFile("flush_logs.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		f.WriteString(logEntry)
		f.Close()
	}

	b.reset()
	return nil
}
