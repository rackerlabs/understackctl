package workflow

import (
	"fmt"

	"github.com/spf13/cobra"
)

const inspectWorkflowTemplate = "wftmpl/inspect-server"

type inspectOptions struct {
	Logs bool
}

func newCmdInspectServer() *cobra.Command {
	opts := &inspectOptions{}

	cmd := &cobra.Command{
		Use:          "inspect-server <node-id>",
		Short:        "Submit the inspect-server Argo workflow for a node",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeID := args[0]

			argoArgs := buildInspectArgoArgs(nodeID)

			return submitWorkflow(cmd, argoArgs, nodeID, opts.Logs)
		},
	}

	cmd.Flags().BoolVar(&opts.Logs, "log", false, "Stream workflow logs after submission")

	return cmd
}

func buildInspectArgoArgs(nodeID string) []string {
	return buildSubmitArgs(inspectWorkflowTemplate, fmt.Sprintf("node=%s", nodeID))
}
