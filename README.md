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

Requires Go 1.24+.

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
| `kubeseal` | all secret-generating commands, `quickstart` |
| `helm` (3.8+) | `deploy render` |
| `git` | `deploy init`, `deploy render` |
| `argo` | `node enroll-server`, `node inspect-server` |
| `kubectl` / a valid kubeconfig | anything that talks to a cluster |

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
[GoReleaser](https://goreleaser.com/install/) on your `PATH`. Both also run in
CI on every pull request, so a break in the release pipeline shows up before
you tag.

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
