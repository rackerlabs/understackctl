package root

import (
	"fmt"

	"github.com/rackerlabs/understackctl/cmd/argocd"
	"github.com/rackerlabs/understackctl/cmd/certManager"
	"github.com/rackerlabs/understackctl/cmd/deploy"
	"github.com/rackerlabs/understackctl/cmd/deviceType"
	"github.com/rackerlabs/understackctl/cmd/dex"
	"github.com/rackerlabs/understackctl/cmd/flavor"
	"github.com/rackerlabs/understackctl/cmd/helmConfig"
	"github.com/rackerlabs/understackctl/cmd/nautobotOp"
	"github.com/rackerlabs/understackctl/cmd/node"
	"github.com/rackerlabs/understackctl/cmd/openstack"
	"github.com/rackerlabs/understackctl/cmd/other"
	"github.com/rackerlabs/understackctl/cmd/quickstart"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "understackctl SUBCOMMAND ...",
	Short: "UnderStack CLI",
	Long:  ``,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand, show help
		return fmt.Errorf("a subcommand is required")
	},
}

func init() {
	rootCmd.AddCommand(deploy.NewCmdDeploy())
	rootCmd.AddCommand(argocd.NewCmdArgocdSecret())
	rootCmd.AddCommand(certManager.NewCmdCertManagerSecret())
	rootCmd.AddCommand(deviceType.NewCmdDeviceType())
	rootCmd.AddCommand(dex.NewCmdDexSecrets())
	rootCmd.AddCommand(flavor.NewCmdFlavor())
	rootCmd.AddCommand(helmConfig.NewCmdHelmConfig())
	rootCmd.AddCommand(nautobotOp.NewCmdNautobotOp())
	rootCmd.AddCommand(node.NewCmdNode())
	rootCmd.AddCommand(openstack.NewCmdOpenstackSecrets())
	rootCmd.AddCommand(quickstart.NewCmdQuickStart())
	rootCmd.AddCommand(other.NewCmdOtherSecrets())
}

// SetVersion sets the version reported by `understackctl --version`.
func SetVersion(version, commit string) {
	rootCmd.Version = fmt.Sprintf("%s (%s)", version, commit)
}

// Execute will execute the root command
func Execute() error {
	return rootCmd.Execute()
}
