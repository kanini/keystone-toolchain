package main

import (
	"os"

	"github.com/kanini/keystone-toolchain/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
