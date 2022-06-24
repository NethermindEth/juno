package services

/*
// getFullContract retrieves the compiled contract at addr from the
// feeder gateway and returns an error otherwise.
func getFullContract(addr *big.Int) ([]byte, error) {
	// TODO: Get base URL from config.
	url, err := url.Parse("http://alpha4.starknet.io/feeder_gateway/get_full_contract")
	if err != nil {
		return nil, err
	}

	vals := url.Query()
	vals.Add("contractAddress", "0x"+addr.Text(16))
	url.RawQuery = vals.Encode()

	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
*/
