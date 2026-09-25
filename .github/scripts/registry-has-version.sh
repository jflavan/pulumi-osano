#!/usr/bin/env bash
# Reports whether an exact package version is already published, so a re-run of a partially failed
# release can skip the registries that already have it. Read-only: it never publishes anything.
#
# Usage: registry-has-version.sh <npm|maven|pypi|nuget> <package> <version>
#   npm    <package> is the npm name, for example @jflavan/pulumi-osano
#   maven  <package> is groupId:artifactId, for example io.github.jflavan.pulumi:pulumi-osano
#   pypi   <package> is the PyPI project, for example pulumi-osano (<version> in PEP 440 form)
#   nuget  <package> is the NuGet package id, for example Community.Pulumi.Osano
#
# Prints the answer and, when GITHUB_OUTPUT is set, writes published=true or published=false to it.
# A lookup that fails for any other reason than "not found" counts as not published and prints a
# warning: the publish then runs, and every registry rejects a version it already has, so a failed
# lookup can fail a job but never publishes a version twice. Build metadata (+...) is ignored because
# registries store versions without it. Exits non-zero only for usage errors.
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <npm|maven|pypi|nuget> <package> <version>" >&2
  exit 2
fi
registry=$1
package=$2
version=${3%%+*}

# Prints the final HTTP status of a GET request, or 000 when the request itself failed.
http_status() {
  local code
  code=$(curl --silent --show-error --location --retry 3 --max-time 60 \
    --output /dev/null --write-out '%{http_code}' "$1") || code=000
  echo "$code"
}

# Maps an HTTP status to published=true (200) or false (404); anything else is a failed lookup.
from_status() {
  local url=$1 code
  code=$(http_status "$url")
  case "$code" in
    200) published=true ;;
    404) published=false ;;
    *)
      echo "::warning::Could not tell whether ${package} ${version} is on ${registry} (HTTP ${code} from ${url}); treating it as not published."
      published=false
      ;;
  esac
}

published=false
case "$registry" in
  npm)
    # `npm view` prints the version when it exists, prints nothing when the package exists without that
    # version, and fails with E404 when the package does not exist at all.
    err=$(mktemp)
    if out=$(npm view "${package}@${version}" version 2>"$err"); then
      [[ "$out" == "$version" ]] && published=true
    elif ! grep -q E404 "$err"; then
      echo "::warning::npm view ${package}@${version} failed; treating it as not published."
      cat "$err" >&2
    fi
    rm -f "$err"
    ;;
  maven)
    if [[ "$package" != *:* ]]; then
      echo "maven packages are groupId:artifactId, got ${package}" >&2
      exit 2
    fi
    group=${package%%:*}
    artifact=${package#*:}
    # repo1 is the canonical Maven Central repository. A version appears there some minutes after the
    # Central Portal releases it, so a re-run soon after a Java publish may not see it yet.
    from_status "https://repo1.maven.org/maven2/${group//.//}/${artifact}/${version}/${artifact}-${version}.pom"
    ;;
  pypi)
    from_status "https://pypi.org/pypi/${package}/${version}/json"
    ;;
  nuget)
    id=${package,,}
    lower_version=${version,,}
    from_status "https://api.nuget.org/v3-flatcontainer/${id}/${lower_version}/${id}.nuspec"
    ;;
  *)
    echo "unknown registry: ${registry}" >&2
    exit 2
    ;;
esac

if [[ "$published" == true ]]; then
  echo "::notice::${package} ${version} is already published on ${registry}."
else
  echo "${package} ${version} is not published on ${registry} yet."
fi
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  echo "published=${published}" >> "$GITHUB_OUTPUT"
fi
