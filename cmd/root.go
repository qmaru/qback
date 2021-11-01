package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:     "qBack",
		Short:   "qBack is a File Transfer Service",
		Version: "1.0-211101",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)

func AddressChecker(address string) (string, bool) {
	if strings.Contains(address, ":") {
		return address, true
	}
	return address, false
}

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(
		serverCmd,
		clientCmd,
		pingCmd,
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
