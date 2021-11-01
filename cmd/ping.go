package cmd

import (
	"qBack/client"

	"github.com/spf13/cobra"
)

var (
	serverAddress string
	pingCmd       = &cobra.Command{
		Use:   "ping",
		Short: "Ping Server",
		Run: func(cmd *cobra.Command, args []string) {
			client.PingServer(serverAddress)
		},
	}
)

func init() {
	pingCmd.Flags().StringVarP(&serverAddress, "address", "a", "", "Server Address")
	pingCmd.MarkFlagRequired("address")
}
