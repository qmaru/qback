package cmd

import (
	"log"

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
			if connectAddress != "" {
				if a, ok := AddressChecker(connectAddress); ok {
					client.Run(connectAddress, fileTag, filePath)
				} else {
					log.Printf("%s is Error HOST:PORT", a)
				}
			} else {
				client.Run(connectAddress, fileTag, filePath)
			}
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
