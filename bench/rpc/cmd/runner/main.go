package main

import (
	"fmt"
	"os"
)

func main() {
	config, err := loadConfig(os.LookupEnv, os.ReadFile, nowUTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	r := newRunner(config, os.Stdout, os.Stderr)
	os.Exit(r.run())
}
