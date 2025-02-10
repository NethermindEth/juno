package feedergatewaysync

// CustomTransaction represents a transaction in the block
type CustomTransaction struct {
	Type      string   `json:"type"`
	Hash      string   `json:"transaction_hash"`
	ClassHash string   `json:"class_hash,omitempty"`
	Calldata  []string `json:"calldata,omitempty"`
}

// CustomBlock represents a block fetched from Feeder Gateway
type CustomBlock struct {
	Number       uint64              `json:"block_number"`
	Hash         string              `json:"block_hash"`
	ParentHash   string              `json:"parent_block_hash"`
	Timestamp    uint64              `json:"timestamp"`
	Transactions []CustomTransaction `json:"transactions"`
}
