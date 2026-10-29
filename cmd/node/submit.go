package node

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

const labelPrefix = "understack.rackspace.com/"

var invalidLabelChars = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// sanitizeLabelValue coerces s into a valid Kubernetes label value: at most 63
// characters of alphanumerics, '-', '_' and '.', starting and ending with an
// alphanumeric.
func sanitizeLabelValue(s string) string {
	s = invalidLabelChars.ReplaceAllString(s, "-")
	if len(s) > 63 {
		s = s[:63]
	}

	return strings.Trim(s, "-_.")
}

// submitLabels returns the comma-separated labels stamped on a submitted
// workflow. Empty values are omitted. The workflow-kind and device-type labels
// come from the WorkflowTemplate itself.
func submitLabels(target, submittedBy string) string {
	var labels []string
	if v := sanitizeLabelValue(target); v != "" {
		labels = append(labels, labelPrefix+"target="+v)
	}
	if v := sanitizeLabelValue(submittedBy); v != "" {
		labels = append(labels, labelPrefix+"submitted-by="+v)
	}

	return strings.Join(labels, ",")
}

// submitWorkflow runs `argo submit` for the given args, which must include
// `-o name` so that stdout is just the generated workflow name. It labels the
// workflow with its target, prints the name and, if logs is set, follows the
// workflow logs.
func submitWorkflow(cmd *cobra.Command, argoArgs []string, target string, logs bool) error {
	currentContext, err := currentKubeContext()
	if err != nil {
		return fmt.Errorf("get current Kubernetes context: %w", err)
	}

	cmd.Printf("Kubernetes context: %s\n", currentContext)
	var submittedBy string
	if u, err := user.Current(); err == nil {
		submittedBy = u.Username
	}
	if labels := submitLabels(target, submittedBy); labels != "" {
		argoArgs = append(argoArgs, "--labels", labels)
	}

	cmd.Printf("Running: %s\n", shellQuoteCommand("argo", argoArgs))

	var stdout bytes.Buffer
	argoCmd := exec.Command("argo", argoArgs...)
	argoCmd.Stdin = os.Stdin
	argoCmd.Stdout = &stdout
	argoCmd.Stderr = cmd.ErrOrStderr()

	if err := argoCmd.Run(); err != nil {
		return fmt.Errorf("submit argo workflow: %w", err)
	}

	name := strings.TrimSpace(stdout.String())
	if name == "" {
		return fmt.Errorf("argo submit did not return a workflow name")
	}

	cmd.Printf("Workflow: %s\n", name)

	if server, err := currentKubeServer(); err == nil {
		if uiURL, ok := workflowUIURL(server, enrollWorkflowNamespace, name); ok {
			cmd.Printf("Argo UI: %s\n", uiURL)
		}
	}

	if !logs {
		return nil
	}

	logsArgs := []string{"-n", enrollWorkflowNamespace, "logs", "--follow", name}
	logsCmd := exec.Command("argo", logsArgs...)
	logsCmd.Stdin = os.Stdin
	logsCmd.Stdout = cmd.OutOrStdout()
	logsCmd.Stderr = cmd.ErrOrStderr()

	if err := logsCmd.Run(); err != nil {
		return fmt.Errorf("follow logs for workflow %s: %w", name, err)
	}

	return nil
}

// buildSubmitArgs returns the argo arguments to submit a WorkflowTemplate,
// printing only the generated workflow name.
func buildSubmitArgs(template string, params ...string) []string {
	args := []string{
		"-n", enrollWorkflowNamespace,
		"submit",
		"--from", template,
		"-o", "name",
	}
	for _, p := range params {
		args = append(args, "-p", p)
	}

	return args
}

// workflowUIURL derives the Argo Workflows UI link for a workflow from the
// Kubernetes API server URL by replacing its left-most hostname label with
// "workflows" (e.g. https://kubernetes.dc.example.com:6443 ->
// https://workflows.dc.example.com). It returns false when the host is an IP
// address or has no domain to swap into.
func workflowUIURL(server, namespace, name string) (string, bool) {
	u, err := url.Parse(server)
	if err != nil {
		return "", false
	}

	host := u.Hostname()
	if host == "" || net.ParseIP(host) != nil {
		return "", false
	}

	_, domain, found := strings.Cut(host, ".")
	if !found || domain == "" {
		return "", false
	}

	return fmt.Sprintf("https://workflows.%s/workflows/%s/%s", domain, namespace, name), true
}
