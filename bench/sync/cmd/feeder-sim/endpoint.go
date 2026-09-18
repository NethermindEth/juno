package main

import (
	"fmt"
	"net/url"
)

type endpoint[K, F comparable] struct {
	name  string
	fixed F
}

var (
	contractAddresses = &endpoint[struct{}, struct{}]{name: "get_contract_addresses"}
	block             = &endpoint[blockKey, headerOnly]{
		name:  "get_block",
		fixed: headerOnly{HeaderOnly: true},
	}
	stateUpdate = &endpoint[blockKey, withBlockAndSignature]{
		name:  "get_state_update",
		fixed: withBlockAndSignature{IncludeBlock: true, IncludeSignature: true},
	}
	classByHash = &endpoint[classKey, atLatest]{
		name:  "get_class_by_hash",
		fixed: atLatest{BlockNumber: "latest"},
	}
	compiledClass = &endpoint[classKey, atLatest]{
		name:  "get_compiled_class_by_class_hash",
		fixed: atLatest{BlockNumber: "latest"},
	}
)

const preConfirmedBlockRoute = "get_preconfirmed_block"

type resource struct {
	name  string
	file  string
	query url.Values
}

func (endpoint *endpoint[K, F]) resource(key K) (resource, error) {
	keyValues, err := encode(key)
	if err != nil {
		return resource{}, err
	}
	fixedValues, err := encode(endpoint.fixed)
	if err != nil {
		return resource{}, err
	}

	file := endpoint.name
	query := url.Values{}
	for name, value := range keyValues {
		file += "/" + value
		query.Set(name, value)
	}
	for name, value := range fixedValues {
		query.Set(name, value)
	}
	return resource{name: endpoint.name, file: file + ".json.gz", query: query}, nil
}

func (resource resource) url(feederURL *url.URL) *url.URL {
	requestURL := feederURL.JoinPath(resource.name)
	requestURL.RawQuery = resource.query.Encode()
	return requestURL
}

func (endpoint *endpoint[K, F]) checkFixed(values url.Values) error {
	fixed, err := decode[F](values)
	if err != nil {
		return fmt.Errorf("%s: %w", endpoint.name, err)
	}
	if fixed != endpoint.fixed {
		return fmt.Errorf("%s: unsupported query %q", endpoint.name, values.Encode())
	}
	return nil
}

func (endpoint *endpoint[K, F]) key(values url.Values) (K, error) {
	key, err := decode[K](values)
	if err != nil {
		return key, fmt.Errorf("%s: %w", endpoint.name, err)
	}
	return key, nil
}
