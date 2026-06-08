package jsonrpc_test

import (
	"strings"
	"testing"

	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var parseErrorTests = map[string]struct {
	req string
	res string
}{
	"invalid json": {
		req: `{]`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{]\n ^\nunexpected ']', expected a string key or '}' [line 1, position 2]"},"id":null}`,
	},

	"invalid json batch path": {
		req: `[{]`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"[{]\n  ^\nunexpected ']', expected a string key or '}' [line 1, position 3]"},"id":null}`,
	},

	"missing closing brace": {
		req: `{"jsonrpc": "2.0", "method": "method", "id": 1`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\"jsonrpc\": \"2.0\", \"method\": \"method\", \"id\": 1\n                                              ^\nunexpected end of input [line 1, position 47]"},"id":null}`,
	},

	"leading blank line keeps line number": {
		req: "\n{]",
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"\n{]\n ^\nunexpected ']', expected a string key or '}' [line 2, position 2]"},"id":null}`,
	},

	"trailing comma in object": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_blockNumber",
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_blockNumber\",\n}\n^\nunexpected trailing comma before '}' [line 4, position 1]"},"id":null}`,
	},

	"trailing comma with whitespace": {
		req: `{"jsonrpc": "2.0", "method": "starknet_blockNumber", }`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\"jsonrpc\": \"2.0\", \"method\": \"starknet_blockNumber\", }\n                                                     ^\nunexpected trailing comma before '}' [line 1, position 54]"},"id":null}`,
	},

	"empty input": {
		req: ``,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"\n^\nunexpected end of input [line 1, position 1]"},"id":null}`,
	},

	"trailing comma in array": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": ["0x1", "0x2",],
  "id": 1
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n  \"params\": [\"0x1\", \"0x2\",],\n                          ^\nunexpected trailing comma before ']' [line 4, position 27]"},"id":null}`,
	},

	"unexpected token expecting value": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_chainId",
  "id": @
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_chainId\",\n  \"id\": @\n        ^\nunexpected '@', expected a value [line 4, position 9]"},"id":null}`,
	},

	"missing comma between array elements": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": ["0x1" "0x2"],
  "id": 1
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n  \"params\": [\"0x1\" \"0x2\"],\n                   ^\nunexpected '\"', expected ',' or ']' [line 4, position 20]"},"id":null}`,
	},

	"param type mismatch": {
		req: `{"jsonrpc": 5, "method": "x", "id": 1}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\"jsonrpc\": 5, \"method\": \"x\", \"id\": 1}\n            ^\nfield \"jsonrpc\" should be string, got number [line 1, position 13]"},"id":null}`,
	},

	"long line is windowed": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": {"calldata": [` + strings.Repeat(`"0x1", `, 20) + `"0x2"] z},
  "id": 1
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n... \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x1\", \"0x2\"] z},\n                                                                          ^\nunexpected 'z', expected ',' or '}' [line 4, position 174]"},"id":null}`,
	},

	"error at start of long line": {
		req: `{@"jsonrpc": "2.0", "method": "starknet_estimateFee", "params": [` + strings.Repeat(`"0xdeadbeef", `, 20) + `"0x0"]}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{@\"jsonrpc\": \"2.0\", \"method\": \"starknet_estimateFee\", \"params\": [\"0xdeadbe...\n ^\nunexpected '@', expected a string key or '}' [line 1, position 2]"},"id":null}`,
	},

	"top-level type mismatch": {
		req: `5`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"5\n^\nexpected a JSON object, got number [line 1, position 1]"},"id":null}`,
	},

	"untranslatable syntax error": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_syncing",
  "params": truX
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_syncing\",\n  \"params\": truX\n               ^\ninvalid character 'X' in literal true (expecting 'e') [line 4, position 16]"},"id":null}`,
	},

	"error on a middle line": {
		req: "{\n\"a\" 1\n}",
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n\"a\" 1\n    ^\nunexpected '1', expected ':' [line 2, position 5]"},"id":null}`,
	},

	"multiline starknet request with missing comma": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_getStorageAt",
  "params": ["0x4c5772d", "0x206f38f" "latest"],
  "id": 1
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_getStorageAt\",\n  \"params\": [\"0x4c5772d\", \"0x206f38f\" \"latest\"],\n                                      ^\nunexpected '\"', expected ',' or ']' [line 4, position 39]"},"id":null}`,
	},

	"error past the captured window": {
		req: "[\n" + strings.Repeat("\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7\",\n", 40) + "\"0xbad\" \"0x1\"\n]",
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7\",\n\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7\",\n\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7\",\n\"0xbad\" \"0x1\"\n        ^\nunexpected '\"', expected ',' or ']' [line 42, position 9]"},"id":null}`,
	},

	"oversized single-line input keeps only the trailing window": {
		req: `{"jsonrpc": "2.0", "method": "starknet_call", "params": [` + strings.Repeat(`"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", `, 10) + `"0xbad" @]}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"...36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7\", \"0xbad\" @]}\n                                                                          ^\nunexpected '@', expected ',' or ']' [line 1, position 510]"},"id":null}`,
	},

	"context is capped at three lines within the window": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": {
    "contract_address": "0x04c5772d",
    "entry_point_selector": "0x0206f38f"
    "calldata": []
  },
  "id": 1
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"  \"params\": {\n    \"contract_address\": \"0x04c5772d\",\n    \"entry_point_selector\": \"0x0206f38f\"\n    \"calldata\": []\n    ^\nunexpected '\"', expected ',' or '}' [line 7, position 5]"},"id":null}`,
	},

	"long line is windowed on both sides": {
		req: `{"jsonrpc": "2.0", "method": "starknet_getStorageAt", "params": {"contract_address": "0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7" @ "key": "0x02f0b3c5710379609eb5495f1ecd348cb28167711b73609fe565a72734550354"}, "id": 1}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"...84644ddd6b96f7c741b1562b82f9e004dc7\" @ \"key\": \"0x02f0b3c5710379609eb5495f1...\n                                        ^\nunexpected '@', expected ',' or '}' [line 1, position 155]"},"id":null}`,
	},

	"long preceding context line is windowed": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "params": ["0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7", "0x02f0b3c5710379609eb5495f1ecd348cb28167711b73609fe565a72734550354", "latest"]
  "id": 1
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n  \"params\": [\"0x049d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e...\n  \"id\": 1\n  ^\nunexpected '\"', expected ',' or '}' [line 5, position 3]"},"id":null}`,
	},

	"oversized line drops all preceding context": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_estimateFee",
  "params": {"request": [{"calldata": [` + strings.Repeat(`"0xdeadbeef", `, 40) + `"0x0"]}]}
  "id": 1
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"  \"id\": 1\n  ^\nunexpected '\"', expected ',' or '}' [line 5, position 3]"},"id":null}`,
	},

	"column counts runes not bytes": {
		req: `{
  "jsonrpc": "2.0",
  "method": "starknet_call",
  "👍": @
}`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\n  \"👍\": @\n       ^\nunexpected '@', expected a value [line 4, position 8]"},"id":null}`,
	},

	"rpc call with invalid JSON": {
		req: `{"jsonrpc": "2.0", "method": "foobar, "params": "bar", "baz]`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"{\"jsonrpc\": \"2.0\", \"method\": \"foobar, \"params\": \"bar\", \"baz]\n                                       ^\nunexpected 'p', expected ',' or '}' [line 1, position 40]"},"id":null}`,
	},

	"rpc call Batch, invalid JSON:": {
		req: `[
  {"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": "1"},
  {"jsonrpc": "2.0", "method"
]`,
		res: `{"jsonrpc":"2.0","error":{"code":-32700,"message":"Parse error","data":"[\n  {\"jsonrpc\": \"2.0\", \"method\": \"sum\", \"params\": [1,2,4], \"id\": \"1\"},\n  {\"jsonrpc\": \"2.0\", \"method\"\n]\n^\nunexpected ']', expected ':' [line 4, position 1]"},"id":null}`,
	},
}

func TestHandleParseError(t *testing.T) {
	server := jsonrpc.NewServer(1, log.NewNopZapLogger())

	for desc, test := range parseErrorTests {
		t.Run(desc, func(t *testing.T) {
			res, httpHeader, err := server.HandleReader(t.Context(), strings.NewReader(test.req))
			require.NoError(t, err)
			assert.NotNil(t, httpHeader)
			assert.JSONEq(t, test.res, string(res))
		})
	}
}
