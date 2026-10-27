package main

import (
	"os"

	"github.com/rackerlabs/understackctl/cmd/root"
)

func main() {
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
