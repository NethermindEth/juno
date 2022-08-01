package models

import (
	"errors"

	"github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/transaction"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"
)

// Modeler is the interface implemented by types that can retrieve the
// data used in the gateway.
type Modeler interface {
	BlockByHash(hash string) (*Block, error)
	BlockByNumber(num uint64) (*Block, error)
	BlockHashByID(num uint64) (*felt.Felt, error)
	BlockIDByHash(hash *felt.Felt) (uint64, error)
}

// Model has methods defined on it that abstract database queries.
type Model struct {
	BlockMan *block.Manager
	TxMan    *transaction.Manager
}

// TODO: The gas_price and sequencer_address fields are missing from
// this structure but may warrant inclusion.

// Header represents a block header.
type Header struct {
	// Hash is the block hash which is computed over the header's fields.
	Hash string `json:"block_hash"`
	// Number is the height of the block.
	Number uint64 `json:"block_number"`
	// Root is the state commitment after this instance's block.
	Root string `json:"state_root"`
	// Status carries the lifecycle information of the block such as
	// whether it has been accepted by the verifier on Ethereum.
	Status types.BlockStatus `json:"status"`
	// Timestamp is the time the sequencer created the block before
	// executing its transactions.
	Timestamp int64 `json:"timestamp"`

	// L1TxHash is the transaction hash of the batch submitted to the
	// verifier on Ethereum.
	L1TxHash string `json:"ethereum_tx_hash,omitempty"`
	// EventLen is the number of events in the block.
	EventLen uint64 `json:"event_count"`
	// MsgLen is the number of messages in the block.
	MsgLen uint64 `json:"message_count"`
	// TxLen is the number of transactions in the block.
	TxLen uint64 `json:"transaction_count"`
}

type some struct {
	L1TxHash *felt.Felt
	EventLen uint64
	MsgLen   uint64
	TxLen    uint64
}

// Block represents a block.
type Block struct {
	Header

	// Transactions are the transactions in the block.
	Transactions []any `json:"transactions"`

	// TODO: Add receipts when they become supported.
	// Receipts are the corresponding receipts of the transactions in the
	// block.
	// Receipts []Receipt `json:"transaction_receipts"`
}

// TODO: Add the index field when that becomes available.

// Transaction represents a StarkNet transaction.
type Transaction struct {
	// TxHash is the transaction hash.
	Hash string `json:"transaction_hash"`
	// TxIndex is the transaction index.
	Index uint64 `json:"transaction_index"`
	// Type is the transaction type.
	Type TxType `json:"type"`
}

// Declare represents a transaction used to introduce new classes in
// StarkNet which are then used by other contracts to deploy instances
// of or simply use them in library calls.
type Declare struct {
	Transaction

	// ClassHash is a reference to a contract class.
	ClassHash string `json:"class_hash"`

	// Sender is the address of the account initiating the transaction.
	Sender string `json:"sender_address"`
	// MaxFee is the maximum fee that the sender is willing to pay for the
	// transaction.
	MaxFee string `json:"max_fee"`
	// Sig is the signature of the transaction.
	Sig string `json:"signature"`
	// Nonce is the transaction nonce.
	Nonce string `json:"nonce"`
	// Ver is the transaction's version.
	Ver string `json:"version,omitempty"`
}

// Deploy represents a transaction type used to deploy contracts to
// StarkNet and set to be deprecated in future versions of the protocol.
type Deploy struct {
	Transaction

	// Caller is the deploying account contract.
	Caller string `json:"contract_address"`
	// ClassHash is a reference to the contract definition that defines
	// the contract's functionality.
	ClassHash string `json:"class_hash"`

	// Salt is a random number used to distinguish between different
	// instances of the contract.
	Salt string `json:"contract_address_salt"`
	// ConstructorCalldata are the arguments passed into the constructor
	// during contract deployment.
	ConstructorCalldata []string `json:"constructor_calldata"`
	// Ver is the transaction's version.
	Ver string `json:"version,omitempty"`
}

// Invoke represents a transaction type used to invoke contract
// functions in StarkNet.
type Invoke struct {
	Transaction

	// EntryPointType is the entry point type in the contract.
	EntryPointType string `json:"entry_point_type"`

	// Addr is the address of the contract invoked by this transaction.
	Addr string `json:"contract_address"`
	// Selector is the entry point in the contract.
	Selector string `json:"entry_point_selector"`
	// Calldata is the arguments passed to the invoked function.
	Calldata []string `json:"calldata"`
	// Sig is the signature of the transaction.
	Sig []string `json:"signature"`
	// MaxFee is the maximum fee that the sender is willing to pay for the
	// transaction.
	MaxFee string `json:"max_fee"`
	// Ver is the transaction's version.
	Ver string `json:"version,omitempty"`
}

// TxType is the transaction type.
type TxType int

const (
	DeclareTx = iota
	DeployTx
	InvokeTx
)

// MarshalJSON implements the json.Marshaler interface for TxType.
func (t TxType) MarshalJSON() ([]byte, error) {
	switch t {
	case DeclareTx:
		return []byte(`"DECLARE"`), nil
	case DeployTx:
		return []byte(`"DEPLOY"`), nil
	case InvokeTx:
		return []byte(`"INVOKE_FUNCTION"`), nil
	}
	return []byte(`"UNKNOWN"`), nil
}

/*
// Receipt represents a transaction receipt.
type Receipt struct {
	// TxIndex is the transaction index.
	TxIndex uint64 `json:"transaction_index"`
	// TxHash is the transaction hash.
	TxHash *felt.Felt `json:"transaction_hash"`
	// ConsumedMsg is the message from Ethereum consumed in the
	// transaction.
	ConsumedMsg Received `json:"l1_to_l2_consumed_message"`
	// L2ToL1Msgs contains the messages sent to Ethereum.
	L2ToL1Msgs []Sent `json:"l2_to_l1_messages"`
	// Events are events emitted by the contract during execution.
	Events []struct {
		// From is the address emitting the event(s).
		From *felt.Felt `json:"from_address"`
		// Keys are the keys used to index the events.
		Keys []*felt.Felt `json:"keys"`
		// Data is event data.
		Data []*felt.Felt `json:"data"`
	} `json:"events"`
	// Exe are the computational resources used in the transaction.
	Exe struct {
		Steps   uint64 `json:"n_steps"`
		Counter struct {
			Pedersen   uint64 `json:"n_steps"`
			RangeCheck uint64 `json:"range_check_builtin"`
			Output     uint64 `json:"output_builtin"`
			ECDSA      uint64 `json:"ecdsa_builtin"`
			Bitwise    uint64 `json:"bitwise_builtin"`
		} `json:"builtin_instance_counter"`
		MemoryHoles uint64 `json:"n_memory_holes"`
	} `json:"execution_resources"`
	// ActualFee is the fee charged in the transaction.
	ActualFee *felt.Felt `json:"actual_fee"`
}

// TransactionStatus holds information about the lifecycle of the
// transaction along with the hash of the block it was made.
type TransactionStatus struct {
	// Status is the transaction status which is an enumeration of
	// strings.
	Status string `json:"tx_status"`
	// BlockHash is the hash of the block the transaction was made.
	BlockHash string `json:"block_hash"`
}

// Received represents an Ethereum to StarkNet message.
type Received struct {
	// From is the Ethereum address the message comes from.
	From [20]byte `json:"from_address"`
	// To is the StarkNet address the message is sent to.
	To *felt.Felt `json:"to_address"`
	// Selector is the entry point in the contract.
	Selector *felt.Felt `json:"selector"`
	// Payload is the payload data.
	Payload []*felt.Felt `json:"payload"`
	// Nonce is the transaction nonce.
	Nonce *felt.Felt `json:"nonce"`
}

// Sent represents a StarkNet to Ethereum message.
type Sent struct {
	// TODO: Confirm this structure and add JSON fields.

	// To is the Ethereum address the message comes from.
	To [20]byte
	// Payload is the payload data.
	Payload []*felt.Felt

	// From is the StarkNet address the message is sent from.
	// From *felt.Felt
	// Selector is the entry point in the contract.
	// Selector *felt.Felt
	// Nonce is the transaction nonce.
	// Nonce *felt.Felt
}
*/

// ErrNotFound indicates a record that was not found from the database.
var ErrNotFound = errors.New("models: record not found")

// newBlock creates a Block from the types.Block header.
func (m *Model) newBlock(header *types.Block) (*Block, error) {
	const prefix = "0x"

	block := &Block{
		Header: Header{
			Hash:      prefix + header.BlockHash.Hex(),
			Number:    header.BlockNumber,
			Root:      prefix + header.NewRoot.Hex(),
			Status:    header.Status,
			Timestamp: header.TimeStamp,
			L1TxHash:  prefix + header.TxCommitment.Hex(),
			EventLen:  header.EventCount,
			MsgLen:    header.EventCount,
			TxLen:     header.TxCount,
		},

		Transactions: make([]any, 0, len(header.TxHashes)),
		// TODO: Receipts are currently not stored at the moment. See
		// comment below.
		// Receipts: make([]any, 0, len(header.TxHashes)),
	}

	for _, hash := range header.TxHashes {
		gen, err := m.TxMan.GetTransaction(hash)
		if err != nil {
			return nil, err
		}

		var tx any
		switch cast := gen.(type) {
		// TODO: Add case for declare transaction.
		case *types.TransactionDeploy:
			// TODO: The following fields are missing.
			// 	- class_hash.
			// 	- contract_address_salt.
			// 	- version.
			tx = &Deploy{
				Transaction: Transaction{
					Hash: prefix + cast.Hash.Hex(),
					Type: DeployTx,
				},
				Caller:              prefix + cast.ContractAddress.Hex(),
				ConstructorCalldata: Strings(cast.ConstructorCallData),
			}
		case *types.TransactionInvoke:
			// TODO: The following fields are missing.
			// 	- entry_point_type.
			// 	- version.
			tx = &Invoke{
				Transaction: Transaction{
					Hash: prefix + cast.Hash.Hex(),
					Type: InvokeTx,
				},
				Addr:     prefix + cast.ContractAddress.Hex(),
				Selector: prefix + cast.EntryPointSelector.Hex(),
				Calldata: Strings(cast.CallData),
				Sig:      Strings(cast.Signature),
				MaxFee:   prefix + cast.MaxFee.Hex(),
			}
		}

		block.Transactions = append(block.Transactions, tx)

		// TODO: Transaction receipts are not stored in the database right
		// now.
		/*
			cast, err := txMan.GetReceipt(hash)
			if err != nil {
				return nil, err
			}

			// TODO: The following fields are missing.
			// 	- transaction_index.
			//	- In l1_to_l2_consumed_message:
			//		- to_address.
			//		- selector.
			//		- nonce.
			// 	- execution_resources.
			receipt := &Receipt{
				TxHash: cast.TxHash,
				ConsumedMsg: Received{
					From:    cast.L1OriginMessage.FromAddress,
					Payload: cast.L1OriginMessage.Payload,
				},
				ActualFee: cast.ActualFee,
			}

			// TODO: See comment in Sent struct and investigate missing fields.
			receipt.L2ToL1Msgs = make([]Sent, 0, len(cast.MessagesSent))
			for _, msg := range cast.MessagesSent {
				receipt.L2ToL1Msgs = append(
					receipt.L2ToL1Msgs,
					Sent{To: msg.ToAddress, Payload: msg.Payload},
				)
			}

			receipt.Events = make([]Event, 0, len(cast.Events))
			for _, event := range cast.Events {
				receipt.Events = append(
					receipt.Events,
					Event{From: event.FromAddress, Keys: event.Keys, Data: event.Data},
				)
			}

			block.Receipts = append(block.Receipts, receipt)
		*/
	}
	return block, nil
}

// BlockByHash returns a Block corresponding to the hash given. The
// function is agnostic to whether the string has a "0x" prefix.
func (m *Model) BlockByHash(hash string) (*Block, error) {
	header, err := m.BlockMan.GetBlockByHash(new(felt.Felt).SetHex(hash))
	if err != nil {
		return nil, ErrNotFound
	}
	return m.newBlock(header)
}

// BlockByNumber returns a Block corresponding to the height num.
func (m *Model) BlockByNumber(num uint64) (*Block, error) {
	header, err := m.BlockMan.GetBlockByNumber(num)
	if err != nil {
		return nil, ErrNotFound
	}
	return m.newBlock(header)
}

// BlockHashByID returns the hash of the block corresponding to the
// given block number.
func (m *Model) BlockHashByID(num uint64) (*felt.Felt, error) {
	block, err := m.BlockMan.GetBlockByNumber(num)
	if err != nil {
		return nil, ErrNotFound
	}
	return block.BlockHash, nil
}

// BlockIDByHash returns the block number of the block with the given
// hash.
func (m *Model) BlockIDByHash(hash *felt.Felt) (uint64, error) {
	block, err := m.BlockMan.GetBlockByHash(hash)
	if err != nil {
		return 0, ErrNotFound
	}
	return block.BlockNumber, nil
}
