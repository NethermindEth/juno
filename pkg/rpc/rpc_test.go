package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/store"
	"github.com/NethermindEth/juno/pkg/trie"
)

func getServerHandler() *HandlerJsonRpc {
	return NewHandlerJsonRpc(HandlerRPC{})
}

type rpcTest struct {
	Request  string `json:"request"`
	Response string `json:"response"`
}

func testServer(t *testing.T, tests []rpcTest) {
	server := getServerHandler()

	for i, v := range tests {
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer([]byte(v.Request)))
		w := httptest.NewRecorder()
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(w, req)
		res := w.Result()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Errorf("expected error to be nil got %v", err)
			_ = res.Body.Close()
		}
		s := string(data)
		if s != v.Response {
			t.Errorf("expected %v, got %v", v.Response, string(data))
			_ = res.Body.Close()
		}
		t.Log("Executed test ", i)
	}
}

func TestTrieAdapter(t *testing.T) {
	height := 251
	// Contract storage trie with a single slot modified.
	slot := big.NewInt(132)
	val := big.NewInt(3)
	storage := trie.New(store.New(), height)
	storage.Put(slot, val)

	// Global state trie with a single contact.
	addr, _ := new(big.Int).SetString("57dde83c18c0efe7123c36a52d704cf27d5c38cdf0b1e1edc3b0dae3ee4e374", 16)
	contractHash, _ := new(big.Int).SetString("50b2148c0d782914e0b12a1a32abe5e398930b7e914f82c65cb7afce0a0ab9b", 16)

	info := pedersen.Digest(pedersen.Digest(pedersen.Digest(contractHash, storage.Commitment()), new(big.Int)), new(big.Int))
	state := trie.New(store.New(), height)
	state.Put(addr, info)

	t.Run("", func(t *testing.T) {
		// Get the root node of the state trie.
		root := state.Root()
		fmt.Printf("key = patricia_node:%.64x\n", root.Hash)
		fmt.Printf("val = %.64x%.64x%.2x\n", root.Bottom, root.Path, root.Length)

		// XXX: Unclear whether the node's bottom value is always going to
		// give the desired result or whether it has be retrieved using
		// trie.Get(Node.Path).

		// The root node's bottom value gives the contract state to query
		// (see also above).
		fmt.Printf("key = contract_state:%.64x\n", root.Bottom)
		format := `val =
		{
			"storage_commitment_tree: {
				"root": %.64x,
				"height": %d,
				"contract_hash": %.64x
			}
		}
		`
		fmt.Printf(format+"\n", root.Hash, height, contractHash)

		// Next, the root node of the contract storage trie.
		root = storage.Root()
		fmt.Printf("key = patricia_node:%.64x\n", root.Hash)
		fmt.Printf("val = %.64x%.64x%.2x\n", root.Bottom, root.Path, root.Length)

		// Finally, the storage leaf (see comment above).
		fmt.Printf("key = starknet_storage_leaf:%.64x\n", root.Bottom)
		fmt.Printf("val = %.64x\n", root.Bottom)
	})
}

func TestRPCServer(t *testing.T) {
	jsonFile, err := os.Open("rpc_tests.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully opened rpc_tests.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, _ := io.ReadAll(jsonFile)

	// we initialize our Users array
	var tests []rpcTest

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	err = json.Unmarshal(byteValue, &tests)
	if err != nil {
		return
	}
	testServer(t, tests)
}

func TestServer(t *testing.T) {
	server := NewServer(":8080")
	go func() {
		_ = server.ListenAndServe()
	}()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	server.Close(ctx)
	cancel()
}
