package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
)

type contract struct {
	abiName  string
	binName  string
	typeName string
}

//go:generate go run main.go
func main() {
	contracts := []contract{
		{
			abiName:  "./starknet.json",
			typeName: "Starknet",
		},
		{
			abiName:  "./gps_statement_verifier.json",
			binName:  "./gps_statement_verifier.bin",
			typeName: "GpsStatementVerifier",
		},
		{
			abiName:  "./memory_page_fact_registry.json",
			binName:  "./memory_page_fact_registry.bin",
			typeName: "MemoryPageFactRegistry",
		},
	}

	for _, contract := range contracts {
		// Read in ABI
		abi, err := os.ReadFile(contract.abiName)
		if err != nil {
			fatal(fmt.Errorf("failed to read input ABI: %w", err))
		}

		// Generate code
		code, err := bind.Bind([]string{contract.typeName}, []string{string(abi)}, []string{""}, nil, "contracts", bind.LangGo, nil, nil)
		if err != nil {
			fatal(fmt.Errorf("failed to generate ABI binding: %w", err))
		}

		// Write code to file
		path := filepath.Join("../../../", snakeCase(contract.typeName)+".go")
		if err := os.WriteFile(path, []byte(code), 0o600); err != nil {
			fatal(fmt.Errorf("failed to write ABI binding: %w", err))
		}
	}
}

func snakeCase(str string) string {
	matchFirstCap := regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap := regexp.MustCompile("([a-z0-9])([A-Z])")
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func fatal(err error) {
	fmt.Fprint(os.Stderr, err.Error())
	os.Exit(1)
}
