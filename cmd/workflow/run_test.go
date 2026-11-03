package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunSubmission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake argo executable uses a POSIX shell")
	}
	for _, tc := range []struct {
		name   string
		flags  []string
		params []string
		logs   bool
	}{
		{name: "defaults"},
		{name: "parameters and logs", flags: []string{"--parameter", "node_uuid=node-123", "-p", `options=["first","second"]`, "--parameter", "query=a=b", "--log"}, params: []string{"node_uuid=node-123", `options=["first","second"]`, "query=a=b"}, logs: true},
		{name: "empty and false values", flags: []string{"-p", "value=", "-p", "enabled=false"}, params: []string{"value=", "enabled=false"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			calls := filepath.Join(dir, "calls")
			// Record each argument separately to verify the actual subprocess contract.
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >> \"$ARGO_TEST_CALLS\"\nif [ \"$3\" = submit ]; then echo my-workflow-test; fi\n"
			if err := os.WriteFile(filepath.Join(dir, "argo"), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(dir, "config")
			if err := os.WriteFile(config, []byte(`apiVersion: v1
kind: Config
current-context: test-cluster
contexts:
- name: test-cluster
  context:
    cluster: test-cluster
clusters:
- name: test-cluster
  cluster:
    server: https://kubernetes.site.example.com
`), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("KUBECONFIG", config)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("ARGO_TEST_CALLS", calls)

			cmd := NewCmdWorkflow()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(append([]string{"run", "my-workflow"}, tc.flags...))
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute: %v\n%s", err, output.String())
			}
			data, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			got := string(data)
			want := "-n\nargo-events\nsubmit\n--from\nwftmpl/my-workflow\n-o\nname\n"
			for _, parameter := range tc.params {
				want += "-p\n" + parameter + "\n"
			}
			if !strings.HasPrefix(got, want) {
				t.Fatalf("unexpected argo submission: %q", got)
			}
			logsCall := "-n\nargo-events\nlogs\n--follow\nmy-workflow-test\n"
			if strings.Contains(got, logsCall) != tc.logs {
				t.Fatalf("logs call mismatch: %q", got)
			}
			if !strings.Contains(output.String(), "https://workflows.site.example.com/workflows/argo-events/my-workflow-test") {
				t.Fatalf("missing workflow UI link: %s", output.String())
			}
		})
	}
}

func TestRunRequiresOneTemplate(t *testing.T) {
	for _, args := range [][]string{{}, {"node-1", "node-2"}} {
		cmd := newCmdRun()
		if err := cmd.Args(cmd, args); err == nil {
			t.Errorf("expected argument error for %q", args)
		}
	}
}

func TestRunRejectsInvalidParameters(t *testing.T) {
	for _, parameter := range []string{"missing-value", "=value", " =value"} {
		cmd := newCmdRun()
		cmd.SetArgs([]string{"my-workflow", "-p", parameter})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "expected name=value") {
			t.Errorf("expected parameter validation error for %q, got %v", parameter, err)
		}
	}
}
