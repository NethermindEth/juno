package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	typeArray   = "array"
	typeInteger = "integer"
	typeObject  = "object"
	typeString  = "string"

	entryPointSelector = "entry_point_selector"
	largeOutput        = " The output can be very large."
)

// tool is a JSON-RPC method served as an MCP tool of the same name.
type tool struct {
	name        string
	description string
	args        []arg // the method's parameters, in order
}

// arg is a tool argument, passed to the JSON-RPC method as the parameter of the same name.
type arg struct {
	name     string
	schema   *jsonschema.Schema
	optional bool
	// normalise, if set, rewrites lenient inputs into the form the JSON-RPC method expects.
	normalise normaliser
}

func (t *tool) mcpTool() *mcpsdk.Tool {
	return &mcpsdk.Tool{
		Name:        t.name,
		Description: t.description,
		InputSchema: objectSchema("", t.args...),
		Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(false)},
	}
}

// params turns the tool call arguments into named parameters for the JSON-RPC method.
func (t *tool) params(arguments json.RawMessage) (map[string]json.RawMessage, error) {
	var params map[string]json.RawMessage
	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &params); err != nil {
			return nil, fmt.Errorf("the arguments must be a JSON object: %w", err)
		}
	}
	for _, a := range t.args {
		if value, ok := params[a.name]; ok && a.normalise != nil {
			params[a.name] = a.normalise(value)
		}
	}
	return params, nil
}

func objectSchema(description string, properties ...arg) *jsonschema.Schema {
	schema := &jsonschema.Schema{
		Type:        typeObject,
		Description: description,
		Properties:  make(map[string]*jsonschema.Schema, len(properties)),
	}
	for _, property := range properties {
		schema.Properties[property.name] = property.schema
		schema.PropertyOrder = append(schema.PropertyOrder, property.name)
		if !property.optional {
			schema.Required = append(schema.Required, property.name)
		}
	}
	return schema
}

func arraySchema(description string, items *jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{Type: typeArray, Description: description, Items: items}
}

func feltSchema(description string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:        typeString,
		Description: description,
		Pattern:     "^0x[0-9a-fA-F]{1,64}$",
	}
}

func flagsSchema(description string, flags ...any) *jsonschema.Schema {
	return arraySchema(description, &jsonschema.Schema{Type: typeString, Enum: flags})
}

var (
	blockIDSchema = &jsonschema.Schema{
		Type: typeString,
		Description: `The block: "latest", "pre_confirmed", "l1_accepted", a block number ` +
			`such as "123456", or a block hash (0x-prefixed hex).`,
	}
	blockIDArg = arg{name: "block_id", schema: blockIDSchema, normalise: normaliseBlockID}

	contractAddressArg = arg{name: "contract_address", schema: feltSchema("The contract address.")}
	classHashArg       = arg{name: "class_hash", schema: feltSchema("The class hash.")}
	transactionHashArg = arg{
		name:   "transaction_hash",
		schema: feltSchema("The transaction hash."),
	}
	transactionFlagsArg = arg{
		name:     "response_flags",
		optional: true,
		schema: flagsSchema(
			"INCLUDE_PROOF_FACTS adds the proof_facts field to the returned transactions.",
			"INCLUDE_PROOF_FACTS",
		),
	}

	feltItemSchema = feltSchema("")
	selectorArg    = arg{
		name: entryPointSelector,
		schema: &jsonschema.Schema{
			Type: typeString,
			Description: "The entry point selector (0x-prefixed hex), or the entry point name, " +
				`such as "balance_of".`,
		},
	}
	selectorNormaliser = fieldNormaliser(normaliseSelector, entryPointSelector)

	broadcastedTransactionsSchema = arraySchema(
		"BROADCASTED_TXN objects, as defined by the Starknet JSON-RPC v0.10 specification. "+
			"Only v3 transactions are supported.",
		&jsonschema.Schema{Type: typeObject},
	)

	eventFilterSchema = objectSchema("The event filter and the page to return.",
		arg{name: "from_block", schema: blockIDSchema, optional: true},
		arg{name: "to_block", schema: blockIDSchema, optional: true},
		arg{
			name:     "address",
			schema:   feltSchema("Only return the events emitted by this contract."),
			optional: true,
		},
		arg{
			name: "keys",
			schema: arraySchema("The values accepted for each event key, by position. "+
				"An empty list accepts any value. The first key is usually the event selector.",
				arraySchema("", feltItemSchema)),
			optional: true,
		},
		arg{
			name: "chunk_size",
			schema: &jsonschema.Schema{
				Type:        typeInteger,
				Minimum:     new(1.0),
				Description: "The maximum number of events to return.",
			},
		},
		arg{
			name: "continuation_token",
			schema: &jsonschema.Schema{
				Type:        typeString,
				Description: "The continuation_token of the previous page, to get the next one.",
			},
			optional: true,
		},
	)

	functionCallSchema = objectSchema("The function call.",
		contractAddressArg,
		selectorArg,
		arg{name: "calldata", schema: arraySchema("The function arguments.", feltItemSchema)},
	)

	messageSchema = objectSchema("The message sent from L1.",
		arg{name: "from_address", schema: &jsonschema.Schema{
			Type:        typeString,
			Description: "The address of the L1 contract sending the message.",
			Pattern:     "^0x[0-9a-fA-F]{40}$",
		}},
		arg{name: "to_address", schema: feltSchema("The address of the L2 contract receiving it.")},
		selectorArg,
		arg{name: "payload", schema: arraySchema("The message payload.", feltItemSchema)},
	)
)

// tools are the Starknet JSON-RPC v0.10 methods served as MCP tools.
var tools = []tool{
	{
		name:        "starknet_specVersion",
		description: "Returns the version of the Starknet JSON-RPC specification being used.",
	},
	{
		name:        "starknet_chainId",
		description: "Returns the chain id of the Starknet network.",
	},
	{
		name:        "starknet_blockNumber",
		description: "Returns the number of the most recent accepted block.",
	},
	{
		name:        "starknet_blockHashAndNumber",
		description: "Returns the hash and number of the most recent accepted block.",
	},
	{
		name:        "starknet_syncing",
		description: "Returns the sync status of the node, or false if it is not syncing.",
	},
	{
		name:        "starknet_getBlockWithTxHashes",
		description: "Returns a block with the hashes of its transactions.",
		args:        []arg{blockIDArg},
	},
	{
		name:        "starknet_getBlockWithTxs",
		description: "Returns a block with its transactions." + largeOutput,
		args:        []arg{blockIDArg, transactionFlagsArg},
	},
	{
		name:        "starknet_getBlockWithReceipts",
		description: "Returns a block with its transactions and their receipts." + largeOutput,
		args:        []arg{blockIDArg, transactionFlagsArg},
	},
	{
		name:        "starknet_getBlockTransactionCount",
		description: "Returns the number of transactions in a block.",
		args:        []arg{blockIDArg},
	},
	{
		name:        "starknet_getStateUpdate",
		description: "Returns the state changes made by a block." + largeOutput,
		args: []arg{blockIDArg, {
			name:     "contract_addresses",
			schema:   arraySchema("Only return the state changes of these contracts.", feltItemSchema),
			optional: true,
		}},
	},
	{
		name:        "starknet_getTransactionByHash",
		description: "Returns the details of a transaction.",
		args:        []arg{transactionHashArg, transactionFlagsArg},
	},
	{
		name:        "starknet_getTransactionByBlockIdAndIndex",
		description: "Returns the transaction at the given index of a block.",
		args: []arg{blockIDArg, {
			name: "index",
			schema: &jsonschema.Schema{
				Type:        typeInteger,
				Minimum:     new(0.0),
				Description: "The index of the transaction in the block.",
			},
		}, transactionFlagsArg},
	},
	{
		name:        "starknet_getTransactionReceipt",
		description: "Returns the receipt of a transaction.",
		args:        []arg{transactionHashArg},
	},
	{
		name: "starknet_getTransactionStatus",
		description: "Returns the status of a transaction, which may still be in the mempool or " +
			"have been dropped from it.",
		args: []arg{transactionHashArg},
	},
	{
		name:        "starknet_getNonce",
		description: "Returns the nonce of a contract.",
		args:        []arg{blockIDArg, contractAddressArg},
	},
	{
		name:        "starknet_getStorageAt",
		description: "Returns the value stored by a contract at the given key.",
		args: []arg{
			contractAddressArg,
			{name: "key", schema: feltSchema("The storage key.")},
			blockIDArg,
			{
				name: "response_flags",
				schema: flagsSchema("INCLUDE_LAST_UPDATE_BLOCK also returns the number of the last "+
					"block that modified the value.", "INCLUDE_LAST_UPDATE_BLOCK"),
				optional: true,
			},
		},
	},
	{
		name:        "starknet_getClassHashAt",
		description: "Returns the class hash of the contract deployed at the given address.",
		args:        []arg{blockIDArg, contractAddressArg},
	},
	{
		name:        "starknet_getClass",
		description: "Returns the definition of the contract class with the given hash." + largeOutput,
		args:        []arg{blockIDArg, classHashArg},
	},
	{
		name: "starknet_getClassAt",
		description: "Returns the class definition of the contract deployed at the given address." +
			largeOutput,
		args: []arg{blockIDArg, contractAddressArg},
	},
	{
		name:        "starknet_getEvents",
		description: "Returns the events matching a filter, one page at a time.",
		args: []arg{{
			name:      "filter",
			schema:    eventFilterSchema,
			normalise: fieldNormaliser(normaliseBlockID, "from_block", "to_block"),
		}},
	},
	{
		name: "starknet_call",
		description: "Calls a contract function without creating a transaction, and returns the " +
			"values it returns.",
		args: []arg{
			{name: "request", schema: functionCallSchema, normalise: selectorNormaliser},
			blockIDArg,
		},
	},
	{
		name:        "starknet_estimateFee",
		description: "Estimates the fees of a sequence of transactions.",
		args: []arg{
			{name: "request", schema: broadcastedTransactionsSchema},
			{name: "simulation_flags", schema: flagsSchema(
				"SKIP_VALIDATE skips the validation of the transactions.", "SKIP_VALIDATE",
			)},
			blockIDArg,
		},
	},
	{
		name:        "starknet_estimateMessageFee",
		description: "Estimates the L2 fee of a message sent from L1.",
		args: []arg{
			{name: "message", schema: messageSchema, normalise: selectorNormaliser},
			blockIDArg,
		},
	},
	{
		name: "starknet_simulateTransactions",
		description: "Simulates a sequence of transactions on the state of a block, and returns " +
			"their execution traces." + largeOutput,
		args: []arg{
			blockIDArg,
			{name: "transactions", schema: broadcastedTransactionsSchema},
			{name: "simulation_flags", schema: flagsSchema(
				"SKIP_VALIDATE skips the validation of the transactions, SKIP_FEE_CHARGE does not "+
					"charge their fees, RETURN_INITIAL_READS also returns the state values read.",
				"SKIP_VALIDATE", "SKIP_FEE_CHARGE", "RETURN_INITIAL_READS",
			)},
		},
	},
	{
		name: "starknet_traceTransaction",
		description: "Returns the execution trace of a transaction, including its internal calls." +
			largeOutput,
		args: []arg{transactionHashArg},
	},
	{
		name:        "starknet_traceBlockTransactions",
		description: "Returns the execution traces of all the transactions in a block." + largeOutput,
		args: []arg{blockIDArg, {
			name: "trace_flags",
			schema: flagsSchema("RETURN_INITIAL_READS also returns the state values read.",
				"RETURN_INITIAL_READS"),
			optional: true,
		}},
	},
}
