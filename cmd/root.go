package cmd

import (
	"fmt"
	"os"

	"qback/utils"

	"github.com/spf13/cobra"
)

var (
	ServiceAddress    string
	ServiceWithSecure bool
	ServiceDebug      bool
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "qback",
		Short:   "qback is a File Transfer Service",
		Version: utils.VERSION,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.PersistentFlags().StringVarP(&ServiceAddress, "address", "a", "127.0.0.1:20000", "Server Address")
	cmd.PersistentFlags().BoolVarP(&ServiceWithSecure, "secure", "s", false, "With TLS")
	cmd.PersistentFlags().BoolVarP(&ServiceDebug, "debug", "d", false, "Enable debug mode")

	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.AddCommand(NewServer(), NewClient())

	return cmd
}

func Execute() {
	if err := NewCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
