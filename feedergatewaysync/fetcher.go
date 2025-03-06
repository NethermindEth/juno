package feedergatewaysync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// FetchBlock retrieves a CustomBlock from the feeder gateway
func (s *SyncManager) FetchBlock(blockNumber uint64) (*CustomBlock, error) {
	reqURL := fmt.Sprintf("%s/feeder_gateway/get_block?blockNumber=%d", s.feederURL, blockNumber)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for block %d: %w", blockNumber, err)
	}

	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
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
