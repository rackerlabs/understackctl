package workflow

import "github.com/spf13/cobra"

func NewCmdWorkflow() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Submit and manage UnderStack Argo workflows",
		Long:  "Submit and manage node lifecycle workflows by wrapping Argo workflows and related platform APIs",
	}

	cmd.AddCommand(newCmdEnrollServer())
	cmd.AddCommand(newCmdEnrollNetdev())
	cmd.AddCommand(newCmdEnrollFW())
	cmd.AddCommand(newCmdInspectServer())
	cmd.AddCommand(newCmdList())

	return cmd
}
