package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
)

func Gzip64Encode(data *[]byte) (string, error) {
	var compressedBuffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedBuffer)
	if _, err := gzipWriter.Write(*data); err != nil {
		return "", err
	}
	if err := gzipWriter.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(compressedBuffer.Bytes()), nil
}
