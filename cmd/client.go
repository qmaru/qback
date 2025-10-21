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
	reverse      bool
	remoteTag    string
	localFile    string
	localDir     string
	chunkTimeout int
	fileChunk    int
	ClientRoot   = &cobra.Command{
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
			if reverse {
				if localDir == "" {
					log.Fatal("Error: --dir flag is required when using --reverse")
				}
			} else {
				if localFile == "" {
					log.Fatal("Error: --file flag is required for normal transfer")
				}
			}

			qClient := client.ClientBasic{
				ServerAddress: ServiceAddress,
				ChunkTimeout:  chunkTimeout,
				Secure:        ServiceWithSecure,
				Chunksize:     fileChunk,
			}

			if reverse {
				log.Printf("Starting reverse transfer: server to client\n")
				return
			}

			log.Printf("Starting transfer: client to server\n")
			result, err := qClient.UploadFile(remoteTag, localFile)
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
				ChunkTimeout:  chunkTimeout,
				Secure:        ServiceWithSecure,
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
				ServerAddress: ServiceAddress,
				Secure:        ServiceWithSecure,
			}

			files, err := qClient.ListFiles(remoteTag)
			if err != nil {
				log.Fatal(err)
			}

			log.Printf("Server list under %s tag\n", remoteTag)
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
	ClientRoot.PersistentFlags().IntVarP(&chunkTimeout, "ct", "", 10, "Connect Timeout")
	ClientRoot.PersistentFlags().IntVarP(&fileChunk, "chunksize", "c", 1048576, "File chunksize [byte]")

	transferCmd.Flags().StringVarP(&remoteTag, "tag", "t", "", "Remote tag")
	transferCmd.Flags().StringVarP(&localFile, "file", "f", "", "Local file")
	transferCmd.Flags().StringVarP(&localDir, "dir", "d", "", "Local directory")
	transferCmd.Flags().BoolVarP(&reverse, "reverse", "r", false, "Reverse transfer (server to client)")
	transferCmd.MarkFlagRequired("tag")

	listCmd.Flags().StringVarP(&remoteTag, "tag", "t", "", "Source tag")
	listCmd.MarkFlagRequired("tag")

	ClientRoot.AddCommand(transferCmd)
	ClientRoot.AddCommand(pingCmd)
	ClientRoot.AddCommand(listCmd)
}
