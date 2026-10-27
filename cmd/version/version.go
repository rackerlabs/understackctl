package version

import (
	"encoding/json"
	"fmt"

	"github.com/rackerlabs/understackctl/internal/version"
	"github.com/spf13/cobra"
)

// NewCmdVersion returns the "version" command.
func NewCmdVersion() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version, commit and build information",
		Long: `Print the version of understackctl along with the commit it was built
from, the build date, and the Go toolchain and platform it was built for.

Include this output when reporting a bug.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			info := version.Get()

			switch output {
			case "text":
				if _, err := fmt.Fprint(cmd.OutOrStdout(), info.Multiline()); err != nil {
					return fmt.Errorf("failed to write version info: %w", err)
				}
			case "json":
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(info); err != nil {
					return fmt.Errorf("failed to encode version info: %w", err)
				}
			default:
				return fmt.Errorf("unsupported output format %q: want text or json", output)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text or json")

	return cmd
}
