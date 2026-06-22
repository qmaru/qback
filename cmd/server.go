package cmd

import (
	"context"
	"log"

	"qback/grpc/server"

	"github.com/spf13/cobra"
)

func NewServer() *cobra.Command {
	var savePath string
	var memoryMode bool

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run Server",
		Run: func(cmd *cobra.Command, args []string) {
			if !memoryMode && savePath == "" {
				log.Fatal("flag required: --output (-o) is required when memory mode is disabled")
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			qServer := server.ServerBasic{
				ListenAddress: ServiceAddress,
				Secure:        ServiceWithSecure,
				SavePath:      savePath,
				MemoryMode:    memoryMode,
				Debug:         ServiceDebug,
			}

			if err := qServer.Run(ctx); err != nil {
				log.Fatal(err)
			}
		},
	}

	cmd.Flags().StringVarP(&savePath, "output", "o", "", "Output Directory")
	cmd.Flags().BoolVarP(&memoryMode, "memory", "m", false, "Memory Mode")

	return cmd
}
