package main

import (
	"os"
)

func main() {
	config := loadConfig(os.LookupEnv, os.ReadFile, nowUTC())
	r := newRunner(config, os.Stdout, os.Stderr)
	os.Exit(r.run())
}
