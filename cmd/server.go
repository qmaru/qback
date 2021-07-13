package cmd

import (
	"qBack/server"

	"github.com/spf13/cobra"
)

var (
	listenAddress string
	rootPath      string
	serverCmd     = &cobra.Command{
		Use:   "server",
		Short: "Run Server",
		Run: func(cmd *cobra.Command, args []string) {
			server.Run(listenAddress, rootPath)
		},
	}
)

func init() {
	serverCmd.Flags().StringVarP(&listenAddress, "address", "a", "", "Listen Address (host:port)")
	serverCmd.Flags().StringVarP(&rootPath, "dir", "d", "", "Save Folder")
}
