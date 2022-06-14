package rpc

import (
	"io"
	"math/big"
	"net/http"
	"net/url"
)

func getFullContract(addr, hash *big.Int) ([]byte, error) {
	// TODO: Get base URL from config.
	url, err := url.Parse("http://alpha4.starknet.io/feeder_gateway/get_full_contract")
	if err != nil {
		return nil, err
	}

	vals := url.Query()
	vals.Add("contractAddress", "0x"+addr.Text(16))
	vals.Add("blockHash", "0x"+hash.Text(16))

	url.RawQuery = vals.Encode()

	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
