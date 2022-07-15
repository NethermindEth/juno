package main

import (
	"fmt"

	"github.com/consensys/gnark-crypto/field"
	"github.com/consensys/gnark-crypto/field/generator"
)

//go:generate go run main.go
func main() {
	const modulus = "3618502788666131213697322783095070105623107215331596699973092056135872020481"
	felt, err := field.NewField("felt", "Felt", modulus, false)
	if err != nil {
		panic(err)
	}
	if err := generator.GenerateFF(felt, "../../../"); err != nil {
		panic(err)
	}
	fmt.Println("successfully generated felt")
}
