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

func TestHandlerRPC_StarknetCall(t *testing.T) {
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
	})
}

func init() {

	logger = log.GetLogger()
}
