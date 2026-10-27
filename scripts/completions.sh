#!/bin/sh
# Generate shell completions into completions/ for inclusion in release
# archives. Run by the GoReleaser `before` hook; safe to run by hand.
set -eu

rm -rf completions
mkdir completions

for sh in bash zsh fish powershell; do
	go run . completion "$sh" >"completions/understackctl.$sh"
done
