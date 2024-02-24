package utils

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
)

func Gzip64Encode(data []byte) (string, error) {
	var compressedBuffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressedBuffer)
	if _, err := gzipWriter.Write(data); err != nil {
		return "", fmt.Errorf("gzip data: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return "", fmt.Errorf("close gzip writer: %v", err)
	}
	return base64.StdEncoding.EncodeToString(compressedBuffer.Bytes()), nil
}

func Gzip64Decode(data string) ([]byte, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(decodedBytes))
	if err != nil {
		return nil, err
	}
	decompressedBytes, err := io.ReadAll(gzipReader)
	if err != nil {
		return nil, err
	}
	err = gzipReader.Close()
	if err != nil {
		return nil, err
	}
	return decompressedBytes, nil
}
