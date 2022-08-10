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
	typeName string
}

//go:generate go run main.go
func main() {
	contracts := []contract{
		{
			abiName:  "./starknet_abi.json",
			typeName: "Starknet",
		},
		{
			abiName:  "./gps_verifier_abi.json",
			typeName: "GpsStatementVerifier",
		},
		{
			abiName:  "./memory_pages_abi.json",
			typeName: "MemoryPageFactRegistry",
		},
	}

	for _, contract := range contracts {
		// Read in ABI
		abi, err := os.ReadFile(contract.abiName)
		if err != nil {
			fatal("Failed to read input ABI: %v", err)
		}

		// Generate code
		code, err := bind.Bind([]string{contract.typeName}, []string{string(abi)}, []string{""}, nil, "contracts", bind.LangGo, nil, nil)
		if err != nil {
			fatal("Failed to generate ABI binding: %v", err)
		}

		// Write code to file
		path := filepath.Join("../../../", snakeCase(contract.typeName)+".go")
		if err := os.WriteFile(path, []byte(code), 0o600); err != nil {
			fatal("Failed to write ABI binding: %v", err)
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

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, msg, err)
	os.Exit(1)
}
