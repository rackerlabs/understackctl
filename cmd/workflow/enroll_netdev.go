package workflow

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	enrollNetdevWorkflowTemplate = "wftmpl/enroll-netdev"
	enrollFWWorkflowTemplate     = "wftmpl/enroll-fw"
)

// netdevOptions holds the flags shared by enroll-netdev and enroll-fw, since
// enroll-fw reuses the enroll-netdev node/port lifecycle.
type netdevOptions struct {
	Name           string
	PhysicalNet    string
	Ports          string
	PortsFile      string
	ExternalCMDBID string
	ResourceClass  string
	Logs           bool
}

type fwOptions struct {
	netdevOptions
	ManagementIP         string
	ManagementSwitch     string
	ManagementSwitchPort string
	MateSerial           string
	Serial               string
	Vendor               string
	Model                string
	Driver               string
}

func addNetdevFlags(cmd *cobra.Command, opts *netdevOptions) {
	cmd.Flags().StringVar(&opts.Name, "name", "", "Node name (required)")
	cmd.Flags().StringVar(&opts.PhysicalNet, "physical-network", "", "Physical network (required)")
	cmd.Flags().StringVar(&opts.Ports, "ports", "", "Ports as a JSON array")
	cmd.Flags().StringVar(&opts.PortsFile, "ports-file", "", "File (YAML or JSON) containing the ports array")
	cmd.Flags().StringVar(&opts.ExternalCMDBID, "external-cmdb-id", "", "External CMDB identifier")
	cmd.Flags().StringVar(&opts.ResourceClass, "resource-class", "", "Ironic resource class")
	cmd.Flags().BoolVar(&opts.Logs, "log", false, "Stream workflow logs after submission")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("physical-network")
}

func newCmdEnrollNetdev() *cobra.Command {
	opts := &netdevOptions{}

	cmd := &cobra.Command{
		Use:          "enroll-netdev",
		Short:        "Submit the enroll-netdev Argo workflow for a network device",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			argoArgs, err := buildEnrollNetdevArgoArgs(cmd, opts)
			if err != nil {
				return err
			}

			return submitWorkflow(cmd, argoArgs, opts.Name, opts.Logs)
		},
	}

	addNetdevFlags(cmd, opts)

	return cmd
}

func newCmdEnrollFW() *cobra.Command {
	opts := &fwOptions{}

	cmd := &cobra.Command{
		Use:          "enroll-fw",
		Short:        "Submit the enroll-fw Argo workflow for a firewall",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			argoArgs, err := buildEnrollFWArgoArgs(cmd, opts)
			if err != nil {
				return err
			}

			return submitWorkflow(cmd, argoArgs, opts.Name, opts.Logs)
		},
	}

	addNetdevFlags(cmd, &opts.netdevOptions)
	cmd.Flags().StringVar(&opts.ManagementIP, "management-ip", "", "Firewall management IP address")
	cmd.Flags().StringVar(&opts.ManagementSwitch, "management-switch", "", "Switch FQDN the management port is on (required)")
	cmd.Flags().StringVar(&opts.ManagementSwitchPort, "management-switch-port", "", "Switch port the management interface is on (required)")
	cmd.Flags().StringVar(&opts.MateSerial, "mate-serial", "", "Serial number of the HA mate")
	cmd.Flags().StringVar(&opts.Serial, "serial", "", "Device serial number")
	cmd.Flags().StringVar(&opts.Vendor, "vendor", "", "Device vendor")
	cmd.Flags().StringVar(&opts.Model, "model", "", "Device model")
	cmd.Flags().StringVar(&opts.Driver, "driver", "", "Ironic hardware type")
	_ = cmd.MarkFlagRequired("management-switch")
	_ = cmd.MarkFlagRequired("management-switch-port")

	return cmd
}

// resolvePorts returns the ports as a compact JSON array, from either --ports
// or --ports-file.
func resolvePorts(cmd *cobra.Command, opts *netdevOptions) (string, error) {
	hasPorts := cmd.Flags().Changed("ports")
	hasFile := cmd.Flags().Changed("ports-file")

	switch {
	case hasPorts && hasFile:
		return "", fmt.Errorf("--ports and --ports-file are mutually exclusive")
	case !hasPorts && !hasFile:
		return "", fmt.Errorf("one of --ports or --ports-file is required")
	}

	raw := []byte(opts.Ports)
	if hasFile {
		var err error
		raw, err = os.ReadFile(opts.PortsFile)
		if err != nil {
			return "", fmt.Errorf("read ports file: %w", err)
		}
	}

	// YAML is a superset of JSON, so this handles both.
	var ports []map[string]any
	if err := yaml.Unmarshal(raw, &ports); err != nil {
		return "", fmt.Errorf("parse ports: %w", err)
	}
	if len(ports) == 0 {
		return "", fmt.Errorf("ports must contain at least one port")
	}

	for i, port := range ports {
		for _, key := range []string{"label", "mac", "switch", "intf"} {
			if v, ok := port[key]; !ok || v == "" {
				return "", fmt.Errorf("port %d is missing required key %q", i, key)
			}
		}
	}

	out, err := json.Marshal(ports)
	if err != nil {
		return "", fmt.Errorf("encode ports: %w", err)
	}

	return string(out), nil
}

func netdevParams(cmd *cobra.Command, opts *netdevOptions) ([]string, error) {
	ports, err := resolvePorts(cmd, opts)
	if err != nil {
		return nil, err
	}

	params := []string{
		fmt.Sprintf("name=%s", opts.Name),
		fmt.Sprintf("physical_network=%s", opts.PhysicalNet),
		fmt.Sprintf("ports=%s", ports),
	}

	if cmd.Flags().Changed("external-cmdb-id") {
		params = append(params, fmt.Sprintf("external_cmdb_id=%s", opts.ExternalCMDBID))
	}
	if cmd.Flags().Changed("resource-class") {
		params = append(params, fmt.Sprintf("resource_class=%s", opts.ResourceClass))
	}

	return params, nil
}

func buildEnrollNetdevArgoArgs(cmd *cobra.Command, opts *netdevOptions) ([]string, error) {
	params, err := netdevParams(cmd, opts)
	if err != nil {
		return nil, err
	}

	return buildSubmitArgs(enrollNetdevWorkflowTemplate, params...), nil
}

func buildEnrollFWArgoArgs(cmd *cobra.Command, opts *fwOptions) ([]string, error) {
	params, err := netdevParams(cmd, &opts.netdevOptions)
	if err != nil {
		return nil, err
	}

	params = append(params,
		fmt.Sprintf("management_switch=%s", opts.ManagementSwitch),
		fmt.Sprintf("management_switch_port=%s", opts.ManagementSwitchPort),
	)

	optional := []struct{ flag, param, value string }{
		{"management-ip", "management_ip", opts.ManagementIP},
		{"mate-serial", "mate_serial", opts.MateSerial},
		{"serial", "serial", opts.Serial},
		{"vendor", "vendor", opts.Vendor},
		{"model", "model", opts.Model},
		{"driver", "driver", opts.Driver},
	}
	for _, o := range optional {
		if cmd.Flags().Changed(o.flag) {
			params = append(params, fmt.Sprintf("%s=%s", o.param, o.value))
		}
	}

	return buildSubmitArgs(enrollFWWorkflowTemplate, params...), nil
}
