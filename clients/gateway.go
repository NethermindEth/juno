package clients

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"strconv"

	"github.com/NethermindEth/juno/core/felt"
)

const feederGatewayPath = "/feeder_gateway/"

type GatewayClient struct {
	baseUrl string
}

func NewGatewayClient(baseUrl string) *GatewayClient {
	return &GatewayClient{
		baseUrl: baseUrl + feederGatewayPath,
	}
}

// `buildQueryString` builds the query url with encoded parameters
func (c *GatewayClient) buildQueryString(endpoint string, args map[string]string) string {
	base, err := url.Parse(c.baseUrl)
	if err != nil {
		panic("Malformed feeder gateway base URL")
	}

	base.Path += endpoint

	params := url.Values{}
	for k, v := range args {
		params.Add(k, v)
	}
	base.RawQuery = params.Encode()

	return base.String()
}

// get performs a "GET" http request with the given URL and returns the response body
func (c *GatewayClient) get(queryUrl string) ([]byte, error) {
	res, err := http.Get(queryUrl)
	if err != nil {
		return nil, err
	} else if res.StatusCode != http.StatusOK {
		return nil, errors.New(res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	return body, err
}

// StateUpdate object returned by the gateway in JSON format for "get_state_update" endpoint
type StateUpdate struct {
	BlockHash *felt.Felt `json:"block_hash"`
	NewRoot   *felt.Felt `json:"new_root"`
	OldRoot   *felt.Felt `json:"old_root"`

	StateDiff struct {
		StorageDiffs map[string][]struct {
			Key   *felt.Felt `json:"key"`
			Value *felt.Felt `json:"value"`
		} `json:"storage_diffs"`

		Nonces            map[string]*felt.Felt `json:"nonces"`
		DeployedContracts []struct {
			Address   *felt.Felt `json:"address"`
			ClassHash *felt.Felt `json:"class_hash"`
		} `json:"deployed_contracts"`
		DeclaredContracts []*felt.Felt `json:"declared_contracts"`
	} `json:"state_diff"`
}

func (c *GatewayClient) GetStateUpdate(blockNumber uint64) (*StateUpdate, error) {
	queryUrl := c.buildQueryString("get_state_update", map[string]string{
		"blockNumber": strconv.FormatUint(blockNumber, 10),
	})

	if body, err := c.get(queryUrl); err != nil {
		return nil, err
	} else {
		update := new(StateUpdate)
		if err = json.Unmarshal(body, update); err != nil {
			return nil, err
		}
		return update, nil
	}
}

// Transaction object returned by the gateway in JSON format for multiple endpoints
type Transaction struct {
	Hash                *felt.Felt   `json:"transaction_hash"`
	Version             *felt.Felt   `json:"version"`
	ContractAddress     *felt.Felt   `json:"contract_address"`
	ContractAddressSalt *felt.Felt   `json:"contract_address_salt"`
	ClassHash           *felt.Felt   `json:"class_hash"`
	ConstructorCalldata []*felt.Felt `json:"constructor_calldata"`
	Type                string       `json:"type"`
	// invoke
	MaxFee             *felt.Felt   `json:"max_fee"`
	Signature          []*felt.Felt `json:"signature"`
	Calldata           []*felt.Felt `json:"calldata"`
	EntryPointSelector *felt.Felt   `json:"entry_point_selector"`
	// declare/deploy_account
	Nonce *felt.Felt `json:"nonce"`
	// declare
	SenderAddress *felt.Felt `json:"sender_address"`
}

type TransactionStatus struct {
	Status           string       `json:"status"`
	BlockHash        *felt.Felt   `json:"block_hash"`
	BlockNumber      *big.Int     `json:"block_number"`
	TransactionIndex *big.Int     `json:"transaction_index"`
	Transaction      *Transaction `json:"transaction"`
}

func (c *GatewayClient) GetTransaction(transactionHash *felt.Felt) (*TransactionStatus, error) {
	queryUrl := c.buildQueryString("get_transaction", map[string]string{
		"transactionHash": "0x" + transactionHash.Text(16),
	})

	if body, err := c.get(queryUrl); err != nil {
		return nil, err
	} else {
		txStatus := new(TransactionStatus)
		if err = json.Unmarshal(body, txStatus); err != nil {
			return nil, err
		}
		return txStatus, nil
	}
}

type Event struct {
	From *felt.Felt   `json:"from_address"`
	Data []*felt.Felt `json:"data"`
	Keys []*felt.Felt `json:"keys"`
}

type L1ToL2Message struct {
	From     string       `json:"from_address"`
	Payload  []*felt.Felt `json:"payload"`
	Selector *felt.Felt   `json:"selector"`
	To       *felt.Felt   `json:"to_address"`
	Nonce    *felt.Felt   `json:"nonce"`
}

type L2ToL1Message struct {
	From    *felt.Felt   `json:"from_address"`
	Payload []*felt.Felt `json:"payload"`
	To      string       `json:"to_address"`
}

type ExecutionResources struct {
	Steps                  uint64 `json:"n_steps"`
	BuiltinInstanceCounter BuiltinInstanceCounter `json:"builtin_instance_counter"`
	MemoryHoles uint64 `json:"n_memory_holes"`
}

type BuiltinInstanceCounter struct {
	Pedersen   uint64 `json:"pedersen_builtin"`
	RangeCheck uint64 `json:"range_check_builtin"`
	Bitwise    uint64 `json:"bitwise_builtin"`
	Output     uint64 `json:"output_builtin"`
	Ecsda      uint64 `json:"ecdsa_builtin"`
	EcOp       uint64 `json:"ec_op_builtin"`
}

type TransactionReceipt struct {
	ActualFee          *felt.Felt          `json:"actual_fee"`
	Events             []*Event            `json:"events"`
	ExecutionResources *ExecutionResources `json:"execution_resources"`
	L1ToL2Message      *L1ToL2Message      `json:"l1_to_l2_consumed_message"`
	L2ToL1Message      []*L2ToL1Message    `json:"l2_to_l1_messages"`
	TransactionHash    *felt.Felt          `json:"transaction_hash"`
	TransactionIndex   *big.Int            `json:"transaction_index"`
}

// Block object returned by the gateway in JSON format for "get_block" endpoint
type Block struct {
	Hash             *felt.Felt            `json:"block_hash"`
	ParentHash       *felt.Felt            `json:"parent_block_hash"`
	Number           uint64                `json:"block_number"`
	StateRoot        *felt.Felt            `json:"state_root"`
	Status           string                `json:"status"`
	GasPrice         *felt.Felt            `json:"gas_price"`
	Transactions     []*Transaction        `json:"transactions"`
	Timestamp        uint64                `json:"timestamp"`
	Version          string                `json:"starknet_version"`
	Receipts         []*TransactionReceipt `json:"transaction_receipts"`
	SequencerAddress *felt.Felt            `json:"sequencer_address"`
}

func (c *GatewayClient) GetBlock(blockNumber uint64) (*Block, error) {
	queryUrl := c.buildQueryString("get_block", map[string]string{
		"blockNumber": strconv.FormatUint(blockNumber, 10),
	})

	if body, err := c.get(queryUrl); err != nil {
		return nil, err
	} else {
		block := new(Block)
		if err = json.Unmarshal(body, block); err != nil {
			return nil, err
		}
		return block, nil
	}
}

type EntryPoint struct {
	Selector *felt.Felt `json:"selector"`
	Offset   *felt.Felt `json:"offset"`
}

type (
	Hints       map[uint64]interface{}
	Identifiers map[string]struct {
		CairoType   string         `json:"cairo_type,omitempty"`
		Decorators  *[]interface{} `json:"decorators,omitempty"`
		Destination string         `json:"destination,omitempty"`
		FullName    string         `json:"full_name,omitempty"`
		Members     *interface{}   `json:"members,omitempty"`
		Pc          *uint64        `json:"pc,omitempty"`
		References  *[]interface{} `json:"references,omitempty"`
		Size        *uint64        `json:"size,omitempty"`
		Type        string         `json:"type,omitempty"`
		Value       json.Number    `json:"value,omitempty"`
	}
	Program struct {
		Attributes       interface{} `json:"attributes,omitempty"`
		Builtins         []string    `json:"builtins"`
		CompilerVersion  string      `json:"compiler_version,omitempty"`
		Data             []string    `json:"data"`
		DebugInfo        interface{} `json:"debug_info"`
		Hints            Hints       `json:"hints"`
		Identifiers      Identifiers `json:"identifiers"`
		MainScope        interface{} `json:"main_scope"`
		Prime            string      `json:"prime"`
		ReferenceManager interface{} `json:"reference_manager"`
	}
)

type ClassDefinition struct {
	Abi         interface{} `json:"abi"`
	EntryPoints struct {
		Constructor []EntryPoint `json:"CONSTRUCTOR"`
		External    []EntryPoint `json:"EXTERNAL"`
		L1Handler   []EntryPoint `json:"L1_HANDLER"`
	} `json:"entry_points_by_type"`
	Program Program `json:"program"`
}

func (c *GatewayClient) GetClassDefinition(classHash *felt.Felt) (*ClassDefinition, error) {
	queryUrl := c.buildQueryString("get_class_by_hash", map[string]string{
		"classHash": "0x" + classHash.Text(16),
	})

	if body, err := c.get(queryUrl); err != nil {
		return nil, err
	} else {
		class := new(ClassDefinition)
		if err = json.Unmarshal(body, class); err != nil {
			return nil, err
		}
		return class, nil
	}
}
