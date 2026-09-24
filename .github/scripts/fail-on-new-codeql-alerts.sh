#!/usr/bin/env bash
# Fails a pull request's CodeQL job when the pull request introduces a CodeQL alert.
#
# github/codeql-action/analyze never fails because of findings, so codeql.yml runs this after it on
# pull_request events. codeql.yml turns off diff-informed analysis, so the uploaded results cover the
# whole codebase, including alerts on lines the pull request did not touch (for example after it deletes
# a guard or a permissions block).
#
# An alert counts as new when it is open on the pull request's merge ref for this job's category and has
# no counterpart on the base branch. Counterparts are matched by rule, file and message rather than by
# alert number, because an existing alert gets a new number when nearby lines change. Base alerts that
# are open or dismissed both count as counterparts; each one matches at most one pull request alert, so
# a second copy of an existing problem is still new. Alerts of every severity count. Dismissing a false
# positive in Security and quality > Code scanning and re-running the job clears it.
#
# When the base branch has no analysis for the category yet (the first run, or a newly added language),
# only alerts whose location is on a line the pull request adds or changes count as new.
#
# If the code scanning API cannot be read (for example with a fork pull request's read-only token), the
# script falls back to the local SARIF and fails on any result on a line the pull request adds or
# changes. That fallback cannot see dismissals or alerts elsewhere.
#
# Environment: GH_TOKEN, GITHUB_REPOSITORY, GITHUB_SHA (the PR merge commit, checked out with both
# parents, fetch-depth 2), CATEGORY (for example /language:go), PR_REF (refs/pull/<n>/merge), BASE_REF
# (refs/heads/<base>), SARIF_DIR (the analyze step's sarif-output), optionally GITHUB_STEP_SUMMARY, and
# optionally CODEQL_GATE_POLL_DELAYS (seconds between checks for GitHub's processing; for tests).
set -euo pipefail

: "${GITHUB_REPOSITORY:?}" "${GITHUB_SHA:?}" "${CATEGORY:?}" "${PR_REF:?}" "${BASE_REF:?}" "${SARIF_DIR:?}"
category="${CATEGORY%/}"
poll_delays="${CODEQL_GATE_POLL_DELAYS:-0 15 30 60 60 120 120 180}"

# Shared jq helpers: workflow-command escaping, URI decoding, line-range tests and alert identity.
# shellcheck disable=SC2016 # a jq program; its $names are jq variables
JQ_LIB='
def esc_data: gsub("%"; "%25") | gsub("\r"; "%0D") | gsub("\n"; "%0A");
def esc_prop: esc_data | gsub(":"; "%3A") | gsub(","; "%2C");
def urldecode: [scan("%[0-9A-Fa-f]{2}|.")]
  | map(if length == 3 and startswith("%")
        then (.[1:] | ascii_downcase | explode | map(if . >= 97 then . - 87 else . - 48 end) | [.[0] * 16 + .[1]] | implode)
        else . end)
  | join("");
def in_ranges($ranges; $path; $line): $ranges | any(.path == $path and $line >= .start and $line <= .end);
def alert_key: [.rule.id, (.most_recent_instance.location.path // ""), (.most_recent_instance.message.text // "")] | tojson;
def for_category($cat): [.[] | select(((.most_recent_instance.category // "") | rtrimstr("/")) == $cat)];
'

summary() {
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '%s\n' "$@" >>"$GITHUB_STEP_SUMMARY"
  fi
}

# Every page of a list endpoint, as one JSON array.
api_list() {
  gh api --paginate "$1" | jq -s 'add // []'
}

# The first page only (newest first); enough to find recent analyses.
api_page() {
  gh api "$1"
}

has_analysis() { # analyses JSON, commit SHA or "" for any commit
  jq -e --arg cat "$category" --arg sha "$2" \
    'any(.[]; ((.category // "") | rtrimstr("/")) == $cat and ($sha == "" or .commit_sha == $sha))' <<<"$1" >/dev/null
}

require_merge_commit() {
  if ! git rev-parse --verify --quiet 'HEAD^2' >/dev/null; then
    echo "::error::Cannot work out the lines this pull request changes: the checkout is not the pull request merge commit with both parents."
    exit 1
  fi
}

# Lines the pull request adds or changes, as [{path, start, end}], from the merge commit against its
# first parent (the base branch). "+++" is only a file header before a file's first hunk.
changed_ranges() {
  git -c core.quotePath=false diff --unified=0 --no-color --no-ext-diff HEAD^1 HEAD |
    awk '
      /^diff --git / { header = 1; path = ""; next }
      header && /^\+\+\+ / {
        p = substr($0, 5)
        sub(/\t$/, "", p)
        path = (p == "/dev/null") ? "" : substr(p, 3)
        next
      }
      /^@@ / {
        header = 0
        if (path == "") next
        match($0, /\+[0-9]+(,[0-9]+)?/)
        split(substr($0, RSTART + 1, RLENGTH - 1), hunk, ",")
        count = (hunk[2] == "") ? 1 : hunk[2]
        if (count > 0) printf "%s\t%d\t%d\n", path, hunk[1], hunk[1] + count - 1
      }' |
    jq -R -s '[split("\n")[] | select(length > 0) | split("\t") | {path: .[0], start: (.[1] | tonumber), end: (.[2] | tonumber)}]'
}

# Fallback when the code scanning API is unreadable: fail on local SARIF results on changed lines.
check_local_sarif() {
  echo "::warning::Could not read the code scanning API for ${PR_REF} (expected on some fork pull requests). Checking the local CodeQL results on the lines this pull request changes instead; dismissed alerts and alerts elsewhere cannot be taken into account."
  require_merge_commit
  local ranges hits count
  ranges=$(changed_ranges)
  hits=$(jq -s --argjson ranges "$ranges" "${JQ_LIB}"'
    [.[] | .runs[]? | .results[]? | select(
      [(.locations // [])[], (.relatedLocations // [])[]] | any(.physicalLocation as $p
        | in_ranges($ranges; ($p.artifactLocation.uri // "") | urldecode; $p.region.startLine // 0)))]' "${sarif_files[@]}")
  count=$(jq 'length' <<<"$hits")
  if [ "$count" -eq 0 ]; then
    echo "No CodeQL results on the lines this pull request changes."
    summary "### CodeQL ${category}" "" "No CodeQL results on the lines this pull request changes (checked locally)."
    exit 0
  fi
  jq -r "${JQ_LIB}"'.[] | (.locations[0].physicalLocation // {}) as $p
    | "::error file=\(($p.artifactLocation.uri // "") | urldecode | esc_prop),line=\($p.region.startLine // 1),title=\(("CodeQL " + (.ruleId // "")) | esc_prop)::\((.message.text // "") | esc_data)"' \
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

# The local SARIF holds every result for the category, so zero results means there is nothing to check.
results=$(jq -s '[.[].runs[]?.results[]?] | length' "${sarif_files[@]}")
echo "CodeQL reported ${results} result(s) for ${category}."
if [ "$results" -eq 0 ]; then
  summary "### CodeQL ${category}" "" "No CodeQL alerts."
  exit 0
fi

# Only trust the alert list once GitHub has processed this exact commit's analysis. analyze stops
# waiting after about 2.5 minutes, so keep polling for a while before giving up.
processed=false
for delay in $poll_delays; do
  sleep "$delay"
  if ! analyses=$(api_page "repos/${GITHUB_REPOSITORY}/code-scanning/analyses?ref=${PR_REF}&tool_name=CodeQL&per_page=100"); then
    check_local_sarif
  fi
  if has_analysis "$analyses" "$GITHUB_SHA"; then
    processed=true
    break
  fi
  echo "Waiting for GitHub to process the ${category} analysis of ${GITHUB_SHA}..."
done
if [ "$processed" != true ]; then
  echo "::error::GitHub has not finished processing the ${category} analysis of ${GITHUB_SHA}. Re-run this job."
  exit 1
fi

if ! pr_alerts=$(api_list "repos/${GITHUB_REPOSITORY}/code-scanning/alerts?ref=${PR_REF}&tool_name=CodeQL&state=open&per_page=100" |
  jq --arg cat "$category" "${JQ_LIB}"'for_category($cat)'); then
  check_local_sarif
fi
if [ "$(jq 'length' <<<"$pr_alerts")" -eq 0 ]; then
  echo "No open CodeQL alerts for ${category} on ${PR_REF}."
  summary "### CodeQL ${category}" "" "No open CodeQL alerts."
  exit 0
fi

# Compare with the base branch when it has been analyzed for this category; otherwise only judge the
# lines the pull request changes.
if base_analyses=$(api_page "repos/${GITHUB_REPOSITORY}/code-scanning/analyses?ref=${BASE_REF}&tool_name=CodeQL&per_page=100") &&
  has_analysis "$base_analyses" "" &&
  base_alerts=$(api_list "repos/${GITHUB_REPOSITORY}/code-scanning/alerts?ref=${BASE_REF}&tool_name=CodeQL&per_page=100" |
    jq --arg cat "$category" "${JQ_LIB}"'for_category($cat) | map(select(.state == "open" or .state == "dismissed"))'); then
  mode="no counterpart open or dismissed on ${BASE_REF}"
  new_alerts=$(jq --argjson base "$base_alerts" "${JQ_LIB}"'
    ($base | map(alert_key) | group_by(.) | map({key: .[0], value: length}) | from_entries) as $counts
    | reduce .[] as $a ({counts: $counts, new: []};
        ($a | alert_key) as $k
        | if (.counts[$k] // 0) > 0 then .counts[$k] -= 1 else .new += [$a] end)
    | .new' <<<"$pr_alerts")
else
  echo "::notice::${BASE_REF} has no CodeQL analysis for ${category} yet (or it could not be read), so only alerts on the lines this pull request changes count as new."
  require_merge_commit
  mode="on a line this pull request changes (${BASE_REF} has no ${category} analysis yet)"
  new_alerts=$(jq --argjson ranges "$(changed_ranges)" "${JQ_LIB}"'
    [.[] | select(in_ranges($ranges; .most_recent_instance.location.path // ""; .most_recent_instance.location.start_line // 0))]' <<<"$pr_alerts")
fi
count=$(jq 'length' <<<"$new_alerts")

if [ "$count" -eq 0 ]; then
  echo "No new CodeQL alerts for ${category}: no open alert on ${PR_REF} is ${mode}."
  summary "### CodeQL ${category}" "" "No new CodeQL alerts."
  exit 0
fi

# One annotation per new alert, then a summary table.
jq -r "${JQ_LIB}"'.[] | (.most_recent_instance.location // {}) as $loc
  | "\(.rule.description) (\(.rule.security_severity_level // .rule.severity)). \(.html_url)" as $msg
  | "::error file=\(($loc.path // "") | esc_prop),line=\($loc.start_line // 1),title=\(("CodeQL " + .rule.id) | esc_prop)::\($msg | esc_data)"' \
  <<<"$new_alerts"
summary "### CodeQL ${category}: ${count} new alert(s)" "" \
  "| Alert | Rule | Severity | Location |" "| --- | --- | --- | --- |"
jq -r '.[] | "| [#\(.number)](\(.html_url)) | `\(.rule.id)` | \(.rule.security_severity_level // .rule.severity) | `\(.most_recent_instance.location.path):\(.most_recent_instance.location.start_line)` |"' \
  <<<"$new_alerts" | while IFS= read -r row; do summary "$row"; done

echo "::error::This pull request introduces ${count} CodeQL alert(s) for ${category} (${mode}). Fix them, or dismiss false positives in Security and quality > Code scanning and re-run this job."
exit 1
