package db

import (
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
)

func LoadPruneWatermark(walDir string) (types.Height, error) {
	return loadPruneWatermark(walDir)
}

func WritePruneWatermark(walDir string, height types.Height) error {
	return writePruneWatermark(walDir, height)
}

// ForcePendingCleanup primes the store so the next prune flush triggers cleanup.
func ForcePendingCleanup(
	walStore TendermintWALStore[starknet.Value, starknet.Hash, starknet.Address],
) {
	walStore.(*tendermintWALStore[starknet.Value, starknet.Hash, starknet.Address]).
		pruneRecordsSinceCleanup = cleanupPruneRecordInterval - 1
}

func EncodeWALRecordPayload(entry starknet.WALEntry) ([]byte, error) {
	envelope := walRecordEnvelope[starknet.Value, starknet.Hash, starknet.Address]{
		Kind: walRecordEntry,
	}
	if err := envelope.setEntry(entry); err != nil {
		return nil, err
	}
	return appendWALRecordPayload(nil, &envelope)
}

func DecodeWALRecordPayload(payload []byte) error {
	_, err := decodeWALRecord[starknet.Value, starknet.Hash, starknet.Address](payload)
	return err
}
