package main

import (
	"os"

	"secret-manager/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
