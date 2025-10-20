package cmd

import (
	"log"
	"time"

	"qback/grpc/client"

	"github.com/spf13/cobra"
)

var (
	sourceTag  string
	sourceFile string
	timeout    int
	fileChunk  int
	ClientRoot = &cobra.Command{
		Use:   "client",
		Short: "Run Client",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	transferCmd = &cobra.Command{
		Use:   "transfer",
		Short: "Transfer file",
		Run: func(cmd *cobra.Command, args []string) {
			qClient := client.ClientBasic{
				ServerAddress: ServiceAddress,
				Timeout:       1800,
				Secure:        ServiceWithSecure,
				Chunksize:     fileChunk,
				Debug:         Debug,
			}

			result, err := qClient.FileStream(sourceTag, sourceFile)
			if err != nil {
				log.Fatal(err)
			}
			log.Println(result)
		},
	}
	pingCmd = &cobra.Command{
		Use:   "ping",
		Short: "Ping Server",
		Run: func(cmd *cobra.Command, args []string) {
			startTime := time.Now().UnixMilli()

			qClient := client.ClientBasic{
				ServerAddress: ServiceAddress,
				Timeout:       timeout,
				Secure:        ServiceWithSecure,
				Debug:         Debug,
			}

			err := qClient.ServerCheck()
			if err != nil {
				log.Printf("Server is down: %s\n", err.Error())
			} else {
				endTime := time.Now().UnixMilli()
				delay := endTime - startTime
				log.Printf("Server is up [%d ms]\n", delay)
			}
		},
	}
)

func init() {
	ClientRoot.PersistentFlags().IntVarP(&timeout, "timeout", "", 1800, "Timeout")
	ClientRoot.PersistentFlags().IntVarP(&fileChunk, "chunksize", "c", 1048576, "File chunksize [byte]")

	transferCmd.Flags().StringVarP(&sourceTag, "tag", "t", "", "Source tag")
	transferCmd.Flags().StringVarP(&sourceFile, "file", "f", "", "Source file")
	transferCmd.MarkFlagRequired("tag")
	transferCmd.MarkFlagRequired("file")

	ClientRoot.AddCommand(transferCmd)
	ClientRoot.AddCommand(pingCmd)
}
