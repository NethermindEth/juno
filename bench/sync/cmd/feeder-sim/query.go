package main

import (
	"fmt"
	"net/url"

	"github.com/mitchellh/mapstructure"
)

type (
	blockKey struct {
		BlockNumber uint64 `mapstructure:"blockNumber"`
	}
	classKey struct {
		ClassHash string `mapstructure:"classHash"`
	}
	headerOnly struct {
		HeaderOnly bool `mapstructure:"headerOnly"`
	}
	withBlockAndSignature struct {
		IncludeBlock     bool `mapstructure:"includeBlock"`
		IncludeSignature bool `mapstructure:"includeSignature"`
	}
	atLatest struct {
		BlockNumber string `mapstructure:"blockNumber"`
	}
)

func decode[T any](values url.Values) (T, error) {
	flat := make(map[string]string, len(values))
	for name := range values {
		flat[name] = values.Get(name)
	}

	var result T
	if err := mapstructure.WeakDecode(flat, &result); err != nil {
		return result, err
	}
	return result, nil
}

func encode[T any](value T) (map[string]string, error) {
	fields := make(map[string]any)
	if err := mapstructure.Decode(value, &fields); err != nil {
		return nil, err
	}

	encoded := make(map[string]string, len(fields))
	for name, field := range fields {
		encoded[name] = fmt.Sprint(field)
	}
	return encoded, nil
}
