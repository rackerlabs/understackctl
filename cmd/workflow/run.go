package workflow

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCmdRun() *cobra.Command {
	var parameters []string
	var logs bool

	cmd := &cobra.Command{
		Use:          "run <template-name>",
		Short:        "Submit an installed Argo WorkflowTemplate",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, parameter := range parameters {
				key, _, ok := strings.Cut(parameter, "=")
				if !ok || strings.TrimSpace(key) == "" {
					return fmt.Errorf("invalid parameter %q: expected name=value", parameter)
				}
			}
			argoArgs := buildSubmitArgs("wftmpl/"+args[0], parameters...)
			return submitWorkflow(cmd, argoArgs, "", logs)
		},
	}

	cmd.Flags().StringArrayVarP(&parameters, "parameter", "p", nil, "Workflow parameter as name=value (repeatable)")
	cmd.Flags().BoolVar(&logs, "log", false, "Stream workflow logs after submission")

	return cmd
}
