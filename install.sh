#!/bin/sh
# Install understackctl on Linux or macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/rackerlabs/understackctl/main/install.sh | sh
#
# Environment variables:
#   VERSION      version to install, e.g. v0.1.0 (default: latest release)
#   INSTALL_DIR  directory to install into (default: first writable of
#                /usr/local/bin, $HOME/.local/bin)
#   BASE_URL     download artifacts from somewhere other than GitHub, for an
#                internal mirror. Requires VERSION to be set, since there is
#                no "latest" to resolve. Expects the release artifacts to sit
#                directly under this URL.
#   NO_VERIFY    set to 1 to skip checksum verification (not recommended)
#
# Windows users: use Scoop, or download the .zip from the releases page.
#
# Piping a script from the internet into a shell means trusting this file and
# the connection that fetched it. To review it first:
#   curl -fsSLO https://raw.githubusercontent.com/rackerlabs/understackctl/main/install.sh
#   less install.sh && sh install.sh

set -eu

REPO="rackerlabs/understackctl"
BINARY="understackctl"

info() { printf '==> %s\n' "$*" >&2; }
err() { printf 'error: %s\n' "$*" >&2; exit 1; }

need() {
	command -v "$1" >/dev/null 2>&1 || err "$1 is required but was not found in PATH"
}

detect_platform() {
	os=$(uname -s)
	arch=$(uname -m)

	case "$os" in
		Linux) os=linux ;;
		Darwin) os=darwin ;;
		MINGW* | MSYS* | CYGWIN*)
			err "Windows is not supported by this script; use Scoop or download the .zip from https://github.com/$REPO/releases"
			;;
		*) err "unsupported operating system: $os" ;;
	esac

	case "$arch" in
		x86_64 | amd64) arch=amd64 ;;
		arm64 | aarch64) arch=arm64 ;;
		*) err "unsupported architecture: $arch (understackctl publishes amd64 and arm64)" ;;
	esac

	PLATFORM="${os}_${arch}"
}

# Resolves the latest release tag by following the redirect on the
# /releases/latest URL, which avoids both an API token and a JSON parser.
latest_version() {
	url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
		err "could not reach GitHub to determine the latest version"

	tag=${url##*/}
	case "$tag" in
		v*) printf '%s\n' "$tag" ;;
		*) err "could not parse a version tag out of '$url'" ;;
	esac
}

# Picks the first writable directory, preferring a system-wide install.
choose_install_dir() {
	if [ -n "${INSTALL_DIR:-}" ]; then
		printf '%s\n' "$INSTALL_DIR"
		return
	fi

	for dir in /usr/local/bin "$HOME/.local/bin"; do
		if [ -d "$dir" ] && [ -w "$dir" ]; then
			printf '%s\n' "$dir"
			return
		fi
	done

	# Nothing writable: fall back to a user directory and create it.
	printf '%s\n' "$HOME/.local/bin"
}

verify_checksum() {
	archive=$1
	checksums=$2

	if [ "${NO_VERIFY:-0}" = "1" ]; then
		info "skipping checksum verification (NO_VERIFY=1)"
		return
	fi

	expected=$(awk -v name="$(basename "$archive")" '$2 == name { print $1 }' "$checksums")
	[ -n "$expected" ] || err "$(basename "$archive") is not listed in checksums.txt"

	if command -v sha256sum >/dev/null 2>&1; then
		actual=$(sha256sum "$archive" | cut -d' ' -f1)
	elif command -v shasum >/dev/null 2>&1; then
		actual=$(shasum -a 256 "$archive" | cut -d' ' -f1)
	else
		err "neither sha256sum nor shasum is available; re-run with NO_VERIFY=1 to install anyway"
	fi

	[ "$actual" = "$expected" ] ||
		err "checksum mismatch for $(basename "$archive"): expected $expected, got $actual"

	info "checksum verified"
}

main() {
	need curl
	need tar
	need awk

	detect_platform

	if [ -n "${BASE_URL:-}" ] && [ -z "${VERSION:-}" ]; then
		err "BASE_URL requires VERSION to be set; a mirror has no 'latest' to resolve"
	fi

	version=${VERSION:-$(latest_version)}
	case "$version" in
		v*) ;;
		*) version="v$version" ;;
	esac

	# Archive names carry the version without the leading "v".
	archive_name="${BINARY}_${version#v}_${PLATFORM}.tar.gz"
	base_url=${BASE_URL:-"https://github.com/$REPO/releases/download/$version"}

	info "installing $BINARY $version ($PLATFORM)"

	tmp=$(mktemp -d)
	# shellcheck disable=SC2064 # expand tmp now, not at trap time
	trap "rm -rf '$tmp'" EXIT INT TERM

	curl -fsSL -o "$tmp/$archive_name" "$base_url/$archive_name" ||
		err "could not download $base_url/$archive_name (does version $version publish a $PLATFORM build?)"

	if [ "${NO_VERIFY:-0}" != "1" ]; then
		curl -fsSL -o "$tmp/checksums.txt" "$base_url/checksums.txt" ||
			err "could not download checksums.txt from $base_url"
		verify_checksum "$tmp/$archive_name" "$tmp/checksums.txt"
	fi

	tar -xzf "$tmp/$archive_name" -C "$tmp"
	[ -f "$tmp/$BINARY" ] || err "archive did not contain a $BINARY binary"
	chmod 0755 "$tmp/$BINARY"

	dir=$(choose_install_dir)
	mkdir -p "$dir" || err "could not create $dir"

	if [ -w "$dir" ]; then
		mv "$tmp/$BINARY" "$dir/$BINARY"
	elif command -v sudo >/dev/null 2>&1; then
		info "$dir is not writable, escalating with sudo"
		sudo mv "$tmp/$BINARY" "$dir/$BINARY"
	else
		err "$dir is not writable and sudo is unavailable; set INSTALL_DIR to somewhere you can write"
	fi

	info "installed $dir/$BINARY"

	case ":$PATH:" in
		*":$dir:"*) ;;
		*)
			info "note: $dir is not on your PATH; add it with"
			info "  export PATH=\"$dir:\$PATH\""
			;;
	esac

	"$dir/$BINARY" version || true

	info "shell completion: $BINARY completion --help"
}

main "$@"
