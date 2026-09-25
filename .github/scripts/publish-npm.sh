#!/usr/bin/env bash
# Publishes the built Node.js SDK to npm, or shows what a publish would do with --dry-run.
#
# Usage: publish-npm.sh [--dry-run] <package-dir>
#
# <package-dir> is the compiled package that `make build_nodejs` assembles in sdk/nodejs/bin
# (package.json, README.md, LICENSE and the .js/.d.ts output). Do not pass sdk/nodejs: it holds only
# the TypeScript sources, and its .gitignore excludes bin/, so npm would publish no JavaScript.
#
# Authentication (real publishes only): npm 11.5+ first tries trusted publishing, exchanging the job's
# GitHub OIDC token (id-token: write) for a short-lived npm token. When that exchange fails, for example
# because the package has no trusted publisher yet, npm falls back to the token in the .npmrc that
# actions/setup-node writes, which reads NODE_AUTH_TOKEN (the temporary NPM_TOKEN secret). When the
# exchange succeeds its token replaces the .npmrc token, so a leftover or empty NPM_TOKEN never gets in
# the way of trusted publishing.
#
# Re-runs: when this exact version is already on npm, the script publishes nothing and succeeds.
set -euo pipefail

dry_run=false
if [[ "${1:-}" == "--dry-run" ]]; then
  dry_run=true
  shift
fi
if [[ $# -ne 1 ]]; then
  echo "usage: $0 [--dry-run] <package-dir>" >&2
  exit 2
fi

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
cd "$1"
if [[ ! -f package.json || ! -f index.js ]]; then
  echo "::error::$(pwd) is not a built Node.js package (package.json and index.js are required); run make build_nodejs first."
  exit 1
fi

name=$(node -p 'require("./package.json").name')
version=$(node -p 'require("./package.json").version')

# Prerelease versions need an explicit dist-tag so they never become "latest".
case "$version" in
  *-alpha*) tag=alpha ;;
  *-beta*) tag=beta ;;
  *-rc*) tag=rc ;;
  *) tag=latest ;;
esac

result=$(mktemp)
GITHUB_OUTPUT="$result" bash "$script_dir/registry-has-version.sh" npm "$name" "$version"
if grep -qx 'published=true' "$result"; then
  rm -f "$result"
  if [[ "$dry_run" == true ]]; then
    echo "Dry run: a release would skip npm because ${name}@${version} is already published. Package contents:"
    npm pack --dry-run
  fi
  exit 0
fi
rm -f "$result"

if [[ "$dry_run" == true ]]; then
  echo "Dry run: a release would publish ${name}@${version} with dist-tag ${tag}."
  npm publish --dry-run --access public --tag "$tag"
else
  echo "Publishing ${name}@${version} with dist-tag ${tag}."
  npm publish --access public --tag "$tag" --provenance
fi
