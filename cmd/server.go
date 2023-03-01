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
			s := new(server.ServerBasic)
			s.ListenAddress = ServiceAddress
			s.Secure = ServiceWithSecure
			s.SavePath = savePath
			err := s.Run()
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
