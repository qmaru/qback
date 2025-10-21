package cmd

import (
	"log"

	"qback/grpc/server"

	"github.com/spf13/cobra"
)

var (
	savePath   string
	memoryMode bool
	ServerRoot = &cobra.Command{
		Use:   "server",
		Short: "Run Server",
		Run: func(cmd *cobra.Command, args []string) {
			if !memoryMode && savePath == "" {
				log.Fatal("flag required: --dir (-d) is required when memory mode is disabled")
			}

			qServer := server.ServerBasic{
				ListenAddress: ServiceAddress,
				Secure:        ServiceWithSecure,
				SavePath:      savePath,
				MemoryMode:    memoryMode,
			}
			err := qServer.Run()
			if err != nil {
				log.Fatal(err)
			}
		},
	}
)

func init() {
	ServerRoot.Flags().StringVarP(&savePath, "dir", "d", "", "Save Directory")
	ServerRoot.Flags().BoolVarP(&memoryMode, "memory", "m", false, "Memory Mode")
}
