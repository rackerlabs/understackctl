package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/duration"
)

const workflowTemplateLabel = "workflows.argoproj.io/workflow-template"

type listOptions struct {
	Templates   []string
	Kind        string
	DeviceType  string
	Target      string
	SubmittedBy string
	Status      string
	Selector    string
	Output      string
}

func newCmdList() *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List existing Argo workflows",
		Long: "List workflows in the " + enrollWorkflowNamespace + " namespace, filtered by the labels " +
			"UnderStack stamps on them. With no filters, all workflows are listed.",
		Example: `  # Failed enrollments
  understackctl workflow list --kind enroll --status Failed,Error

  # Only server and firewall enrollments
  understackctl workflow list --template enroll-server,enroll-fw

  # Everything that touched a node
  understackctl workflow list --target 10.46.6.165`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			argoArgs := buildListArgoArgs(opts)

			currentContext, err := currentKubeContext()
			if err != nil {
				return fmt.Errorf("get current Kubernetes context: %w", err)
			}

			cmd.Printf("Kubernetes context: %s\n", currentContext)
			cmd.Printf("Running: %s\n", shellQuoteCommand("argo", argoArgs))

			argoCmd := exec.Command("argo", argoArgs...)
			argoCmd.Stdin = os.Stdin
			argoCmd.Stderr = cmd.ErrOrStderr()

			// An explicit -o is passed straight through to argo. Otherwise we
			// fetch JSON and render the default table ourselves, since argo's
			// has no way to drop its PRIORITY column.
			if opts.Output != "" {
				argoCmd.Stdout = cmd.OutOrStdout()
				if err := argoCmd.Run(); err != nil {
					return fmt.Errorf("list argo workflows: %w", err)
				}

				return nil
			}

			var stdout bytes.Buffer
			argoCmd.Stdout = &stdout
			if err := argoCmd.Run(); err != nil {
				return fmt.Errorf("list argo workflows: %w", err)
			}

			return renderWorkflowTable(cmd.OutOrStdout(), stdout.Bytes(), time.Now())
		},
	}

	cmd.Flags().StringSliceVarP(&opts.Templates, "template", "t", nil, "Only workflows from these WorkflowTemplates, comma separated or repeated (e.g. enroll-server,enroll-fw)")
	cmd.Flags().StringVar(&opts.Kind, "kind", "", "Only workflows of this kind (enroll, inspect)")
	cmd.Flags().StringVar(&opts.DeviceType, "device-type", "", "Only workflows for this device type (server, netdev, fw)")
	cmd.Flags().StringVar(&opts.Target, "target", "", "Only workflows submitted for this IP address, node name or node ID")
	cmd.Flags().StringVar(&opts.SubmittedBy, "submitted-by", "", "Only workflows submitted by this user")
	cmd.Flags().StringVar(&opts.Status, "status", "", "Only workflows in these phases, comma separated (e.g. Failed,Error)")
	cmd.Flags().StringVarP(&opts.Selector, "selector", "l", "", "Additional label selector, ANDed with the filters above")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "Output format passed to argo (wide, name, json, yaml)")

	return cmd
}

// buildListSelector joins the label filters, and any user-supplied selector,
// into a single label selector. Values are sanitized the same way as when the
// labels are stamped at submission, so a target typed as given still matches.
func buildListSelector(opts *listOptions) string {
	var terms []string
	if templates := nonEmpty(opts.Templates); len(templates) == 1 {
		terms = append(terms, fmt.Sprintf("%s=%s", workflowTemplateLabel, templates[0]))
	} else if len(templates) > 1 {
		terms = append(terms, fmt.Sprintf("%s in (%s)", workflowTemplateLabel, strings.Join(templates, ",")))
	}
	for _, f := range []struct{ key, value string }{
		{"workflow-kind", opts.Kind},
		{"device-type", opts.DeviceType},
		{"target", opts.Target},
		{"submitted-by", opts.SubmittedBy},
	} {
		if v := sanitizeLabelValue(f.value); v != "" {
			terms = append(terms, fmt.Sprintf("%s%s=%s", labelPrefix, f.key, v))
		}
	}
	if opts.Selector != "" {
		terms = append(terms, opts.Selector)
	}

	return strings.Join(terms, ",")
}

func nonEmpty(values []string) []string {
	var out []string
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}

	return out
}

func buildListArgoArgs(opts *listOptions) []string {
	args := []string{"-n", enrollWorkflowNamespace, "list"}

	if selector := buildListSelector(opts); selector != "" {
		args = append(args, "-l", selector)
	}
	if opts.Status != "" {
		args = append(args, "--status", opts.Status)
	}
	if opts.Output != "" {
		args = append(args, "-o", opts.Output)
	} else {
		args = append(args, "-o", "json")
	}

	return args
}

type workflowItem struct {
	Metadata struct {
		Name              string    `json:"name"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
	} `json:"metadata"`
	Status struct {
		Phase      string    `json:"phase"`
		StartedAt  time.Time `json:"startedAt"`
		FinishedAt time.Time `json:"finishedAt"`
	} `json:"status"`
}

// renderWorkflowTable prints the output of `argo list -o json` as a table of
// NAME, STATUS, AGE and DURATION, newest first.
func renderWorkflowTable(w io.Writer, data []byte, now time.Time) error {
	var items []workflowItem
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("parse argo list output: %w", err)
	}

	if len(items) == 0 {
		_, err := fmt.Fprintln(w, "No workflows found")
		return err
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Metadata.CreationTimestamp.After(items[j].Metadata.CreationTimestamp)
	})

	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	// tabwriter buffers, so write errors surface from Flush below.
	_, _ = fmt.Fprintln(tw, "NAME\tSTATUS\tAGE\tDURATION")
	for _, item := range items {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			item.Metadata.Name,
			item.Status.Phase,
			duration.HumanDuration(now.Sub(item.Metadata.CreationTimestamp)),
			workflowDuration(item, now),
		)
	}

	return tw.Flush()
}

// workflowDuration is how long the workflow ran, or has been running.
func workflowDuration(item workflowItem, now time.Time) string {
	if item.Status.StartedAt.IsZero() {
		return "0s"
	}

	end := item.Status.FinishedAt
	if end.IsZero() {
		end = now
	}

	return duration.HumanDuration(end.Sub(item.Status.StartedAt))
}
