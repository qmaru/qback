package server

import (
	"log"

	"qBack/grpc/server"

	"github.com/spf13/cobra"
)

var (
	address    string
	savePath   string
	secure     bool
	ServerRoot = &cobra.Command{
		Use:   "server",
		Short: "Run Server",
		Run: func(cmd *cobra.Command, args []string) {
			s := new(server.ServerBasic)
			if address == "" {
				address = "127.0.0.1:20000"
			}
			s.ListenAddress = address
			s.Secure = secure
			s.SavePath = savePath
			err := s.Run()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	ServerRoot.Flags().StringVarP(&address, "address", "a", "", "Listen Address (host:port)")
	ServerRoot.Flags().StringVarP(&savePath, "dir", "d", "", "Save Folder")
	ServerRoot.Flags().BoolVarP(&secure, "secure", "s", false, "With TLS")
	ServerRoot.MarkFlagRequired("dir")
}
