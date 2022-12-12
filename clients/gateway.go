package clients

import (
	"encoding/json"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"strconv"

	"github.com/consensys/gnark-crypto/ecc/stark-curve/fp"
)

type GatewayClient struct {
	baseUrl string
}

func NewGatewayClient(baseUrl string) *GatewayClient {
	return &GatewayClient{
		baseUrl: baseUrl,
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
	}

	body, err := ioutil.ReadAll(res.Body)
	return body, err
}

// StateUpdate object returned by the gateway in JSON format for "get_state_update" endpoint
type StateUpdate struct {
	BlockHash *fp.Element `json:"block_hash"`
	NewRoot   *fp.Element `json:"new_root"`
	OldRoot   *fp.Element `json:"old_root"`

	StateDiff struct {
		StorageDiffs map[string][]struct {
			Key   *fp.Element `json:"key"`
			Value *fp.Element `json:"value"`
		} `json:"storage_diffs"`

		Nonces            interface{} `json:"nonces"` // todo: define
		DeployedContracts []struct {
			Address   *fp.Element `json:"address"`
			ClassHash *fp.Element `json:"class_hash"`
		} `json:"deployed_contracts"`
		DeclaredContracts interface{} `json:"declared_contracts"` // todo: define
	} `json:"state_diff"`
}

func (c *GatewayClient) GetStateUpdate(blockNumber uint64) (*StateUpdate, error) {
	queryUrl := c.buildQueryString("get_state_update", map[string]string{
		"blockNumber": strconv.FormatUint(blockNumber, 10),
	})

	body, err := c.get(queryUrl)
	update := new(StateUpdate)
	if err = json.Unmarshal(body, update); err != nil {
		return nil, err
	}

	return update, nil
}

// Transaction object returned by the gateway in JSON format for multiple endpoints
type Transaction struct {
	Hash                *fp.Element   `json:"transaction_hash"`
	Version             *fp.Element   `json:"version"`
	ContractAddress     *fp.Element   `json:"contract_address"`
	ContractAddressSalt *fp.Element   `json:"contract_address_salt"`
	ClassHash           *fp.Element   `json:"class_hash"`
	ConstructorCalldata []*fp.Element `json:"constructor_calldata"`
	Type                string        `json:"type"`
	// invoke
	MaxFee             *fp.Element   `json:"max_fee"`
	Signature          []*fp.Element `json:"signature"`
	Calldata           []*fp.Element `json:"calldata"`
	EntryPointSelector *fp.Element   `json:"entry_point_selector"`
	// declare/deploy_account
	Nonce *fp.Element `json:"nonce"`
	// declare
	SenderAddress *fp.Element `json:"sender_address"`
}

type TransactionStatus struct {
	Status           string       `json:"status"`
	BlockHash        *fp.Element  `json:"block_hash"`
	BlockNumber      *big.Int     `json:"block_number"`
	TransactionIndex *big.Int     `json:"transaction_index"`
	Transaction      *Transaction `json:"transaction"`
}

func (c *GatewayClient) GetTransaction(transactionHash *fp.Element) (*TransactionStatus, error) {
	queryUrl := c.buildQueryString("get_transaction", map[string]string{
		"transactionHash": "0x" + transactionHash.Text(16),
	})

	body, err := c.get(queryUrl)
	txStatus := new(TransactionStatus)
	if err = json.Unmarshal(body, txStatus); err != nil {
		return nil, err
	}

	return txStatus, nil
}

type Event struct {
	From *fp.Element   `json:"from_address"`
	Data []*fp.Element `json:"data"`
	Keys []*fp.Element `json:"keys"`
}

type L1ToL2Message struct {
	From     string        `json:"from_address"`
	Payload  []*fp.Element `json:"payload"`
	Selector *fp.Element   `json:"selector"`
	To       *fp.Element   `json:"to_address"`
	Nonce    *fp.Element   `json:"nonce"`
}

type L2ToL1Message struct {
	From    *fp.Element   `json:"from_address"`
	Payload []*fp.Element `json:"payload"`
	To      string        `json:"to_address"`
}

type ExecutionResources struct {
	Steps                  uint64 `json:"n_steps"`
	BuiltinInstanceCounter struct {
		Pedersen   uint64 `json:"pedersen_builtin"`
		RangeCheck uint64 `json:"range_check_builtin"`
		Bitwise    uint64 `json:"bitwise_builtin"`
		Output     uint64 `json:"output_builtin"`
		Ecsda      uint64 `json:"ecdsa_builtin"`
		EcOp       uint64 `json:"ec_op_builtin"`
	} `json:"builtin_instance_counter"`
	MemoryHoles uint64 `json:"n_memory_holes"`
}

type TransactionReceipt struct {
	ActualFee          *fp.Element         `json:"actual_fee"`
	Events             []*Event            `json:"events"`
	ExecutionResources *ExecutionResources `json:"execution_resources"`
	L1ToL2Message      *L1ToL2Message      `json:"l1_to_l2_consumed_message"`
	L2ToL1Message      *[]L2ToL1Message    `json:"l2_to_l1_messages"`
	TransactionHash    *fp.Element         `json:"transaction_hash"`
	TransactionIndex   *big.Int            `json:"transaction_index"`
}

// Block object returned by the gateway in JSON format for "get_block" endpoint
type Block struct {
	Hash         *fp.Element           `json:"block_hash"`
	ParentHash   *fp.Element           `json:"parent_block_hash"`
	Number       uint64                `json:"block_number"`
	StateRoot    *fp.Element           `json:"state_root"`
	Status       string                `json:"status"`
	GasPrice     *fp.Element           `json:"gas_price"`
	Transactions []*Transaction        `json:"transactions"`
	Timestamp    uint64                `json:"timestamp"`
	Version      string                `json:"starknet_version"`
	Receipts     []*TransactionReceipt `json:"transaction_receipts"`
}

func (c *GatewayClient) GetBlock(blockNumber uint64) (*Block, error) {
	queryUrl := c.buildQueryString("get_block", map[string]string{
		"blockNumber": strconv.FormatUint(blockNumber, 10),
	})

	body, err := c.get(queryUrl)
	block := new(Block)
	if err = json.Unmarshal(body, block); err != nil {
		return nil, err
	}

	return block, nil
}

type EntryPoint struct {
	Selector *fp.Element `json:"selector"`
	Offset   *fp.Element `json:"offset"`
}

type Abi []struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Inputs []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"inputs"`
	Outputs []interface{} `json:"outputs"`
}

type ClassDefinition struct {
	Abi         Abi `json:"abi"`
	EntryPoints struct {
		Constructor []EntryPoint `json:"CONSTRUCTOR"`
		External    []EntryPoint `json:"EXTERNAL"`
		L1Handler   []EntryPoint `json:"L1_HANDLER"`
	} `json:"entry_points_by_type"`
	Program struct {
		Builtins         []string      `json:"builtins"`
		Prime            string        `json:"prime"`
		ReferenceManager interface{}   `json:"reference_manager"`
		Identifiers      interface{}   `json:"identifiers"`
		Attributes       interface{}   `json:"attributes"`
		Data             []*fp.Element `json:"data"`
		DebugInfo        interface{}   `json:"debug_info"`
		MainScope        interface{}   `json:"main_scope"`
		Hints            interface{}   `json:"hints"`
		CompilerVersion  string        `json:"compiler_version"`
	} `json:"program"`
}

func (c *GatewayClient) GetClassDefinition(classHash *fp.Element) (*ClassDefinition, error) {
	queryUrl := c.buildQueryString("get_class_by_hash", map[string]string{
		"classHash": "0x" + classHash.Text(16),
	})

	body, err := c.get(queryUrl)
	class := new(ClassDefinition)
	if err = json.Unmarshal(body, class); err != nil {
		return nil, err
	}

	return class, nil
}
