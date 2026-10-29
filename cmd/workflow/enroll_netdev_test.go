package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testPortsJSON = `[{"intf":"Ethernet1","label":"port1","mac":"aa:bb:cc:dd:ee:ff","switch":"sw1.example.com"}]`

func TestBuildEnrollNetdevArgoArgs(t *testing.T) {
	cmd := newCmdEnrollNetdev()
	for k, v := range map[string]string{
		"name":             "fw-a",
		"physical-network": "physnet1",
		"ports":            testPortsJSON,
		"resource-class":   "generic",
	} {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}

	opts := &netdevOptions{Name: "fw-a", PhysicalNet: "physnet1", Ports: testPortsJSON, ResourceClass: "generic"}
	args, err := buildEnrollNetdevArgoArgs(cmd, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"-n", enrollWorkflowNamespace,
		"submit",
		"--from", enrollNetdevWorkflowTemplate,
		"-o", "name",
		"-p", "name=fw-a",
		"-p", "physical_network=physnet1",
		"-p", "ports=" + testPortsJSON,
		"-p", "resource_class=generic",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args: got %#v want %#v", args, expected)
	}
}

func TestBuildEnrollFWArgoArgs(t *testing.T) {
	cmd := newCmdEnrollFW()
	for k, v := range map[string]string{
		"name":                   "fw-a",
		"physical-network":       "physnet1",
		"ports":                  testPortsJSON,
		"management-switch":      "sw1.example.com",
		"management-switch-port": "Ethernet48",
		"mate-serial":            "SN2",
	} {
		if err := cmd.Flags().Set(k, v); err != nil {
			t.Fatalf("set %s: %v", k, err)
		}
	}

	opts := &fwOptions{
		netdevOptions:        netdevOptions{Name: "fw-a", PhysicalNet: "physnet1", Ports: testPortsJSON},
		ManagementSwitch:     "sw1.example.com",
		ManagementSwitchPort: "Ethernet48",
		MateSerial:           "SN2",
	}
	args, err := buildEnrollFWArgoArgs(cmd, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"-n", enrollWorkflowNamespace,
		"submit",
		"--from", enrollFWWorkflowTemplate,
		"-o", "name",
		"-p", "name=fw-a",
		"-p", "physical_network=physnet1",
		"-p", "ports=" + testPortsJSON,
		"-p", "management_switch=sw1.example.com",
		"-p", "management_switch_port=Ethernet48",
		"-p", "mate_serial=SN2",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected args: got %#v want %#v", args, expected)
	}
}

func TestResolvePortsFromYAMLFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ports.yaml")
	yamlPorts := "- label: port1\n  mac: aa:bb:cc:dd:ee:ff\n  switch: sw1.example.com\n  intf: Ethernet1\n"
	if err := os.WriteFile(path, []byte(yamlPorts), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newCmdEnrollNetdev()
	if err := cmd.Flags().Set("ports-file", path); err != nil {
		t.Fatal(err)
	}

	got, err := resolvePorts(cmd, &netdevOptions{PortsFile: path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != testPortsJSON {
		t.Fatalf("got %s want %s", got, testPortsJSON)
	}
}

func TestResolvePortsErrors(t *testing.T) {
	tests := map[string]struct {
		flags map[string]string
		opts  netdevOptions
		want  string
	}{
		"none":       {want: "one of --ports or --ports-file"},
		"both":       {flags: map[string]string{"ports": "[]", "ports-file": "x"}, want: "mutually exclusive"},
		"empty":      {flags: map[string]string{"ports": "[]"}, opts: netdevOptions{Ports: "[]"}, want: "at least one port"},
		"missingKey": {flags: map[string]string{"ports": `[{"label":"a"}]`}, opts: netdevOptions{Ports: `[{"label":"a"}]`}, want: `missing required key "mac"`},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := newCmdEnrollNetdev()
			for k, v := range tc.flags {
				if err := cmd.Flags().Set(k, v); err != nil {
					t.Fatal(err)
				}
			}
			_, err := resolvePorts(cmd, &tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want error containing %q", err, tc.want)
			}
		})
	}
}
