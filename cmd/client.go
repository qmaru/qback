package cmd

import (
	"qBack/client"

	"github.com/spf13/cobra"
)

var (
	fileTag        string
	filePath       string
	connectAddress string
	clientCmd      = &cobra.Command{
		Use:   "client",
		Short: "Run Client",
		Run: func(cmd *cobra.Command, args []string) {
			client.Run(connectAddress, fileTag, filePath)
		},
	}
)

func init() {
	clientCmd.Flags().StringVarP(&connectAddress, "address", "a", "", "Server Address")
	clientCmd.Flags().StringVarP(&fileTag, "tag", "t", "", "File Tag")
	clientCmd.Flags().StringVarP(&filePath, "path", "p", "", "File Path")
	clientCmd.MarkFlagRequired("tag")
	clientCmd.MarkFlagRequired("path")
}
