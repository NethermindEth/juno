package feedergatewaysync

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// FetchBlock retrieves a CustomBlock from the feeder gateway
func (s *SyncManager) FetchBlock(blockNumber uint64) (*CustomBlock, error) {
	url := fmt.Sprintf("%s/feeder_gateway/get_block?blockNumber=%d", s.feederURL, blockNumber)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block %d: %w", blockNumber, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch block %d: HTTP %d", blockNumber, resp.StatusCode)
	}

	var block CustomBlock
	if err := json.NewDecoder(resp.Body).Decode(&block); err != nil {
		return nil, fmt.Errorf("failed to decode block %d: %w", blockNumber, err)
	}

	return &block, nil
}
