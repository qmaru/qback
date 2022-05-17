package cmd

import (
	"fmt"
	"os"

	"qBack/cmd/client"
	"qBack/cmd/server"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:     "qBack",
		Short:   "qBack is a File Transfer Service",
		Version: "1.0-220517",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(
		server.ServerRoot,
		client.ClientRoot,
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
