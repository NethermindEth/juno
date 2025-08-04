package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/NethermindEth/juno/starknet"
)

type BlockDownloader struct {
	url string
	cli http.Client
}

func NewBlockDownloader(baseURL string) *BlockDownloader {
	bd := &BlockDownloader{
		url: baseURL,
		cli: *http.DefaultClient,
	}
	return bd
}

func (bd *BlockDownloader) buildQueryString(endpoint string, args map[string]string) string {
	base, err := url.Parse(bd.url)
	if err != nil {
		panic("Malformed feeder base URL")
	}

	base.Path += endpoint

	params := url.Values{}
	for k, v := range args {
		params.Add(k, v)
	}
	base.RawQuery = params.Encode()

	return base.String()
}

func (bd *BlockDownloader) getRequest(ctx context.Context, queryURL string) (io.ReadCloser, error) {
	maxRetries := 5

	var res *http.Response
	var err error
	wait := time.Duration(0)

	for range maxRetries + 1 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
			var req *http.Request
			req, err = http.NewRequestWithContext(ctx, http.MethodGet, queryURL, http.NoBody)
			if err != nil {
				fmt.Println("error:", err)
				return nil, err
			}
			res, err = bd.cli.Do(req)
			// tooManyRequests, badRequest := false, false
			if err == nil {
				// tooManyRequests = res.StatusCode == http.StatusTooManyRequests
				// badRequest = res.StatusCode == http.StatusBadRequest
				if res.StatusCode == http.StatusOK {
					return res.Body, nil
				} else {
					res.Body.Close()
					err = errors.New(res.Status)
				}
			}
			// if tooManyRequests || badRequest {
			// }

			if wait < 2*time.Second {
				wait = 2 * time.Second
			}
			wait = wait * 2
		}
	}
	return nil, err
}

func (bd *BlockDownloader) getBlock(ctx context.Context, blockID int) (*starknet.StateUpdateWithBlock, error) {
	queryURL := bd.buildQueryString("get_state_update", map[string]string{
		"blockNumber":  fmt.Sprint(blockID),
		"includeBlock": fmt.Sprintf("%t", true),
	})
	fmt.Println("Getting a block #", blockID, "URL", queryURL)
	body, err := bd.getRequest(ctx, queryURL)
	if err != nil {
		fmt.Println("Error getting a block: ", err)
		return nil, err
	}

	defer body.Close()

	stateUpdate := new(starknet.StateUpdateWithBlock)
	if err := json.NewDecoder(body).Decode(stateUpdate); err != nil {
		return nil, err
	}

	// b, _ := json.Marshal(stateUpdate)
	// fmt.Println("Got block: ", blockID, string(b))
	return stateUpdate, nil
}

func (bd *BlockDownloader) getSignature(ctx context.Context, blockID int) (*starknet.Signature, error) {
	queryURL := bd.buildQueryString("get_signature", map[string]string{
		"blockNumber": fmt.Sprint(blockID),
	})

	fmt.Println("Getting a signature #", blockID, "URL", queryURL)
	body, err := bd.getRequest(ctx, queryURL)
	if err != nil {
		fmt.Println("Error getting a signature: ", err)
		return nil, err
	}

	defer body.Close()

	signature := new(starknet.Signature)
	if err := json.NewDecoder(body).Decode(signature); err != nil {
		return nil, err
	}

	return signature, nil
}

func (bd *BlockDownloader) storeBlocks(ctx context.Context, fromBlock int, toBlock int) (string, error) {
	if fromBlock > toBlock {
		fmt.Println("From block is bigger than toBlock")
		return "", nil
	}
	blocks := []starknet.StateUpdateWithBlock{}
	for i := fromBlock; i <= toBlock; i++ {
		block, err := bd.getBlock(ctx, i)
		if err != nil {
			fmt.Println("error:", err)
			return "", nil
		}
		blocks = append(blocks, *block)
	}
	filePath := fmt.Sprintf("blocks_%d_%d.json", fromBlock, toBlock)
	data, err := json.Marshal(blocks)
	if err != nil {

		fmt.Println("error:", err)
		return "", err
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		fmt.Println("error:", err)
		return "", err
	}
	return filePath, nil
}

func (bd *BlockDownloader) storeSignatures(ctx context.Context, fromBlock int, toBlock int) (string, error) {
	if fromBlock > toBlock {
		fmt.Println("From block is bigger than toBlock")
		return "", nil
	}
	signatures := []starknet.Signature{}
	for i := fromBlock; i <= toBlock; i++ {
		sig, err := bd.getSignature(ctx, i)
		if err != nil {
			fmt.Println("error:", err)
			return "", nil
		}
		signatures = append(signatures, *sig)
	}
	filePath := fmt.Sprintf("signatures_%d_%d.json", fromBlock, toBlock)
	data, err := json.Marshal(signatures)
	if err != nil {

		fmt.Println("error:", err)
		return "", err
	}

	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		fmt.Println("error:", err)
		return "", err
	}
	return filePath, nil
}

func main() {
	// https://feeder.alpha-sepolia.starknet.io/feeder_gateway/get_state_update?blockNumber=439&includeBlock=true
	feederURL := "https://feeder.alpha-sepolia.starknet.io/feeder_gateway/"
	bd := NewBlockDownloader(feederURL)
	ctx := context.Background()
	fromBlock := 0
	toBlock := 1000
	bd.storeBlocks(ctx, fromBlock, toBlock)
	bd.storeSignatures(ctx, fromBlock, toBlock)
	fmt.Println("Hello, World!")
}
