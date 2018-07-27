package main

import (
	"os"

	"github.com/rcowham/p4dbeat/cmd"

	_ "github.com/rcowham/p4dbeat/include"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
