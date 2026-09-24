#!/usr/bin/env bash
# Fails a pull request's CodeQL job when the pull request introduces a CodeQL alert.
#
# github/codeql-action/analyze never fails because of findings, so codeql.yml runs this after it on
# pull_request events. An alert counts as new when it is open on the pull request's merge ref for this
# job's category and is not already open on the base branch. Alerts of every severity count. Dismissed
# alerts do not count, so after dismissing a false positive in Security and quality > Code scanning,
# re-running the job clears it.
#
# If the code scanning API cannot be read (for example with a fork pull request's read-only token), the
# script falls back to the local SARIF: it fails when any result is on a line the pull request adds or
# changes, the same filter codeql-action applies before uploading. That fallback cannot see dismissals.
#
# Environment: GH_TOKEN, GITHUB_REPOSITORY, GITHUB_SHA (the PR merge commit, checked out with its two
# parents, fetch-depth 2), CATEGORY (for example /language:go), PR_REF (refs/pull/<n>/merge), BASE_REF
# (refs/heads/<base>), SARIF_DIR (the analyze step's sarif-output), and optionally GITHUB_STEP_SUMMARY.
set -euo pipefail

: "${GITHUB_REPOSITORY:?}" "${GITHUB_SHA:?}" "${CATEGORY:?}" "${PR_REF:?}" "${BASE_REF:?}" "${SARIF_DIR:?}"
category="${CATEGORY%/}"

summary() {
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '%s\n' "$@" >>"$GITHUB_STEP_SUMMARY"
  fi
}

# All pages of a list endpoint as one JSON array.
api_list() {
  gh api --paginate "$1" | jq -s 'add // []'
}

# Open CodeQL alerts on a ref, limited to this job's category.
open_alerts() {
  api_list "repos/${GITHUB_REPOSITORY}/code-scanning/alerts?ref=$1&tool_name=CodeQL&state=open&per_page=100" |
    jq --arg cat "$category" '[.[] | select(((.most_recent_instance.category // "") | rtrimstr("/")) == $cat)]'
}

# Lines the pull request adds or changes, as [{path, start, end}], from the merge commit against its
# first parent (the base branch).
changed_ranges() {
  git diff --unified=0 --no-color --no-ext-diff HEAD^1 HEAD |
    awk '
      /^\+\+\+ / { path = ($2 == "/dev/null") ? "" : substr($0, 7); next }
      /^@@ / && path != "" {
        match($0, /\+[0-9]+(,[0-9]+)?/)
        split(substr($0, RSTART + 1, RLENGTH - 1), hunk, ",")
        count = (hunk[2] == "") ? 1 : hunk[2]
        if (count > 0) printf "%s\t%d\t%d\n", path, hunk[1], hunk[1] + count - 1
      }' |
    jq -R -s '[split("\n")[] | select(length > 0) | split("\t") | {path: .[0], start: (.[1] | tonumber), end: (.[2] | tonumber)}]'
}

# Fallback when the code scanning API is unreadable: fail on local SARIF results on changed lines.
check_local_sarif() {
  echo "::warning::Could not read the code scanning API for ${PR_REF} (expected on some fork pull requests). Checking the local CodeQL results on the lines this pull request changes instead; dismissed alerts cannot be taken into account."
  if ! git rev-parse --verify --quiet 'HEAD^2' >/dev/null; then
    echo "::error::Cannot work out the lines this pull request changes: the checkout is not the pull request merge commit with both parents."
    exit 1
  fi
  local ranges hits count
  ranges=$(changed_ranges)
  hits=$(jq -s --argjson ranges "$ranges" '[.[] | .runs[]? | .results[]? | select(
      [(.locations // [])[], (.relatedLocations // [])[]] | any(.physicalLocation as $p
        | ($p.artifactLocation.uri // "") as $uri | ($p.region.startLine // 0) as $line
        | $ranges | any(.path == $uri and $line >= .start and $line <= .end)))]' "${sarif_files[@]}")
  count=$(jq 'length' <<<"$hits")
  if [ "$count" -eq 0 ]; then
    echo "No CodeQL results on the lines this pull request changes."
    summary "### CodeQL ${category}" "" "No CodeQL results on the lines this pull request changes (checked locally)."
    exit 0
  fi
  jq -r '.[] | (.locations[0].physicalLocation // {}) as $p
    | "::error file=\($p.artifactLocation.uri // ""),line=\($p.region.startLine // 1),title=CodeQL \(.ruleId)::\(.message.text | gsub("%"; "%25") | gsub("\r?\n"; " "))"' \
    <<<"$hits"
  summary "### CodeQL ${category}: ${count} result(s) on changed lines" "" "Checked locally because the code scanning API was unreadable."
  echo "::error::This pull request has ${count} CodeQL result(s) for ${category} on the lines it changes. Fix them; a maintainer can review false positives."
  exit 1
}

shopt -s nullglob
sarif_files=("$SARIF_DIR"/*.sarif)
if [ "${#sarif_files[@]}" -eq 0 ]; then
  echo "::error::No SARIF output in '$SARIF_DIR', so the CodeQL results for $category cannot be verified."
  exit 1
fi

# The local SARIF can contain results anywhere in the code (the Actions queries do not restrict
# themselves to the diff); the upload keeps only results on lines the pull request changes. Zero local
# results therefore means nothing new, so skip the API in that case.
results=$(jq -s '[.[].runs[]?.results[]?] | length' "${sarif_files[@]}")
echo "CodeQL reported ${results} result(s) for ${category}."
if [ "$results" -eq 0 ]; then
  summary "### CodeQL ${category}" "" "No CodeQL alerts in the code this pull request changes."
  exit 0
fi

if ! analyses=$(api_list "repos/${GITHUB_REPOSITORY}/code-scanning/analyses?ref=${PR_REF}&tool_name=CodeQL&per_page=100"); then
  check_local_sarif
fi
# Only trust the alert list once GitHub has processed this exact commit's analysis.
if ! jq -e --arg cat "$category" --arg sha "$GITHUB_SHA" \
  'any(.[]; ((.category // "") | rtrimstr("/")) == $cat and .commit_sha == $sha)' <<<"$analyses" >/dev/null; then
  echo "::error::GitHub has not finished processing the ${category} analysis of ${GITHUB_SHA}. Re-run this job."
  exit 1
fi

if ! pr_alerts=$(open_alerts "$PR_REF"); then
  check_local_sarif
fi
# Alerts already open on the base branch are existing debt, not something this pull request added.
if ! base_alerts=$(open_alerts "$BASE_REF"); then
  echo "::warning::Could not read the code scanning alerts for ${BASE_REF}; treating every open alert as new."
  base_alerts='[]'
fi

new_alerts=$(jq --argjson base "$base_alerts" \
  '($base | map(.number)) as $old | [.[] | select(.number as $n | ($old | any(. == $n)) | not)]' <<<"$pr_alerts")
count=$(jq 'length' <<<"$new_alerts")

if [ "$count" -eq 0 ]; then
  echo "No new CodeQL alerts: every result is outside the lines this pull request changes, dismissed, or already open on ${BASE_REF}."
  summary "### CodeQL ${category}" "" "No new CodeQL alerts. Every result is outside the lines this pull request changes, dismissed, or already open on \`${BASE_REF}\`."
  exit 0
fi

# One annotation per alert on the changed line, then a summary table.
jq -r '.[] | (.most_recent_instance.location // {}) as $loc
  | "\(.rule.description) (\(.rule.security_severity_level // .rule.severity)). \(.html_url)" as $msg
  | "::error file=\($loc.path // ""),line=\($loc.start_line // 1),title=CodeQL \(.rule.id)::\($msg | gsub("%"; "%25"))"' \
  <<<"$new_alerts"
summary "### CodeQL ${category}: ${count} new alert(s)" "" \
  "| Alert | Rule | Severity | Location |" "| --- | --- | --- | --- |"
jq -r '.[] | "| [#\(.number)](\(.html_url)) | `\(.rule.id)` | \(.rule.security_severity_level // .rule.severity) | `\(.most_recent_instance.location.path):\(.most_recent_instance.location.start_line)` |"' \
  <<<"$new_alerts" | while IFS= read -r row; do summary "$row"; done

echo "::error::This pull request introduces ${count} CodeQL alert(s) for ${category}. Fix them, or dismiss false positives in Security and quality > Code scanning and re-run this job."
exit 1
