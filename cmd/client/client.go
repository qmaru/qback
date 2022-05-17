package client

import (
	"log"

	"qBack/grpc/client"

	"github.com/spf13/cobra"
)

var (
	sourceTag  string
	sourceFile string
	address    string
	secure     bool
	timeout    int
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
			c := new(client.ClientBasic)
			if address == "" {
				address = "127.0.0.1:20000"
			}
			c.ServerAddress = address
			c.Timeout = 1800
			c.Secure = secure
			result, err := c.FileStream(sourceTag, sourceFile)
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
			c := new(client.ClientBasic)
			if address == "" {
				address = "127.0.0.1:20000"
			}
			c.ServerAddress = address
			c.Timeout = timeout
			c.Secure = secure
			err := c.ServerCheck()
			if err != nil {
				log.Printf("Server is down: %s\n", err.Error())
			} else {
				log.Println("Server is up")
			}
		},
	}
)

func init() {
	ClientRoot.PersistentFlags().StringVarP(&address, "address", "a", "", "Server Address")
	ClientRoot.PersistentFlags().BoolVarP(&secure, "secure", "s", false, "With TLS")
	ClientRoot.PersistentFlags().IntVarP(&timeout, "timeout", "", 1800, "Timeout")

	transferCmd.Flags().StringVarP(&sourceTag, "tag", "t", "", "Source tag")
	transferCmd.Flags().StringVarP(&sourceFile, "file", "f", "", "Source file")
	transferCmd.MarkFlagRequired("tag")
	transferCmd.MarkFlagRequired("file")

	ClientRoot.AddCommand(transferCmd)
	ClientRoot.AddCommand(pingCmd)
}
