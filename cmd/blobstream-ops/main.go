package main

import (
	"context"
	"os"

	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/root"
)

func main() {
	rootCmd := root.Cmd()
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}
