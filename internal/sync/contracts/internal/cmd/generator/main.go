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

var (
	matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

//go:generate go run main.go
func main() {
	contracts := []contract{
		{
			abiName:  "./starknet.json",
			binName:  "./starknet.bin",
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
		{
			abiName:  "./cairo_bootloader_program.json",
			binName:  "./cairo_bootloader_program.bin",
			typeName: "CairoBootloaderProgram",
		},
		{
			abiName:  "./cairo_verifier.json",
			binName:  "./cairo_verifier.bin",
			typeName: "CairoVerifier",
		},
		{
			abiName:  "./cpu_constraint_poly.json",
			binName:  "./cpu_constraint_poly.bin",
			typeName: "CpuConstraintPoly",
		},
		{
			abiName:  "./pedersen_hash_points_x_column.json",
			binName:  "./pedersen_hash_points_x_column.bin",
			typeName: "PedersenHashPointsXColumn",
		},
		{
			abiName:  "./pedersen_hash_points_y_column.json",
			binName:  "./pedersen_hash_points_y_column.bin",
			typeName: "PedersenHashPointsYColumn",
		},
		{
			abiName:  "./ecdsa_points_x_column.json",
			binName:  "./ecdsa_points_x_column.bin",
			typeName: "EcdsaPointsXColumn",
		},
		{
			abiName:  "./ecdsa_points_y_column.json",
			binName:  "./ecdsa_points_y_column.bin",
			typeName: "EcdsaPointsYColumn",
		},
		{
			abiName:  "./merkle_statement.json",
			binName:  "./merkle_statement.bin",
			typeName: "MerkleStatement",
		},
		{
			abiName:  "./cpu_odds.json",
			binName:  "./cpu_oods.bin",
			typeName: "CpuOods",
		},
		{
			abiName:  "./fri_statement.json",
			binName:  "./fri_statement.bin",
			typeName: "FriStatement",
		},
	}

	for _, contract := range contracts {
		// Read in ABI
		abi, err := os.ReadFile(contract.abiName)
		if err != nil {
			fatal(fmt.Errorf("failed to read input ABI: %w", err))
		}
		bin, err := os.ReadFile(contract.binName)
		if err != nil {
			fatal(fmt.Errorf("failed to read input bin: %w", err))
		}

		// Generate code
		code, err := bind.Bind([]string{contract.typeName}, []string{string(abi)}, []string{string(bin)}, nil, "contracts", bind.LangGo, nil, nil)
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
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func fatal(err error) {
	fmt.Fprint(os.Stderr, err.Error())
	os.Exit(1)
}
