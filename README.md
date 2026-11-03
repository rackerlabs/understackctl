# understackctl

`understackctl` is the CLI tool for managing [UnderStack](https://github.com/rackerlabs/understack)
deployments. It handles deployment repository scaffolding, secret generation,
hardware definitions and node lifecycle workflows.

For the full command reference, see the
[documentation](https://rackerlabs.github.io/understack/operator-guide/understackctl/).

## Installation

Every release publishes binaries for Linux, macOS and Windows on amd64 and
arm64. Pick whichever method suits you.

### macOS — Homebrew

```bash
brew install rackerlabs/tap/understackctl
```

### Windows — Scoop

```powershell
scoop bucket add rackerlabs https://github.com/rackerlabs/scoop-bucket
scoop install understackctl
```

### Manual download

Grab the archive for your platform from the
[releases page](https://github.com/rackerlabs/understackctl/releases):

| Platform | Archive |
|---|---|
| Linux | `understackctl_<version>_linux_{amd64,arm64}.tar.gz` |
| macOS | `understackctl_<version>_darwin_all.tar.gz` (runs on both Intel and Apple Silicon) |
| Windows | `understackctl_<version>_windows_{amd64,arm64}.zip` |

The binary sits at the root of the archive, alongside `LICENSE`, `README.md`
and a `completions/` directory.

```bash
tar xzf understackctl_*.tar.gz
sudo mv understackctl /usr/local/bin/
```

Verify what you downloaded. Checksums:

```bash
sha256sum --check --ignore-missing checksums.txt
```

Every archive also carries a GitHub build provenance attestation, which proves
it was built by this repository's release workflow from a specific commit:

```bash
gh attestation verify understackctl_0.1.0_linux_amd64.tar.gz --repo rackerlabs/understackctl
```

### go install

Requires Go 1.26+.

```bash
go install github.com/rackerlabs/understackctl@latest
```

This puts the binary in `$GOPATH/bin` (`$HOME/go/bin` by default); make sure
that directory is on your `PATH`.

### From source

```bash
git clone https://github.com/rackerlabs/understackctl.git
cd understackctl
make build      # -> build/understackctl
make install    # -> $GOBIN
```

## Verifying your install

```console
$ understackctl version
Version:    v0.1.0
Commit:     630c1e9
Built:      2026-09-06T03:29:01Z
Go version: go1.24.7
Platform:   linux/amd64
```

Include this output when reporting a bug. `understackctl version -o json` emits
the same information as JSON.

## Shell completion

Release archives include pre-generated completions under `completions/`, or
generate them yourself:

```bash
# bash (current shell)
source <(understackctl completion bash)
# bash (permanent, Linux)
understackctl completion bash | sudo tee /etc/bash_completion.d/understackctl

# zsh — ensure `autoload -U compinit && compinit` is in your ~/.zshrc first
understackctl completion zsh > "${fpath[1]}/_understackctl"

# fish
understackctl completion fish > ~/.config/fish/completions/understackctl.fish

# PowerShell — append to your profile to make it permanent
understackctl completion powershell | Out-String | Invoke-Expression
```

## External dependencies

`understackctl` shells out to other tools. You only need the ones used by the
commands you run:

| Tool | Needed by |
|---|---|
| `helm` (3.8+) | `deploy render` |
| `git` | `deploy init`, `deploy render` |
| `argo` | `workflow enroll-server`, `workflow enroll-netdev`, `workflow enroll-fw`, `workflow inspect-server`, `workflow run`, `workflow list` |
| `kubectl` / a valid kubeconfig | anything that talks to a cluster |

## Server enrollment options

Only the BMC IP address is required. All flags are optional. Enrollment uses
the current Kubernetes context and the installed `enroll-server` WorkflowTemplate.

| Argument / flag | Purpose | Default when omitted |
|---|---|---|
| `<ip-address>` | BMC IPv4 or IPv6 address (**required**) | None |
| `--old-password` | Existing BMC password | `calvin` |
| `--firmware-update` | Update firmware | `false` |
| `--raid-configure` | Configure RAID | `true`; use `--raid-configure=false` to disable |
| `--external-cmdb-id` | External CMDB identifier | Empty |
| `--rack` | Rack value in node extra metadata | Empty |
| `--position` | Position value in node extra metadata | Empty |
| `--post-inspection-runbooks` | JSON array of runbooks to run after inspection | Site ConfigMap value, or `[]` |
| `--post-enroll-workflows` | JSON array of WorkflowTemplate names to run after enrollment | Site ConfigMap value, or `[]` |
| `--log` | Follow workflow logs after submission | `false` |

Workflow parameter defaults above come from the current Understack template;
omitted flags leave the installed template's defaults in effect. For either
hook flag, pass `'[]'` to disable those hooks for this enrollment.

Example with every flag:

```bash
understackctl workflow enroll-server 192.0.2.10 \
  --old-password '<existing-bmc-password>' \
  --firmware-update=false \
  --raid-configure=true \
  --external-cmdb-id cmdb-123 \
  --rack rack-42 \
  --position 12 \
  --post-inspection-runbooks '[]' \
  --post-enroll-workflows '["my-post-enrollment-workflow"]' \
  --log
```

`--post-enroll-workflows` requires an installed `enroll-server` WorkflowTemplate
that exposes `post_enroll_workflows` as a workflow-level parameter.
Each listed template must expose a `run` template accepting `node_uuid`.

## Run a WorkflowTemplate

Use `workflow run` to submit any installed WorkflowTemplate in `argo-events`
using the current Kubernetes context. Pass parameters with repeatable
`--parameter` (or `-p`) flags:

```bash
understackctl workflow run my-workflow \
  --parameter node_uuid=<node-uuid> \
  --parameter 'options=["first","second"]' --log
```

Parameter values are passed unchanged, including JSON arrays and empty values
(`--parameter name=`). Omitted parameters use the template's defaults.
The command prints the workflow name and Argo UI link; `--log` follows logs.

For example, if your cluster has an `enable-obm-monitoring` WorkflowTemplate:

```bash
understackctl workflow run enable-obm-monitoring -p node_uuid=<uuid>
```

## Development

```bash
make help          # list targets
make test          # go test ./...
make lint          # golangci-lint
make fmt-check     # fail if not gofmt-clean
make snapshot      # build all release artifacts into dist/, publishing nothing
make check         # validate .goreleaser.yaml
```

`make snapshot` and `make check` need
[GoReleaser](https://goreleaser.com/install/) on your `PATH`. `make check` also
runs in CI on every pull request. Run `make snapshot` yourself before tagging
if you have changed the release configuration.

### Releasing

Releases are cut by pushing a tag; everything else is automated by
[`.github/workflows/release.yaml`](.github/workflows/release.yaml).

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Pre-release tags (`v0.1.0-rc.1`) are published as GitHub pre-releases and are
not marked "latest", so they can be tested before a real release.

## Contributing

Please open an issue or pull request on
[GitHub](https://github.com/rackerlabs/understackctl). Commit messages follow
[Conventional Commits](https://www.conventionalcommits.org/) — the release
changelog is generated from them.

## License

Apache 2.0. See [LICENSE](LICENSE).
