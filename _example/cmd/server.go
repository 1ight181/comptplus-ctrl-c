package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage servers",
	Run:   func(cmd *cobra.Command, args []string) { cmd.Usage() },
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all servers",
	Run: func(cmd *cobra.Command, args []string) {
		dc, _ := cmd.Flags().GetString("datacenter-id")
		fmt.Printf("Listing servers in datacenter %s\n", dc)
	},
}

var serverCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a server",
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		cores, _ := cmd.Flags().GetInt("cores")
		fmt.Printf("Creating server %q with %d cores\n", name, cores)
	},
}

var serverDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a server",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Server deleted")
	},
}

func init() {
	RootCmd.AddCommand(serverCmd)
	serverCmd.PersistentFlags().String("datacenter-id", "", "datacenter UUID")
	_ = serverCmd.RegisterFlagCompletionFunc("datacenter-id", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{
			"dc-1234-abcd\tUS East",
			"dc-5678-efgh\tEU Central",
			"dc-9012-ijkl\tAP South",
		}, cobra.ShellCompDirectiveNoFileComp
	})

	serverCmd.AddCommand(serverListCmd)
	serverListCmd.Flags().StringP("output", "o", "table", "output format")
	_ = serverListCmd.RegisterFlagCompletionFunc("output", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"json", "table", "text"}, cobra.ShellCompDirectiveNoFileComp
	})

	serverCmd.AddCommand(serverCreateCmd)
	serverCreateCmd.Flags().String("name", "", "server name")
	serverCreateCmd.Flags().Int("cores", 1, "number of CPU cores")
	serverCreateCmd.Flags().String("ram", "1024", "RAM in MB")

	serverCmd.AddCommand(serverDeleteCmd)
	serverDeleteCmd.Flags().String("server-id", "", "server UUID")
}
