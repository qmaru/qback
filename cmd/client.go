package cmd

import (
	"fmt"
	"log"
	"time"

	"qback/grpc/client"
	"qback/utils"

	"github.com/spf13/cobra"
)

var (
	sourceTag      string
	sourceFile     string
	metatimeout    int
	connecttimeout int
	fileChunk      int
	ClientRoot     = &cobra.Command{
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
				ServerAddress:  ServiceAddress,
				ConnectTimeout: connecttimeout,
				MetaTimeout:    metatimeout,
				Secure:         ServiceWithSecure,
				Chunksize:      fileChunk,
			}

			result, err := qClient.UploadFile(sourceTag, sourceFile)
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
				ServerAddress:  ServiceAddress,
				ConnectTimeout: connecttimeout,
				MetaTimeout:    metatimeout,
				Secure:         ServiceWithSecure,
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

	listCmd = &cobra.Command{
		Use:   "list",
		Short: "List server files",
		Run: func(cmd *cobra.Command, args []string) {
			qClient := client.ClientBasic{
				ServerAddress:  ServiceAddress,
				ConnectTimeout: connecttimeout,
				MetaTimeout:    metatimeout,
				Secure:         ServiceWithSecure,
			}

			files, err := qClient.ListFiles(sourceTag)
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("Server list under %s tag\n", sourceTag)
			if len(files) == 0 {
				log.Println("No files found")
				return
			}

			result, err := utils.JSONSuite.Json.MarshalIndent(files, "", "  ")
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(result))
		},
	}
)

func init() {
	ClientRoot.PersistentFlags().IntVarP(&connecttimeout, "ct", "", 10, "Connect Timeout")
	ClientRoot.PersistentFlags().IntVarP(&metatimeout, "mt", "", 30, "Metadata Timeout")
	ClientRoot.PersistentFlags().IntVarP(&fileChunk, "chunksize", "c", 1048576, "File chunksize [byte]")

	transferCmd.Flags().StringVarP(&sourceTag, "tag", "t", "", "Source tag")
	transferCmd.Flags().StringVarP(&sourceFile, "file", "f", "", "Source file")
	transferCmd.MarkFlagRequired("tag")
	transferCmd.MarkFlagRequired("file")

	listCmd.Flags().StringVarP(&sourceTag, "tag", "t", "", "Source tag")
	listCmd.MarkFlagRequired("tag")

	ClientRoot.AddCommand(transferCmd)
	ClientRoot.AddCommand(pingCmd)
	ClientRoot.AddCommand(listCmd)
}
