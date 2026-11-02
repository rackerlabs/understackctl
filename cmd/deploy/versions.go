package deploy

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

const (
	versionsRemote = "origin"
	versionsBranch = "main"
)

// deployConfigSource yields the environments of a deployment repo and the
// parsed deploy.yaml of each, from either the working tree or a git ref.
type deployConfigSource struct {
	environments func() ([]string, error)
	load         func(environment string) (map[string]any, error)
}

func newCmdDeployVersions() *cobra.Command {
	var showUnknowns, local, noFetch bool

	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Show the UnderStack version deployed in each environment",
		Long: `List every environment in the deployment repo along with its understack_ref and deploy_ref from deploy.yaml.
By default origin/main is fetched and read straight from git, so the output reflects the latest main
regardless of the branch checked out. Use --local to read the working tree instead.
Environments where either ref is unknown are hidden unless --show-unknowns is passed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if local {
				return runDeployVersions(".", cmd.OutOrStdout(), showUnknowns)
			}
			return runDeployVersionsFromGit(".", cmd.OutOrStdout(), showUnknowns, !noFetch)
		},
	}

	cmd.Flags().BoolVar(&showUnknowns, "show-unknowns", false, "include environments whose understack_ref or deploy_ref is unknown")
	cmd.Flags().BoolVar(&local, "local", false, "read the working tree instead of origin/main")
	cmd.Flags().BoolVar(&noFetch, "no-fetch", false, "skip fetching origin/main and use the last fetched state")
	cmd.MarkFlagsMutuallyExclusive("local", "no-fetch")

	return cmd
}

func runDeployVersions(repoDir string, out io.Writer, showUnknowns bool) error {
	return printDeployVersions(workingTreeSource(repoDir), out, showUnknowns)
}

func runDeployVersionsFromGit(repoDir string, out io.Writer, showUnknowns, fetch bool) error {
	ref := versionsRemote + "/" + versionsBranch

	if fetch {
		if _, err := runGit(repoDir, "fetch", versionsRemote, versionsBranch); err != nil {
			return fmt.Errorf("failed to fetch %s (use --no-fetch or --local to work offline): %w", ref, err)
		}
	}

	return printDeployVersions(gitRefSource(repoDir, ref), out, showUnknowns)
}

func workingTreeSource(repoDir string) deployConfigSource {
	return deployConfigSource{
		environments: func() ([]string, error) {
			entries, err := os.ReadDir(repoDir)
			if err != nil {
				return nil, fmt.Errorf("failed to read deployment repo: %w", err)
			}

			var names []string
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				if _, err := os.Stat(filepath.Join(repoDir, entry.Name(), "deploy.yaml")); err != nil {
					continue
				}
				names = append(names, entry.Name())
			}
			return names, nil
		},
		load: func(environment string) (map[string]any, error) {
			return loadDeployConfig(filepath.Join(repoDir, environment))
		},
	}
}

func gitRefSource(repoDir, ref string) deployConfigSource {
	return deployConfigSource{
		environments: func() ([]string, error) {
			listing, err := runGit(repoDir, "ls-tree", "-r", "--name-only", ref)
			if err != nil {
				return nil, fmt.Errorf("failed to list files in %s: %w", ref, err)
			}

			var names []string
			for _, file := range strings.Split(listing, "\n") {
				if dir, base := path.Split(file); base == "deploy.yaml" && strings.Count(dir, "/") == 1 {
					names = append(names, strings.TrimSuffix(dir, "/"))
				}
			}
			return names, nil
		},
		load: func(environment string) (map[string]any, error) {
			data, err := runGit(repoDir, "show", ref+":"+environment+"/deploy.yaml")
			if err != nil {
				return nil, fmt.Errorf("failed to read deploy.yaml for %s from %s: %w", environment, ref, err)
			}
			return parseDeployConfig([]byte(data))
		},
	}
}

func runGit(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", err
	}
	return string(output), nil
}

func printDeployVersions(source deployConfigSource, out io.Writer, showUnknowns bool) error {
	environments, err := source.environments()
	if err != nil {
		return err
	}

	// tabwriter defers write errors to Flush, which is returned below.
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ENVIRONMENT\tUNDERSTACK\tDEPLOY")

	found := false
	for _, environment := range environments {
		config, err := source.load(environment)
		if err != nil {
			return err
		}

		understackRef, _ := config["understack_ref"].(string)
		if understackRef == "" {
			understackRef = "unknown"
		}

		deployRef, _ := config["deploy_ref"].(string)
		if deployRef == "" {
			deployRef = "unknown"
		}

		found = true

		if !showUnknowns && (understackRef == "unknown" || deployRef == "unknown") {
			continue
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", environment, understackRef, deployRef)
	}

	if !found {
		return fmt.Errorf("no environments with a deploy.yaml found, is this a deployment repo?")
	}

	return w.Flush()
}
