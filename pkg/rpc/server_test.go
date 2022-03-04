package rpc

import (
	"bytes"
	"github.com/NethermindEth/juno/internal/log"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func getServerHandler() *MethodRepository {
	server := NewMethodRepositoryWithMethods(HandlerRPC{})
	return server
}

type rpcTest struct {
	request  string
	response string
}

func testServer(t *testing.T, tests []rpcTest) {
	server := getServerHandler()

	for i, v := range tests {
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer([]byte(v.request)))
		w := httptest.NewRecorder()
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(w, req)
		res := w.Result()
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Errorf("expected error to be nil got %v", err)
			_ = res.Body.Close()
		}
		s := string(data)
		if s != v.response {
			t.Errorf("expected %v, got %v", v.response, string(data))
			_ = res.Body.Close()
		}
		t.Log("Executed test ", i)
	}
}

func TestRPCServer(t *testing.T) {
	testServer(t, []rpcTest{
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"34\",\"method\":\"starknet_call\",\"params\":[{\"callata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"latest\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":[\"Response\",\"of\",\"starknet_call\"],\"id\":\"34\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"34\",\"method\":\"starknet_call\",\"params\":[{\"calldata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"latest\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":[\"Response\",\"of\",\"starknet_call\"],\"id\":\"34\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"34\",\"method\":\"starknet_call\",\"params\":9{[{\"calldata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"latest\"]}}",
			response: "{\"jsonrpc\":\"2.0\",\"error\":{\"code\":-32700,\"message\":\"Parse error\"}}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"0\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"pending\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"pending\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"0\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"2\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"pending\",\"TXN_HASH\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"pending\",\"parent_hash\":\"TXN_HASH\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"2\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"1\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"pending\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"1\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"3\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"pending\",\"TXN_HASH\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"3\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"4\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"latest\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"latest\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"4\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"5\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"latest\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"5\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"6\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"latest\",\"TXN_HASH\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"latest\",\"parent_hash\":\"TXN_HASH\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"6\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"7\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"latest\",\"TXN_HASH\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"7\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"8\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"latest\",\"FULL_TXNS\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"latest\",\"parent_hash\":\"FULL_TXNS\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"8\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"9\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"latest\",\"FULL_TXNS\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"9\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"10\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"latest\",\"FULL_TXN_AND_RECEIPTS\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"latest\",\"parent_hash\":\"FULL_TXN_AND_RECEIPTS\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"10\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"11\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"latest\",\"FULL_TXN_AND_RECEIPTS\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"11\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"39\",\"method\":\"starknet_protocolVersion\"}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":\"Here the Protocol Version\",\"id\":\"39\"}\n",
		},
	})
}

func init() {

	logger = log.GetLogger()
}
