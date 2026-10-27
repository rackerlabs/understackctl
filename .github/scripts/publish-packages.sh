#!/usr/bin/env bash
# Renders the Homebrew formula and Scoop manifest for a release.
#
# Usage: publish-packages.sh <tag> <checksums.txt> <tap-dir> <bucket-dir>
set -euo pipefail

tag=$1
checksums=$2
tap_dir=$3
bucket_dir=$4

version=${tag#v}
base_url="https://github.com/rackerlabs/understackctl/releases/download/${tag}"

sha() {
  local sum
  sum=$(awk -v f="understackctl_$1" '$2 == f { print $1 }' "$checksums")
  if [ -z "$sum" ]; then
    echo "no checksum found for understackctl_$1" >&2
    exit 1
  fi
  echo "$sum"
}

mkdir -p "$tap_dir/Formula" "$bucket_dir/bucket"

cat > "$tap_dir/Formula/understackctl.rb" <<RUBY
class Understackctl < Formula
  desc "CLI tool for managing UnderStack deployments"
  homepage "https://github.com/rackerlabs/understackctl"
  version "${version}"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "${base_url}/understackctl_darwin_arm64.tar.gz"
      sha256 "$(sha darwin_arm64.tar.gz)"
    end
    on_intel do
      url "${base_url}/understackctl_darwin_amd64.tar.gz"
      sha256 "$(sha darwin_amd64.tar.gz)"
    end
  end

  on_linux do
    on_arm do
      url "${base_url}/understackctl_linux_arm64.tar.gz"
      sha256 "$(sha linux_arm64.tar.gz)"
    end
    on_intel do
      url "${base_url}/understackctl_linux_amd64.tar.gz"
      sha256 "$(sha linux_amd64.tar.gz)"
    end
  end

  def install
    bin.install "understackctl"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/understackctl --version")
  end
end
RUBY

cat > "$bucket_dir/bucket/understackctl.json" <<JSON
{
  "version": "${version}",
  "description": "CLI tool for managing UnderStack deployments",
  "homepage": "https://github.com/rackerlabs/understackctl",
  "license": "Apache-2.0",
  "architecture": {
    "64bit": {
      "url": "${base_url}/understackctl_windows_amd64.zip",
      "hash": "$(sha windows_amd64.zip)",
      "extract_dir": "understackctl_windows_amd64"
    },
    "32bit": {
      "url": "${base_url}/understackctl_windows_386.zip",
      "hash": "$(sha windows_386.zip)",
      "extract_dir": "understackctl_windows_386"
    },
    "arm64": {
      "url": "${base_url}/understackctl_windows_arm64.zip",
      "hash": "$(sha windows_arm64.zip)",
      "extract_dir": "understackctl_windows_arm64"
    }
  },
  "bin": "understackctl.exe"
}
JSON
