package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	Debug             bool
	ServiceAddress    string
	ServiceWithSecure bool
	rootCmd           = &cobra.Command{
		Use:     "qBack",
		Short:   "qBack is a File Transfer Service",
		Version: VERSION,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
)

func Execute() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(
		ServerRoot,
		ClientRoot,
	)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&Debug, "debug", "", false, "Debug mode")
	rootCmd.PersistentFlags().StringVarP(&ServiceAddress, "address", "a", "127.0.0.1:20000", "Server Address")
	rootCmd.PersistentFlags().BoolVarP(&ServiceWithSecure, "secure", "s", false, "With TLS")
}
