package main_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	corpusgen "github.com/NethermindEth/juno/bench/rpc/cmd/corpus-gen"
	"github.com/stretchr/testify/require"
)

const (
	fakeLatestBlock = 100

	fakeInvokeTx = `{
		"transaction_hash": "0x1",
		"type": "INVOKE",
		"version": "0x3",
		"nonce": "0x7",
		"sender_address": "0x2",
		"signature": ["0x3"],
		"calldata": ["0x1", "0xa", "0xb", "0x0"]
	}`
	fakeL1HandlerTx = `{"transaction_hash":"0x4","type":"L1_HANDLER","version":"0x0"}`
)

type functionCall struct {
	ContractAddress    string   `json:"contract_address"`
	EntryPointSelector string   `json:"entry_point_selector"`
	Calldata           []string `json:"calldata"`
}

type corpusEntry struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type corpusFile struct {
	Requests []corpusEntry `json:"requests"`
}

func runCorpusGen(t *testing.T, args ...string) corpusFile {
	t.Helper()

	var out bytes.Buffer
	cmd := corpusgen.NewRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	require.NoError(t, cmd.Execute())

	var corpus corpusFile
	require.NoError(t, json.Unmarshal(out.Bytes(), &corpus))
	return corpus
}

func corpusGenError(t *testing.T, args ...string) string {
	t.Helper()

	cmd := corpusgen.NewRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.Execute()
	require.Error(t, err)
	return err.Error()
}

// fakeNode answers the calls corpus-gen makes while it samples an execution
// request, and records what it was asked for.
type fakeNode struct {
	txs []string

	mu           sync.Mutex
	blocks       []uint64
	flags        []string
	verified     int
	stateUpdates int
}

func newFakeNode(txs ...string) *fakeNode {
	return &fakeNode{txs: txs}
}

func (n *fakeNode) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method string `json:"method"`
		Params struct {
			BlockID struct {
				BlockNumber uint64 `json:"block_number"`
			} `json:"block_id"`
			ResponseFlags []string `json:"response_flags"`
		} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var result string
	switch req.Method {
	case "starknet_specVersion":
		result = `"0.10.0"`
	case "starknet_blockNumber":
		result = strconv.Itoa(fakeLatestBlock)
	case "starknet_getBlockWithTxs":
		n.recordBlock(req.Params.BlockID.BlockNumber, req.Params.ResponseFlags)
		result = `{"transactions":[` + strings.Join(n.txs, ",") + `]}`
	case "starknet_getStateUpdate":
		n.recordStateUpdate()
		result = `{"state_diff":{"storage_diffs":[],"nonces":[]}}`
	case "starknet_estimateFee", "starknet_simulateTransactions":
		n.recordVerification()
		result = `[]`
	case "starknet_call":
		n.recordVerification()
		result = `["0x0"]`
	default:
		http.Error(w, "unexpected method "+req.Method, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":`+result+`}`)
}

func (n *fakeNode) recordBlock(blockNumber uint64, flags []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.blocks = append(n.blocks, blockNumber)
	n.flags = append(n.flags, flags...)
}

func (n *fakeNode) recordVerification() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.verified++
}

func (n *fakeNode) recordStateUpdate() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stateUpdates++
}

func (n *fakeNode) stateUpdatesAsked() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.stateUpdates
}

func (n *fakeNode) blocksAsked() []uint64 {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.blocks)
}

func (n *fakeNode) proofFactsFlags() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return slices.Clone(n.flags)
}

func (n *fakeNode) verifications() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.verified
}
