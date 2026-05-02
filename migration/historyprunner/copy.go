package historyprunner

import (
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/db"
)

// copyScratch holds the reusable byte buffers for one worker's copy
// operations. Each worker owns one copyScratch; buffers are overwritten on
// every entry and consumed (via batch.Put) before the next iteration
// touches them, so reuse is safe. Allocated once at stager/restorer
// construction.
type copyScratch struct {
	historyKey [historyKeyBufLen]byte
	scratchKey [scratchKeyBufLen]byte
	value      [historyValueLen]byte
}

type copyDirection bool

const (
	historyToScratch copyDirection = false
	scratchToHistory copyDirection = true
)

func copyStateHistory(
	reader db.KeyValueReader,
	batch db.Batch,
	scratch *copyScratch,
	diff *core.StateDiff,
	blockNum uint64,
	direction copyDirection,
) error {
	var blockBE [blockNumberSuffixLen]byte
	binary.BigEndian.PutUint64(blockBE[:], blockNum)

	move := func(historyKey, scratchKey []byte) error {
		src, dst := historyKey, scratchKey
		if direction == scratchToHistory {
			src, dst = dst, src
		}
		return copyValue(reader, batch, scratch.value[:], src, dst)
	}

	for addr, slots := range diff.StorageDiffs {
		for slot := range slots {
			if err := move(
				fillStorageHistoryKey(scratch.historyKey[:], &addr, &slot, blockBE),
				fillStorageScratchKey(scratch.scratchKey[:], &addr, &slot, blockBE),
			); err != nil {
				return err
			}
		}
	}

	for addr := range diff.Nonces {
		if err := move(
			fillNonceHistoryKey(scratch.historyKey[:], &addr, blockBE),
			fillNonceScratchKey(scratch.scratchKey[:], &addr, blockBE),
		); err != nil {
			return err
		}
	}

	for addr := range diff.ReplacedClasses {
		if err := move(
			fillClassHashHistoryKey(scratch.historyKey[:], &addr, blockBE),
			fillClassHashScratchKey(scratch.scratchKey[:], &addr, blockBE),
		); err != nil {
			return err
		}
	}

	return nil
}

// copyValue reads source into buf, then writes buf to dest. buf is the
// caller's reusable felt-sized scratch (32 bytes); after Put returns, the
// underlying batch has copied buf, so the caller may overwrite it safely.
func copyValue(
	reader db.KeyValueReader,
	writer db.KeyValueWriter,
	buf,
	source,
	dest []byte,
) error {
	err := reader.Get(source, func(data []byte) error {
		if len(data) != len(buf) {
			return fmt.Errorf(
				"history value size mismatch at key %x: want %d bytes, got %d",
				source, len(buf), len(data),
			)
		}

		copy(buf, data)
		return nil
	})
	if err != nil {
		return err
	}

	return writer.Put(dest, buf)
}
