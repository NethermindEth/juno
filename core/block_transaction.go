package core

type BlockTransactions struct {
	Transactions []Transaction         `cbor:"1,keyasint,omitempty"`
	Receipts     []*TransactionReceipt `cbor:"2,keyasint,omitempty"`
}
