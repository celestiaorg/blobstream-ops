package root

import (
	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/replay"
	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/verify"
	"github.com/celestiaorg/blobstream-ops/cmd/blobstream-ops/version"
	"github.com/spf13/cobra"
)

// Cmd creates a new root command for the Blobstream-ops CLI. It is called once in the
// main function.
func Cmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "blobstream-ops",
		Short:        "The Blobstream OPS CLI",
		SilenceUsage: true,
	}

	rootCmd.AddCommand(
		version.Cmd,
		verify.Command(),
		replay.Command(),
	)

	rootCmd.SetHelpCommand(&cobra.Command{})

	return rootCmd
}
