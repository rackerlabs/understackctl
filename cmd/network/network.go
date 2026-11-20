package network

import "github.com/spf13/cobra"

func NewCmdNetwork() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "Network topology and mapping utilities",
		Long:  "Query and map network topology, showing relationships between OpenStack networks, servers, and physical switch ports",
	}

	cmd.AddCommand(newCmdMap())

	return cmd
}
