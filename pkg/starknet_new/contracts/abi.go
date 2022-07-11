package abi

import _ "embed"

//go:embed gps_verifier_abi.json
var GpsVerifierAbiJson string

//go:embed memory_pages_abi.json
var MemoryPagesAbiJson string

//go:embed starknet_abi.json
var StarknetAbiJson string
