package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const directoryMode = 0o750

type dataset struct {
	root string
}

func (dataset dataset) path(file string) string {
	return filepath.Join(dataset.root, file)
}

func (dataset dataset) read(file string) ([]byte, error) {
	return os.ReadFile(dataset.path(file))
}

func (dataset dataset) write(file string, gzipped []byte) error {
	target := dataset.path(file)
	if err := os.MkdirAll(filepath.Dir(target), directoryMode); err != nil {
		return err
	}

	temporary, err := writeTemporary(target, gzipped)
	if err != nil {
		return err
	}
	return os.Rename(temporary, target)
}

func writeTemporary(target string, data []byte) (name string, err error) {
	file, err := os.CreateTemp(filepath.Dir(target), filepath.Base(target)+".*.tmp")
	if err != nil {
		return "", err
	}
	defer func() {
		err = errors.Join(err, file.Close())
		if err != nil {
			err = errors.Join(err, os.Remove(file.Name()))
		}
	}()

	if _, err = file.Write(data); err != nil {
		return "", err
	}
	if err = file.Sync(); err != nil {
		return "", err
	}
	return file.Name(), nil
}

func gzipBytes(body []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(body); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func gunzip(gzipped []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(gzipped))
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return body, reader.Close()
}

func unmarshalGzipped[T any](gzipped []byte) (T, error) {
	var value T
	body, err := gunzip(gzipped)
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(body, &value)
	return value, err
}
