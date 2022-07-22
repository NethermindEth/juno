package gateway

import (
	"errors"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"
)

// Block represents a block.
type Block struct {
	// Hash is the block hash which is computed over the header's fields.
	Hash *felt.Felt `json:"block_hash"`
	// Parent is the block hash of the block's parent instance.
	Parent *felt.Felt `json:"parent_hash"`
	// Number is the height of the block.
	Number uint64 `json:"block_number"`
	// Root is the state commitment after this instance's block.
	Root *felt.Felt `json:"state_root"`
	// Status carries the lifecycle information of the block such as
	// whether it has been accepted by the verifier on Ethereum.
	Status types.BlockStatus `json:"status"`
	// Gas is the gas price.
	Gas *felt.Felt `json:"gas_price"`
	// Timestamp is the time the sequencer created the block before
	// executing its transactions.
	Timestamp int64 `json:"timestamp"`
	// Sequencer is the address of the sequencer that created the block.
	Sequencer *felt.Felt `json:"sequencer_address,omitempty"`
	// Transactions are the transactions in the block.
	Transactions []any `json:"transactions"`
	// Receipts are the corresponding receipts of the transactions in the
	// block.
	Receipts []Receipt `json:"transaction_receipts"`
}

// Declare represents a transaction used to introduce new classes in
// StarkNet which are then used by other contracts to deploy instances
// of or simply use them in library calls.
type Declare struct {
	// TxHash is the transaction hash.
	TxHash *felt.Felt `json:"transaction_hash"`

	// ClassHash is a reference to a contract class.
	ClassHash *felt.Felt `json:"class_hash"`

	/*
		// Class is the class object.
		Class any `json:"contract_class"`
	*/
	// Sender is the address of the account initiating the transaction.
	Sender *felt.Felt `json:"sender_address"`
	// MaxFee is the maximum fee that the sender is willing to pay for the
	// transaction.
	MaxFee *felt.Felt `json:"max_fee"`
	// Sig is the signature of the transaction.
	Sig []*felt.Felt `json:"signature"`
	// Nonce is the transaction nonce.
	Nonce *felt.Felt
	// Ver is the transaction's version.
	Ver *felt.Felt `json:"version"`
}

// Deploy represents a transaction type used to deploy contracts to
// StarkNet and set to be deprecated in future versions of the protocol.
type Deploy struct {
	// TxHash is the transaction hash.
	TxHash *felt.Felt `json:"transaction_hash"`

	// XXX: Could this be the caller address below?
	Addr *felt.Felt `json:"contract_address"`
	// XXX: I am assuming this is to serve as a key to query the contract
	// definition below.
	Class *felt.Felt `json:"class_hash"`

	// Salt is a random number used to distinguish between different
	// instances of the contract.
	Salt *felt.Felt `json:"contract_address_salt"`
	// ConstructorCalldata are the arguments passed into the constructor
	// during contract deployment.
	ConstructorCalldata []*felt.Felt `json:"constructor_calldata"`
	/*
		// Caller is the deploying account contract.
		Caller *felt.Felt `json:"caller_address"`
		// Definition defines the contract's functionality.
		Definition any `json:"contract_definition"`
	*/
	// Ver is the transaction's version.
	Ver *felt.Felt `json:"version"`
}

// Invoke represents a transaction type used to invoke contract
// functions in StarkNet.
type Invoke struct {
	// TxHash is the transaction hash.
	TxHash *felt.Felt `json:"transaction_hash"`

	EntryPointType string `json:"entry_point_type"`

	// Addr is the address of the contract invoked by this transaction.
	Addr *felt.Felt `json:"contract_address"`
	// Selector is the entry point in the contract.
	Selector *felt.Felt `json:"entry_point_selector"`
	// Calldata is the arguments passed to the invoked function.
	Calldata []*felt.Felt `json:"calldata"`
	// Sig is the signature of the transaction.
	Sig []*felt.Felt `json:"signature"`
	// MaxFee is the maximum fee that the sender is willing to pay for the
	// transaction.
	MaxFee *felt.Felt `json:"max_fee"`
	// Ver is the transaction's version.
	Ver *felt.Felt `json:"version"`
}

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

	/*
		// From is the StarkNet address the message is sent from.
		From *felt.Felt
		// Selector is the entry point in the contract.
		Selector *felt.Felt
		// Nonce is the transaction nonce.
		Nonce *felt.Felt
	*/
}

// ErrNotFound indicates a record that was not found from the database.
var ErrNotFound = errors.New("models: record not found")

// newBlock creates a Block from the types.Block header.
func newBlock(header *types.Block) (*Block, error) {
	// TODO: types.Block is missing a gas_price field.
	block := &Block{
		Hash:         header.BlockHash,
		Parent:       header.ParentHash,
		Number:       header.BlockNumber,
		Root:         header.NewRoot,
		Status:       header.Status,
		Timestamp:    header.TimeStamp,
		Sequencer:    header.Sequencer,
		Transactions: make([]any, 0, len(header.TxHashes)),

		// TODO: Receipts are currently not stored at the moment. See
		// comment below.
		// Receipts: make([]any, 0, len(header.TxHashes)),
	}

	for _, hash := range header.TxHashes {
		gen, err := txMan.GetTransaction(hash)
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
				TxHash:              cast.Hash,
				Addr:                cast.ContractAddress,
				ConstructorCalldata: cast.ConstructorCallData,
			}
		case *types.TransactionInvoke:
			// TODO: The following fields are missing.
			// 	- entry_point_type.
			// 	- version.
			tx = &Invoke{
				TxHash:   cast.Hash,
				Addr:     cast.ContractAddress,
				Selector: cast.EntryPointSelector,
				Calldata: cast.CallData,
				Sig:      cast.Signature,
				MaxFee:   cast.MaxFee,
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
func BlockByHash(hash string) (*Block, error) {
	header, err := blockMan.GetBlockByHash(new(felt.Felt).SetHex(hash))
	if err != nil {
		return nil, ErrNotFound
	}
	return newBlock(header)
}

// BlockByNumber returns a Block corresponding to the height num.
func BlockByNumber(num uint64) (*Block, error) {
	header, err := blockMan.GetBlockByNumber(num)
	if err != nil {
		return nil, ErrNotFound
	}
	return newBlock(header)
}

// BlockHashByID returns the hash of the block corresponding to the
// given block number.
func BlockHashByID(num uint64) (*felt.Felt, error) {
	block, err := blockMan.GetBlockByNumber(num)
	if err != nil {
		return nil, ErrNotFound
	}
	return block.BlockHash, nil
}

// BlockIDByHash returns the block number of the block with the given
// hash.
func BlockIDByHash(hash *felt.Felt) (uint64, error) {
	block, err := blockMan.GetBlockByHash(hash)
	if err != nil {
		return 0, ErrNotFound
	}
	return block.BlockNumber, nil
}

// TODO: Implement the json.Marshaler interface because the new field
// elements defaults to a base 10 representation.
