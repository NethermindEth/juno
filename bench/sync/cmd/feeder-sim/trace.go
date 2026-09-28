package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

type traceAPI struct {
	url *url.URL
}

type traceRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  traceParams `json:"params"`
}

type traceParams struct {
	BlockID blockID `json:"block_id"`
}

type blockID struct {
	BlockNumber uint64 `json:"block_number"`
}

type traceResponse struct {
	Result []tracedTransaction `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type tracedTransaction struct {
	TransactionHash string `json:"transaction_hash"`
	TraceRoot       struct {
		StateDiff *rpcStateDiff `json:"state_diff"`
	} `json:"trace_root"`
}

type rpcStateDiff struct {
	StorageDiffs []struct {
		Address        string `json:"address"`
		StorageEntries []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"storage_entries"`
	} `json:"storage_diffs"`
	Nonces []struct {
		ContractAddress string `json:"contract_address"`
		Nonce           string `json:"nonce"`
	} `json:"nonces"`
	DeployedContracts []struct {
		Address   string `json:"address"`
		ClassHash string `json:"class_hash"`
	} `json:"deployed_contracts"`
	DeprecatedDeclaredClasses []string `json:"deprecated_declared_classes"`
	DeclaredClasses           []struct {
		ClassHash         string `json:"class_hash"`
		CompiledClassHash string `json:"compiled_class_hash"`
	} `json:"declared_classes"`
	ReplacedClasses []struct {
		ContractAddress string `json:"contract_address"`
		ClassHash       string `json:"class_hash"`
	} `json:"replaced_classes"`
	MigratedCompiledClasses []struct {
		ClassHash         string `json:"class_hash"`
		CompiledClassHash string `json:"compiled_class_hash"`
	} `json:"migrated_compiled_classes"`
}

type rawBlock struct {
	Block *confirmedBlock `json:"block"`
}

func (trace *traceAPI) newRequest(ctx context.Context, resource resource) (*http.Request, error) {
	key, err := preConfirmedBlock.key(resource.query)
	if err != nil {
		return nil, err
	}

	payload := traceRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "starknet_traceBlockTransactions",
		Params:  traceParams{BlockID: blockID(key)},
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, trace.url.String(), &body)
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	return request, nil
}

func (trace *traceAPI) decode(
	dataset dataset,
	resource resource,
	body []byte,
	encoding string,
) ([]byte, error) {
	key, err := preConfirmedBlock.key(resource.query)
	if err != nil {
		return nil, err
	}

	block, err := readConfirmedBlock(dataset, key)
	if err != nil {
		return nil, err
	}

	traces, err := decodeTraces(body, encoding)
	if err != nil {
		return nil, fmt.Errorf("tracing block %d: %w", key.BlockNumber, err)
	}

	return buildRound(key.BlockNumber, block, traces)
}

func readConfirmedBlock(dataset dataset, key blockKey) (*confirmedBlock, error) {
	confirmed, err := stateUpdate.resource(key)
	if err != nil {
		return nil, err
	}

	stateUpdateGz, err := dataset.read(confirmed.file)
	if err != nil {
		return nil, err
	}

	response, err := unmarshalGzipped[rawBlock](stateUpdateGz)
	if err != nil {
		return nil, err
	}

	if response.Block == nil {
		return nil, errors.New("state update response lacks block")
	}

	return response.Block, nil
}

func decodeTraces(body []byte, encoding string) ([]tracedTransaction, error) {
	switch encoding {
	case "gzip":
		unzipped, err := gunzip(body)
		if err != nil {
			return nil, err
		}

		body = unzipped
	case "", "identity":
	default:
		return nil, fmt.Errorf("unsupported Content-Encoding %q", encoding)
	}

	var response traceResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", response.Error.Code, response.Error.Message)
	}

	return response.Result, nil
}
