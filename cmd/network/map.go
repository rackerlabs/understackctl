package network

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// Progress indicator that overwrites the current line
func showProgress(msg string) {
	fmt.Fprintf(os.Stderr, "\r\033[K%s", msg) // \r returns to start, \033[K clears line
}

func clearProgress() {
	fmt.Fprint(os.Stderr, "\r\033[K") // Clear the progress line
}

type mapOptions struct {
	Network       string
	Project       string
	ProjectName   string
	Cloud         string
	InfraCloud    string
	ResolveTrunks bool
}

type networkSegment struct {
	ID              string `json:"ID"`
	NetworkType     string `json:"Network Type"`
	Segmentation    int    `json:"Segment"`
	PhysicalNetwork string `json:"physical_network"`
}

type neutronPort struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	MACAddress     string         `json:"mac_address"`
	FixedIPs       []fixedIP      `json:"fixed_ips"`
	DeviceID       string         `json:"device_id"`
	DeviceOwner    string         `json:"device_owner"`
	BindingHostID  string         `json:"binding_host_id"`
	BindingProfile bindingProfile `json:"binding_profile"`
}

type bindingProfile struct {
	LocalLinkInformation []localLinkInfo `json:"local_link_information"`
	PhysicalNetwork      string          `json:"physical_network"`
	Tag                  int             `json:"tag"`         // Trunk subport VLAN tag
	ParentName           string          `json:"parent_name"` // Trunk parent port ID
}

type localLinkInfo struct {
	SwitchInfo string `json:"switch_info"`
	PortID     string `json:"port_id"`
	SwitchID   string `json:"switch_id"`
}

type fixedIP struct {
	IPAddress string `json:"ip_address"`
	SubnetID  string `json:"subnet_id"`
}

type ironicNode struct {
	Name         string `json:"name"`
	UUID         string `json:"uuid"`
	InstanceUUID string `json:"instance_uuid"`
}

func newCmdMap() *cobra.Command {
	opts := &mapOptions{}

	cmd := &cobra.Command{
		Use:   "map",
		Short: "Map network to physical switch ports",
		Long: `Map an OpenStack network to physical switch ports by following the chain:
  Network → Neutron Ports → Ironic Ports → Physical Switch Ports

This helps with debugging connectivity issues by showing the complete topology
from IP addresses down to physical switch ports.`,
		Example: `  # Map a specific network
  understackctl network map --network 4448a67b-375d-4190-9256-c34ee82c96f6

  # Map all networks in a project
  understackctl network map --project 9711d95e442846d3b84a4c11dd1a5981

  # Use specific cloud configs
  understackctl network map --network <id> --cloud uc-iad3 --infra-cloud uc-iad3-infra`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Network == "" && opts.Project == "" && opts.ProjectName == "" {
				return fmt.Errorf("one of --network, --project, or --project-name is required")
			}

			if opts.Cloud == "" {
				return fmt.Errorf("--cloud is required (cloud for Neutron API, e.g., uc-dfw3 or uc-dfw3-infra)")
			}
			if opts.InfraCloud == "" {
				opts.InfraCloud = opts.Cloud
			}

			// Resolve project name to UUID if needed
			if opts.ProjectName != "" && opts.Project == "" {
				showProgress("Resolving project name...")
				projectID, err := getProjectIDByName(opts.Cloud, opts.ProjectName)
				if err != nil {
					clearProgress()
					return fmt.Errorf("lookup project %q: %w", opts.ProjectName, err)
				}
				opts.Project = projectID
				clearProgress()
				cmd.Printf("Resolved project %q to %s\n", opts.ProjectName, projectID)
			}

			cmd.Printf("Using OpenStack cloud: %s (tenant), %s (infra)\n\n", opts.Cloud, opts.InfraCloud)

			var networkIDs []string
			if opts.Network != "" {
				networkIDs = []string{opts.Network}
			} else {
				// Get all networks for the project
				showProgress("Fetching networks for project...")
				var err error
				networkIDs, err = getNetworksForProject(opts.Cloud, opts.Project)
				clearProgress()
				if err != nil {
					return fmt.Errorf("get networks for project: %w", err)
				}
				if len(networkIDs) == 0 {
					return fmt.Errorf("no networks found for project %s", opts.Project)
				}
			}

			for i, networkID := range networkIDs {
				showProgress(fmt.Sprintf("Processing network %d/%d...", i+1, len(networkIDs)))
				if err := mapNetwork(cmd, opts, networkID); err != nil {
					clearProgress()
					return err
				}
				clearProgress()
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.Network, "network", "n", "", "Network UUID to map")
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "Project UUID (maps all networks in project)")
	cmd.Flags().StringVar(&opts.ProjectName, "project-name", "", "Project name (alternative to --project)")
	cmd.Flags().StringVarP(&opts.Cloud, "cloud", "c", "", "OpenStack cloud name for tenant/Neutron API (e.g., uc-dfw3 or uc-dfw3-infra) [required]")
	cmd.Flags().StringVar(&opts.InfraCloud, "infra-cloud", "", "OpenStack cloud name for Ironic API (e.g., uc-dfw3-infra); defaults to --cloud if omitted")
	cmd.Flags().BoolVar(&opts.ResolveTrunks, "resolve-trunks", false, "Resolve trunk:subport parents to show firewall names (slow)")

	return cmd
}

// getTrunkParentsByName uses port naming conventions to infer trunk parents
// Pattern: {prefix}-{internal|gateway}-port → {prefix}-{inside|outside}-parent
// Example: gpuaas-poc1-pri-internal-port → gpuaas-poc1-pri-inside-parent
// Returns: (parent device map, tenant VLAN map, error)
func getTrunkParentsByName(cloud, infraCloud string, subportMap map[string]string) (map[string]string, map[string]string, error) {
	parentDevices := make(map[string]string)
	tenantVLANs := make(map[string]string)

	// Extract prefixes and build parent port name candidates
	parentCandidates := make(map[string][]string) // parent name → []subport IDs

	for subportName, subportID := range subportMap {
		// Try to infer parent name from subport name
		var parentName string
		if strings.HasSuffix(subportName, "-internal-port") {
			parentName = strings.TrimSuffix(subportName, "-internal-port") + "-inside-parent"
		} else if strings.HasSuffix(subportName, "-gateway-port") {
			prefix := strings.TrimSuffix(subportName, "-gateway-port")
			parentCandidates[prefix+"-inside-parent"] = append(parentCandidates[prefix+"-inside-parent"], subportID)
			parentCandidates[prefix+"-outside-parent"] = append(parentCandidates[prefix+"-outside-parent"], subportID)
			continue
		} else {
			continue
		}

		parentCandidates[parentName] = append(parentCandidates[parentName], subportID)
	}

	// Query each parent port candidate and get trunk details
	for parentName, subportIDs := range parentCandidates {
		out, err := exec.Command("openstack", "--os-cloud", cloud, "port", "show",
			parentName, "-f", "json").Output()
		if err != nil {
			continue
		}

		var parentPort struct {
			BindingHostID string `json:"binding_host_id"`
			TrunkDetails  *struct {
				TrunkID string `json:"trunk_id"`
			} `json:"trunk_details"`
		}
		if err := json.Unmarshal(out, &parentPort); err != nil {
			continue
		}

		if parentPort.BindingHostID == "" {
			continue
		}

		// Get device name from Ironic
		deviceName, err := getNodeName(infraCloud, parentPort.BindingHostID)
		if err != nil {
			deviceName = parentName
		}

		// Get trunk details for VLAN translation info
		if parentPort.TrunkDetails != nil {
			trunkOut, err := exec.Command("openstack", "--os-cloud", cloud, "network", "trunk",
				"show", parentPort.TrunkDetails.TrunkID, "-f", "json").Output()
			if err == nil {
				var trunk struct {
					SubPorts []struct {
						PortID         string `json:"port_id"`
						SegmentationID int    `json:"segmentation_id"`
					} `json:"sub_ports"`
				}
				if json.Unmarshal(trunkOut, &trunk) == nil {
					// Map segmentation IDs to our subports
					for _, sp := range trunk.SubPorts {
						for _, subportID := range subportIDs {
							if sp.PortID == subportID && sp.SegmentationID > 0 {
								tenantVLANs[subportID] = fmt.Sprintf("%d", sp.SegmentationID)
							}
						}
					}
				}
			}
		}

		// Map all subports to this device
		for _, subportID := range subportIDs {
			parentDevices[subportID] = deviceName
		}
	}

	return parentDevices, tenantVLANs, nil
}

func getNetworkSegments(cloud, networkID string) ([]networkSegment, error) {
	out, err := exec.Command("openstack", "--os-cloud", cloud, "network", "segment",
		"list", "--network", networkID, "-f", "json").Output()
	if err != nil {
		return nil, err
	}

	var segments []networkSegment
	if err := json.Unmarshal(out, &segments); err != nil {
		return nil, err
	}

	// Get physical_network for each segment (not in list output)
	for i := range segments {
		segOut, err := exec.Command("openstack", "--os-cloud", cloud, "network", "segment",
			"show", segments[i].ID, "-f", "json").Output()
		if err != nil {
			continue
		}

		var segDetail struct {
			PhysicalNetwork string `json:"physical_network"`
		}
		if json.Unmarshal(segOut, &segDetail) == nil {
			segments[i].PhysicalNetwork = segDetail.PhysicalNetwork
		}
	}

	return segments, nil
}

func getNetworkInfo(cloud, networkID string) (string, string, error) {
	out, err := exec.Command("openstack", "--os-cloud", cloud, "network", "show",
		networkID, "-f", "json").Output()
	if err != nil {
		return "", "", err
	}

	var network struct {
		Name           string      `json:"name"`
		SegmentationID interface{} `json:"provider:segmentation_id"`
	}
	if err := json.Unmarshal(out, &network); err != nil {
		return "", "", err
	}

	vni := ""
	if network.SegmentationID != nil {
		// Format as integer to avoid scientific notation
		switch v := network.SegmentationID.(type) {
		case float64:
			vni = fmt.Sprintf("%.0f", v)
		case int:
			vni = fmt.Sprintf("%d", v)
		default:
			vni = fmt.Sprintf("%v", v)
		}
	}

	return network.Name, vni, nil
}

func getSubnetName(cloud, subnetID string) (string, error) {
	out, err := exec.Command("openstack", "--os-cloud", cloud, "subnet", "show",
		subnetID, "-f", "json").Output()
	if err != nil {
		return "", err
	}

	var subnet struct {
		Name string `json:"name"`
		CIDR string `json:"cidr"`
	}
	if err := json.Unmarshal(out, &subnet); err != nil {
		return "", err
	}

	// Return CIDR if no name
	if subnet.Name != "" {
		return subnet.Name, nil
	}
	return subnet.CIDR, nil
}

func getProjectIDByName(cloud, projectName string) (string, error) {
	out, err := exec.Command("openstack", "--os-cloud", cloud, "project", "list",
		"-f", "json").Output()
	if err != nil {
		return "", err
	}

	var projects []struct {
		ID   string `json:"ID"`
		Name string `json:"Name"`
	}
	if err := json.Unmarshal(out, &projects); err != nil {
		return "", err
	}

	for _, p := range projects {
		if p.Name == projectName {
			return p.ID, nil
		}
	}

	return "", fmt.Errorf("project %q not found", projectName)
}

func getNetworksForProject(cloud, projectID string) ([]string, error) {
	out, err := exec.Command("openstack", "--os-cloud", cloud, "network", "list",
		"--project", projectID, "-f", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("openstack network list: %w", err)
	}

	var networks []struct {
		ID string `json:"ID"`
	}
	if err := json.Unmarshal(out, &networks); err != nil {
		return nil, fmt.Errorf("parse network list: %w", err)
	}

	var ids []string
	for _, n := range networks {
		ids = append(ids, n.ID)
	}
	return ids, nil
}

func mapNetwork(cmd *cobra.Command, opts *mapOptions, networkID string) error {
	// Get network name and VNI
	showProgress("Fetching network details...")
	networkName, networkVNI, err := getNetworkInfo(opts.Cloud, networkID)
	if err != nil {
		clearProgress()
		return fmt.Errorf("get network info: %w", err)
	}

	vniDisplay := networkVNI
	if vniDisplay == "" {
		vniDisplay = "(none)"
	}

	clearProgress()
	cmd.Printf("=== Network: %s (%s) | VNI: %s ===\n\n", networkName, networkID, vniDisplay)

	// Get network segments to map physical_network → VLAN
	showProgress("Fetching network segments...")
	segments, err := getNetworkSegments(opts.Cloud, networkID)
	if err != nil {
		clearProgress()
		return fmt.Errorf("get network segments: %w", err)
	}

	// Build map of physical_network → VLAN ID
	physnetVLANs := make(map[string]string)
	for _, seg := range segments {
		if seg.NetworkType == "vlan" && seg.PhysicalNetwork != "" {
			physnetVLANs[seg.PhysicalNetwork] = fmt.Sprintf("%d", seg.Segmentation)
		}
	}

	// Get all compute ports on this network
	showProgress("Fetching neutron ports...")
	neutronPorts, err := getNeutronPorts(opts.Cloud, networkID)
	if err != nil {
		clearProgress()
		return fmt.Errorf("get neutron ports: %w", err)
	}

	if len(neutronPorts) == 0 {
		clearProgress()
		cmd.Printf("No ports found on this network (excluding network infrastructure ports)\n\n")
		return nil
	}

	// Get subnet info for VLAN/subnet display (if different per port)
	showProgress("Fetching subnet information...")
	subnetNames := make(map[string]string)
	for _, np := range neutronPorts {
		for _, fixedIP := range np.FixedIPs {
			if _, exists := subnetNames[fixedIP.SubnetID]; !exists {
				if name, err := getSubnetName(opts.Cloud, fixedIP.SubnetID); err == nil {
					subnetNames[fixedIP.SubnetID] = name
				}
			}
		}
	}

	// Get node names from binding_host_id (which is the Ironic node UUID)
	showProgress("Fetching server names from Ironic...")
	nodeNames := make(map[string]string)
	for _, np := range neutronPorts {
		if np.BindingHostID != "" {
			if name, err := getNodeName(opts.InfraCloud, np.BindingHostID); err == nil {
				nodeNames[np.BindingHostID] = name
			}
		}
	}

	// For trunk:subport ports, use name-based inference to find parent device
	// This is much faster than querying all trunks
	trunkParents := make(map[string]string)             // subport ID → parent device name
	trunkVLANs := make(map[string]string)               // subport ID → tenant VLAN (segmentation_id)
	subportSwitchVLANs := make(map[string]string)       // subport ID → switch VLAN (from parent physical_network)
	subportParentPorts := make(map[string]*neutronPort) // subport ID → parent port details (for switch info)

	if opts.ResolveTrunks {
		showProgress("Resolving trunk subports...")
		subportMap := make(map[string]string)    // subport name → subport ID
		parentPortIDs := make(map[string]string) // subport ID → parent port ID

		for _, np := range neutronPorts {
			if np.DeviceOwner == "trunk:subport" {
				subportMap[np.Name] = np.ID
				// Collect parent port ID from binding_profile.parent_name
				if np.BindingProfile.ParentName != "" {
					parentPortIDs[np.ID] = np.BindingProfile.ParentName
				}
			}
		}

		if len(subportMap) > 0 {
			parents, vlans, err := getTrunkParentsByName(opts.Cloud, opts.InfraCloud, subportMap)
			if err != nil {
				clearProgress()
				cmd.Printf("Warning: trunk resolution partially failed: %v\n", err)
			}
			trunkParents = parents
			trunkVLANs = vlans
		}

		// Get switch VLANs and switch info for trunk subports by fetching parent port details
		showProgress(fmt.Sprintf("Fetching parent port details for %d trunk subports...", len(parentPortIDs)))
		for subportID, parentPortID := range parentPortIDs {
			if parentPort, err := getParentPortDetails(opts.Cloud, parentPortID); err == nil {
				subportParentPorts[subportID] = parentPort
				if parentPort.BindingProfile.PhysicalNetwork != "" {
					if switchVLAN, ok := physnetVLANs[parentPort.BindingProfile.PhysicalNetwork]; ok {
						subportSwitchVLANs[subportID] = switchVLAN
					}
				}
			}
		}
	}

	// Print table - one row per port, with multi-line for bonded links
	clearProgress()
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(tw, "IP ADDRESS\tSUBNET\tNAME\tSERVER\tSWITCH\tPORT")
	_, _ = fmt.Fprintln(tw, "----------\t------\t----\t------\t------\t----")

	for _, np := range neutronPorts {
		ip := ""
		subnet := ""
		if len(np.FixedIPs) > 0 {
			ip = np.FixedIPs[0].IPAddress
			subnet = subnetNames[np.FixedIPs[0].SubnetID]
			if subnet == "" {
				subnet = np.FixedIPs[0].SubnetID[:8] + "..."
			}
		}

		serverName := "(unbound)"
		portName := np.Name
		if portName == "" {
			portName = np.ID[:8] + "..."
		}

		// Get server name from binding_host_id
		if np.BindingHostID != "" {
			serverName = nodeNames[np.BindingHostID]
			if serverName == "" {
				serverName = np.BindingHostID[:8] + "..."
			}
		}

		// Get switch VLAN from physical_network
		switchVLAN := ""
		if np.BindingProfile.PhysicalNetwork != "" {
			switchVLAN = physnetVLANs[np.BindingProfile.PhysicalNetwork]
		}

		// Check if this is a trunk subport with VLAN translation
		tenantVLAN := trunkVLANs[np.ID]
		var vlanDisplay string
		if switchVLAN != "" {
			if tenantVLAN != "" && tenantVLAN != switchVLAN {
				vlanDisplay = fmt.Sprintf("VLAN %s→%s", switchVLAN, tenantVLAN)
			} else {
				vlanDisplay = fmt.Sprintf("VLAN %s", switchVLAN)
			}
		}

		// Get switch info from binding_profile
		if len(np.BindingProfile.LocalLinkInformation) == 0 {
			// No binding info - check if it's a trunk subport or other network device
			deviceType := "(network)"
			deviceName := ""

			if np.DeviceOwner == "trunk:subport" {
				// This is a firewall/netdev VLAN subinterface
				deviceName = trunkParents[np.ID]
				if deviceName != "" {
					deviceType = deviceName
				} else {
					deviceType = "(trunk:subport)"
				}

				// For trunk subports, show VLAN translation and parent port's switch info
				switchVLAN := subportSwitchVLANs[np.ID]
				tenantVLAN := ""
				if np.BindingProfile.Tag > 0 {
					tenantVLAN = fmt.Sprintf("%d", np.BindingProfile.Tag)
				}

				var vlanInfo string
				if switchVLAN != "" && tenantVLAN != "" && switchVLAN != tenantVLAN {
					// Show VLAN translation: switch VLAN → tenant VLAN
					vlanInfo = fmt.Sprintf("VLAN %s→%s", switchVLAN, tenantVLAN)
				} else if tenantVLAN != "" {
					// Just show tenant VLAN
					vlanInfo = fmt.Sprintf("VLAN %s", tenantVLAN)
				}

				// Check if we have parent port switch information
				if parentPort, ok := subportParentPorts[np.ID]; ok && len(parentPort.BindingProfile.LocalLinkInformation) > 0 {
					// Show parent port's physical switch connection with VLAN info
					if len(parentPort.BindingProfile.LocalLinkInformation) == 1 {
						link := parentPort.BindingProfile.LocalLinkInformation[0]
						portDisplay := link.PortID
						if vlanInfo != "" {
							portDisplay = fmt.Sprintf("%s (%s)", link.PortID, vlanInfo)
						}
						_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
							ip, subnet, portName, deviceType, link.SwitchInfo, portDisplay)
					} else {
						// Bonded parent port - show all links
						for i, link := range parentPort.BindingProfile.LocalLinkInformation {
							portDisplay := link.PortID
							if vlanInfo != "" {
								portDisplay = fmt.Sprintf("%s (%s)", link.PortID, vlanInfo)
							}
							if i == 0 {
								_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
									ip, subnet, portName, deviceType, link.SwitchInfo, portDisplay)
							} else {
								_, _ = fmt.Fprintf(tw, "\t\t\t\t%s\t%s\n",
									link.SwitchInfo, portDisplay)
							}
						}
					}
				} else {
					// No parent port switch info, show VLAN only
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t\n",
						ip, subnet, portName, deviceType, fmt.Sprintf("(%s)", vlanInfo))
				}
			} else if np.DeviceOwner != "" {
				deviceType = fmt.Sprintf("(%s)", strings.TrimPrefix(np.DeviceOwner, "network:"))
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t\n",
					ip, subnet, portName, deviceType, "")
			} else {
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t\n",
					ip, subnet, portName, deviceType, "")
			}
		} else if len(np.BindingProfile.LocalLinkInformation) == 1 {
			// Single link
			link := np.BindingProfile.LocalLinkInformation[0]
			portDisplay := link.PortID
			if vlanDisplay != "" {
				portDisplay = fmt.Sprintf("%s (%s)", link.PortID, vlanDisplay)
			}
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				ip, subnet, portName, serverName, link.SwitchInfo, portDisplay)
		} else {
			// Multiple links (bonded) - show first line with IP, then additional links
			for i, link := range np.BindingProfile.LocalLinkInformation {
				portDisplay := link.PortID
				if vlanDisplay != "" {
					portDisplay = fmt.Sprintf("%s (%s)", link.PortID, vlanDisplay)
				}
				if i == 0 {
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
						ip, subnet, portName, serverName, link.SwitchInfo, portDisplay)
				} else {
					_, _ = fmt.Fprintf(tw, "\t\t\t\t%s\t%s\n",
						link.SwitchInfo, portDisplay)
				}
			}
		}
	}

	_ = tw.Flush()
	cmd.Printf("\n")

	return nil
}

func getNeutronPorts(cloud, networkID string) ([]neutronPort, error) {
	out, err := exec.Command("openstack", "--os-cloud", cloud, "port", "list",
		"--network", networkID, "-f", "json").Output()
	if err != nil {
		return nil, err
	}

	var allPorts []struct {
		ID          string `json:"ID"`
		Name        string `json:"Name"`
		DeviceOwner string `json:"Device Owner"`
	}
	if err := json.Unmarshal(out, &allPorts); err != nil {
		return nil, err
	}

	// Get full details for all ports
	// Filter: include compute ports, baremetal ports, or ports with no device owner (pre-created)
	var ports []neutronPort
	for _, p := range allPorts {
		// Skip router interface and DHCP ports
		if strings.HasPrefix(p.DeviceOwner, "network:") {
			continue
		}

		out, err := exec.Command("openstack", "--os-cloud", cloud, "port", "show",
			p.ID, "-f", "json").Output()
		if err != nil {
			continue
		}

		var port neutronPort
		if err := json.Unmarshal(out, &port); err != nil {
			continue
		}

		ports = append(ports, port)
	}

	return ports, nil
}

func getNodeName(infraCloud, nodeUUID string) (string, error) {
	out, err := exec.Command("openstack", "--os-cloud", infraCloud, "baremetal", "node",
		"show", nodeUUID, "-f", "json").Output()
	if err != nil {
		return "", err
	}

	var node ironicNode
	if err := json.Unmarshal(out, &node); err != nil {
		return "", err
	}

	return node.Name, nil
}

func getParentPortDetails(cloud, parentPortID string) (*neutronPort, error) {
	out, err := exec.Command("openstack", "--os-cloud", cloud, "port", "show",
		parentPortID, "-f", "json").Output()
	if err != nil {
		return nil, err
	}

	var port neutronPort
	if err := json.Unmarshal(out, &port); err != nil {
		return nil, err
	}

	return &port, nil
}
