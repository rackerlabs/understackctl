package workflow

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	enrollWorkflowNamespace = "argo-events"
	enrollWorkflowTemplate  = "wftmpl/enroll-server"
)

type enrollOptions struct {
	OldPassword    string
	FirmwareUpdate bool
	RaidConfigure  bool
	ExternalCMDBID string
	Logs           bool
}

func newCmdEnrollServer() *cobra.Command {
	opts := &enrollOptions{}

	cmd := &cobra.Command{
		Use:          "enroll-server <ip-address>",
		Short:        "Submit the enroll-server Argo workflow for a node",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ipAddress := args[0]
			if net.ParseIP(ipAddress) == nil {
				return fmt.Errorf("invalid IP address: %s", ipAddress)
			}

			argoArgs, err := buildEnrollArgoArgs(cmd, ipAddress, opts)
			if err != nil {
				return err
			}

			return submitWorkflow(cmd, argoArgs, ipAddress, opts.Logs)
		},
	}

	cmd.Flags().StringVar(&opts.OldPassword, "old-password", "", "Existing node password")
	cmd.Flags().BoolVar(&opts.FirmwareUpdate, "firmware-update", false, "Whether to perform a firmware update")
	cmd.Flags().BoolVar(&opts.RaidConfigure, "raid-configure", false, "Whether to configure RAID")
	cmd.Flags().StringVar(&opts.ExternalCMDBID, "external-cmdb-id", "", "External CMDB identifier")
	cmd.Flags().BoolVar(&opts.Logs, "log", false, "Stream workflow logs after submission")

	return cmd
}

func buildEnrollArgoArgs(cmd *cobra.Command, ipAddress string, opts *enrollOptions) ([]string, error) {
	params := []string{fmt.Sprintf("ip_address=%s", ipAddress)}

	if cmd.Flags().Changed("old-password") {
		params = append(params, fmt.Sprintf("old_password=%s", opts.OldPassword))
	}

	if cmd.Flags().Changed("firmware-update") {
		params = append(params, fmt.Sprintf("firmware_update=%t", opts.FirmwareUpdate))
	}

	if cmd.Flags().Changed("raid-configure") {
		params = append(params, fmt.Sprintf("raid_configure=%t", opts.RaidConfigure))
	}

	if cmd.Flags().Changed("external-cmdb-id") {
		params = append(params, fmt.Sprintf("external_cmdb_id=%s", opts.ExternalCMDBID))
	}

	return buildSubmitArgs(enrollWorkflowTemplate, params...), nil
}

func currentKubeContext() (string, error) {
	config, err := clientcmd.NewDefaultPathOptions().GetStartingConfig()
	if err != nil {
		return "", err
	}

	if config.CurrentContext == "" {
		return "", fmt.Errorf("no current context is set")
	}

	return config.CurrentContext, nil
}

// currentKubeServer returns the API server URL of the current context's cluster.
func currentKubeServer() (string, error) {
	config, err := clientcmd.NewDefaultPathOptions().GetStartingConfig()
	if err != nil {
		return "", err
	}

	ctx, ok := config.Contexts[config.CurrentContext]
	if !ok {
		return "", fmt.Errorf("current context %q not found", config.CurrentContext)
	}

	cluster, ok := config.Clusters[ctx.Cluster]
	if !ok {
		return "", fmt.Errorf("cluster %q not found", ctx.Cluster)
	}

	return cluster.Server, nil
}

func shellQuoteCommand(name string, args []string) string {
	parts := []string{shellQuote(name)}
	for _, arg := range args {
		parts = append(parts, shellQuote(arg))
	}

	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}

	if !strings.ContainsAny(value, " \t\n'\"\\$&;|<>(){}[]*?!#~=`") {
		return value
	}

	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
