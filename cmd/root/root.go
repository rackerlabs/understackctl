package root

import (
	"fmt"

	"github.com/rackerlabs/understackctl/cmd/deploy"
	"github.com/rackerlabs/understackctl/cmd/deviceType"
	"github.com/rackerlabs/understackctl/cmd/flavor"
	"github.com/rackerlabs/understackctl/cmd/nautobotOp"
	cmdversion "github.com/rackerlabs/understackctl/cmd/version"
	"github.com/rackerlabs/understackctl/cmd/workflow"
	"github.com/rackerlabs/understackctl/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "understackctl SUBCOMMAND ...",
	Short:   "UnderStack CLI",
	Long:    ``,
	Version: version.Get().String(),
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand, show help
		return fmt.Errorf("a subcommand is required")
	},
}

func init() {
	rootCmd.AddCommand(deploy.NewCmdDeploy())
	rootCmd.AddCommand(deviceType.NewCmdDeviceType())
	rootCmd.AddCommand(flavor.NewCmdFlavor())
	rootCmd.AddCommand(nautobotOp.NewCmdNautobotOp())
	rootCmd.AddCommand(workflow.NewCmdWorkflow())
	rootCmd.AddCommand(cmdversion.NewCmdVersion())
}

// Execute will execute the root command
func Execute() error {
	return rootCmd.Execute()
}
