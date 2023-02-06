package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompressor(t *testing.T) {
	data := `{
		"prime": "0x800000000000011000000000000000000000000000000000000000000000001",
    	"main_scope": "__main__",
		"compiler_version": "0.10.1",
		"debug_info": null,
		"reference_manager": {
			"references": [
				{
					"ap_tracking_data": {
						"offset": 0,
						"group": 1
					},
					"pc": 3,
					"value": "[cast(fp + (-3), felt*)]"
				},
			]
		}
	}`
	compressedData, err := Compress([]byte(data))
	assert.NoError(t, err)

	decompressedData, err := Decompress(compressedData)
	assert.NoError(t, err)
	assert.Equal(t, data, string(decompressedData))
}
