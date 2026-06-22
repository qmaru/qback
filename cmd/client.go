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
	clientChunkTimeout int
	clientFileChunk    int
)

func NewClient() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "client",
		Short: "Run Client",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmd.PersistentFlags().IntVarP(&clientChunkTimeout, "ct", "", 15, "Connect Timeout")
	cmd.PersistentFlags().IntVarP(&clientFileChunk, "chunksize", "c", 1048576, "File chunksize [byte]")

	cmd.AddCommand(NewCheckSubCmd())
	cmd.AddCommand(NewTransferSubCmd())
	cmd.AddCommand(NewListSubCmd())

	return cmd
}

func NewCheckSubCmd() *cobra.Command {
	var timeout int

	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Ping Server",
		Run: func(cmd *cobra.Command, args []string) {
			startTime := time.Now().UnixMilli()

			qClient := client.ClientBasic{
				ServerAddress: ServiceAddress,
				ChunkTimeout:  clientChunkTimeout,
				Secure:        ServiceWithSecure,
				Debug:         ServiceDebug,
			}

			err := qClient.ServerCheck(timeout)
			if err != nil {
				log.Printf("Server is down: %s\n", err.Error())
			} else {
				endTime := time.Now().UnixMilli()
				delay := endTime - startTime
				log.Printf("Server is up [%d ms]\n", delay)
			}
		},
	}

	cmd.Flags().IntVarP(&timeout, "timeout", "t", 10, "Timeout for server check [second]")

	return cmd
}

func NewTransferSubCmd() *cobra.Command {
	var reverse bool
	var remoteTag string
	var remoteName string
	var localFile string
	var localDir string

	cmd := &cobra.Command{
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
				ChunkTimeout:  clientChunkTimeout,
				Secure:        ServiceWithSecure,
				Chunksize:     clientFileChunk,
				Debug:         ServiceDebug,
			}

			if reverse {
				log.Printf("Starting reverse transfer: server to client\n")
				result, err := qClient.DownloadFile(remoteTag, remoteName, localDir)
				if err != nil {
					log.Fatal(err)
				}
				log.Println(result)
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

	cmd.Flags().StringVarP(&remoteTag, "tag", "t", "", "Remote tag")
	cmd.Flags().StringVarP(&remoteName, "name", "n", "", "Remote file name")
	cmd.Flags().StringVarP(&localFile, "file", "f", "", "Local file")
	cmd.Flags().StringVarP(&localDir, "dir", "d", "", "Local directory")
	cmd.Flags().BoolVarP(&reverse, "reverse", "r", false, "Reverse transfer (server to client)")
	cmd.MarkFlagRequired("tag")

	return cmd

}

func NewListSubCmd() *cobra.Command {
	var remoteTag string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List server files",
		Run: func(cmd *cobra.Command, args []string) {
			qClient := client.ClientBasic{
				ServerAddress: ServiceAddress,
				Secure:        ServiceWithSecure,
				Debug:         ServiceDebug,
			}

			files, err := qClient.ListFiles(remoteTag)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Printf(">> tag=%s\n", remoteTag)
			if len(files) == 0 {
				log.Println("No files found")
				return
			}

			for _, file := range files {
				fmt.Printf(
					"%-24s  %10s  %-12s  %s\n",
					file.GetName(),
					utils.PrettySize(file.GetSize()),
					utils.PrettyHash(file.GetHash()),
					time.Unix(file.GetModifiedTime(), 0).Format("2006-01-02 15:04"),
				)
			}
			fmt.Println("<<")
		},
	}

	cmd.Flags().StringVarP(&remoteTag, "tag", "t", "", "Source tag")
	cmd.MarkFlagRequired("tag")

	return cmd
}
