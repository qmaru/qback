package cmd

import (
	"log"

	"qBack/grpc/server"

	"github.com/spf13/cobra"
)

var (
	savePath   string
	ServerRoot = &cobra.Command{
		Use:   "server",
		Short: "Run Server",
		Run: func(cmd *cobra.Command, args []string) {
			qServer := server.ServerBasic{
				ListenAddress: ServiceAddress,
				Secure:        ServiceWithSecure,
				SavePath:      savePath,
				Debug:         Debug,
			}
			err := qServer.Run()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	ServerRoot.Flags().StringVarP(&savePath, "dir", "d", "", "Save Folder")
	ServerRoot.MarkFlagRequired("dir")
}
