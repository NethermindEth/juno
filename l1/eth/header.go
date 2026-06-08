package eth

// Header is the trimmed-down view of an Ethereum block header that juno
// needs. Only Number is read in production (l1/eth_subscriber.go).
// The HexU64 field type lets eth_getBlockByNumber responses unmarshal
// directly without any per-struct codec boilerplate.
type Header struct {
	Number HexU64 `json:"number"`
}
