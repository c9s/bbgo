#!/usr/bin/env bash
#
# ci-logs.sh - find out why a GitHub Actions run failed, without reading 2000
# lines of runner noise.
#
# See agent/scripts/README.md for the usage and
# agent/design/0002-ci-log-inspection.md for why it works the way it does.

set -euo pipefail

SELF="$(basename "$0")"

CONTEXT=6
MODE=auto # auto | status | full
TARGET=""
JOB_ID=""
DO_RERUN=0
DO_LIST=0
LIMIT=10
MAX_LINES=200
WORKFLOW="go.yml"

usage() {
  cat <<USAGE
$SELF - explain a failing GitHub Actions run

Usage:
  $SELF [options] [<pr-number> | <branch>]

With no target, the pull request of the current branch is used.

Options:
  --job <id>        inspect one job id directly, skipping the check lookup
  --status          print only the check status table
  --full            print the whole job log instead of the extracted failures
  --context <n>     log lines to print after each failure headline (default $CONTEXT)
  --max-lines <n>   cap the extracted output (default $MAX_LINES, 0 means no cap)
  --list            list the recent workflow runs of the branch
  --limit <n>       how many runs --list shows (default $LIMIT)
  --workflow <f>    workflow file for --list (default $WORKFLOW)
  --rerun           rerun the failed jobs of the run, for an infrastructure flake
  -h, --help        this text

Examples:
  $SELF                      # the current branch's PR
  $SELF 2772                 # a pull request
  $SELF main --list          # recent runs on main
  $SELF --job 129054160489   # one job
  $SELF 2767 --rerun         # retry an infrastructure failure
USAGE
}

die() {
  echo "$SELF: $*" >&2
  exit 1
}

while [ $# -gt 0 ]; do
  case "$1" in
    --job) JOB_ID="${2:-}"; shift 2 ;;
    --status) MODE=status; shift ;;
    --full) MODE=full; shift ;;
    --context) CONTEXT="${2:-}"; shift 2 ;;
    --max-lines) MAX_LINES="${2:-}"; shift 2 ;;
    --list) DO_LIST=1; shift ;;
    --limit) LIMIT="${2:-}"; shift 2 ;;
    --workflow) WORKFLOW="${2:-}"; shift 2 ;;
    --rerun) DO_RERUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    -*) die "unknown option $1 (try --help)" ;;
    *) TARGET="$1"; shift ;;
  esac
done

command -v gh >/dev/null 2>&1 || die "the gh CLI is required: https://cli.github.com"

CACHE="$(mktemp -d "${TMPDIR:-/tmp}/ci-logs.XXXXXX")"
trap 'rm -rf "$CACHE"' EXIT

# ---------------------------------------------------------------------------
# failure headlines. Anything matching one of these is printed with context.
#
# Note the patterns are passed to grep with -e so the leading dashes of
# "--- FAIL" are never mistaken for an option, which BSD grep does.
# ---------------------------------------------------------------------------
HEADLINES='--- FAIL|FAIL[[:space:]]+github\.com/|^[[:space:]]*FAIL$|##\[error\]|Install blocked by Aikido|missing go\.sum entry|cannot find (package|module)|undefined: |panic: |\[build failed\]|\[setup failed\]|Unexpected HTTP response|Failed to download|no required module provides'

# classify prints a one-line verdict and a hint, most specific first.
classify() {
  local log="$1"

  if grep -q "Install blocked by Aikido" "$log"; then
    echo "VERDICT  dependency blocked by Aikido Device Protection (not a code failure)"
    echo "HINT     run 'aikidoctl events --json' locally; min_package_age clears on its own"
    return
  fi

  if grep -qE -e "Unexpected HTTP response: 5[0-9][0-9]|Failed to download" "$log"; then
    echo "VERDICT  GitHub Actions infrastructure failure (not a code failure)"
    echo "HINT     rerun it: $SELF <target> --rerun"
    return
  fi

  if grep -qE -e "missing go\.sum entry|no required module provides|cannot find (package|module)" "$log"; then
    echo "VERDICT  dependency resolution failure"
    echo "HINT     run 'go mod tidy' locally, or 'go get <module>' for the named one"
    return
  fi

  if grep -q -e "--- FAIL" "$log"; then
    echo "VERDICT  test failure"

    # the step name is the reliable signal for which test set failed: both the
    # Test and the TestDnum steps write a coverage profile, so the coverage
    # line says nothing about the build tags
    local steps
    steps="$(awk -F'\t' '/--- FAIL/ {print $2}' "$log" | sort -u | tr '\n' ' ')"
    [ -n "${steps// /}" ] && echo "STEP     ${steps}"

    case "$steps" in
      *TestDnum*)
        echo "HINT     reproduce with: go test -tags dnum -count=1 ./pkg/..." ;;
      *UNKNOWN*|"")
        # older logs lose the step grouping, and both test steps write a
        # coverage profile, so neither signal says which build tags ran
        echo "HINT     the step is unattributed, so try both test sets:"
        echo "HINT       go test -count 3 -race ./pkg/..." 
        echo "HINT       go test -tags dnum -count=1 ./pkg/..." ;;
      *)
        echo "HINT     reproduce the way CI does: go test -count 3 -race ./pkg/..." ;;
    esac

    echo "TESTS    $(awk -F'\t' '/--- FAIL/ {sub(/.*--- FAIL: /, ""); sub(/ \(.*/, ""); print}' "$log" \
      | sort -u | tr '\n' ' ')"
    echo "PACKAGES $(grep -oE -e "FAIL[[:space:]]+github\.com/[^[:space:]]+" "$log" \
      | awk '{print $2}' | sort -u | tr '\n' ' ')"
    return
  fi

  if grep -qE -e "\[build failed\]|\[setup failed\]|undefined: " "$log"; then
    echo "VERDICT  compile failure"
    echo "HINT     reproduce with: go build ./... && go vet ./..."
    return
  fi

  if grep -q -e "##\[error\]" "$log"; then
    echo "VERDICT  step failed without a recognized signature"
    echo "HINT     read the whole log: $SELF <target> --full"
    return
  fi

  echo "VERDICT  no failure signature found: the job is still running, or it"
  echo "         succeeded because the run was rerun since this link was made"
}

TAB="$(printf '\t')"

# strip_prefix removes the "job<TAB>step<TAB>timestamp " prefix the gh log
# format puts on every line, leaving the message exactly as the tool wrote it.
strip_prefix() {
  sed -E "s/^[^${TAB}]*${TAB}[^${TAB}]*${TAB}[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z //"
}

# report prints the classification and the extracted failure blocks.
report() {
  local log="$1" label="$2"

  echo
  echo "=============================================================="
  echo "JOB      $label"
  echo "LOG      $log"
  classify "$log"
  echo "=============================================================="

  if [ "$MODE" = full ]; then
    strip_prefix <"$log"
    return
  fi

  local extracted
  extracted="$(grep -E -A "$CONTEXT" -e "$HEADLINES" "$log" 2>/dev/null || true)"

  if [ -z "$extracted" ]; then
    echo "(no failure lines matched; try --full)"
    return
  fi

  # Keep the message verbatim: testify indents its detail with tabs, so the
  # line must not be split on them. Only the job column, the step column and
  # the timestamp are stripped, with sed, from the front.
  local cleaned
  cleaned="$(printf '%s\n' "$extracted" | strip_prefix)"

  if [ "$MAX_LINES" -gt 0 ]; then
    local total
    total="$(printf '%s\n' "$cleaned" | wc -l | tr -d ' ')"

    printf '%s\n' "$cleaned" | head -n "$MAX_LINES"

    if [ "$total" -gt "$MAX_LINES" ]; then
      echo "... $((total - MAX_LINES)) more lines, raise --max-lines or use --full"
    fi
  else
    printf '%s\n' "$cleaned"
  fi
}

fetch_log() {
  local job_id="$1" out="$CACHE/job-$1.log"

  # gh run view --log-failed is unusable on this repository: it attributes every
  # line to "UNKNOWN STEP" and appends the post-job cleanup, which buries the
  # actual failure. The full log plus a grep is both shorter and more reliable.
  gh run view --job "$job_id" --log >"$out" 2>"$CACHE/job-$1.err" || {
    [ -s "$out" ] || { cat "$CACHE/job-$1.err" >&2; die "failed to download the log of job $job_id"; }
  }

  echo "$out"
}

resolve_pr() {
  if [ -n "$TARGET" ]; then
    case "$TARGET" in
      ''|*[!0-9]*)
        gh pr list --head "$TARGET" --json number --jq '.[0].number' 2>/dev/null \
          || die "no pull request found for branch $TARGET"
        ;;
      *) echo "$TARGET" ;;
    esac

    return
  fi

  gh pr view --json number --jq .number 2>/dev/null \
    || die "no pull request for the current branch; pass a PR number or a branch"
}

# --- list mode ---------------------------------------------------------------
if [ "$DO_LIST" = 1 ]; then
  branch="${TARGET:-$(git rev-parse --abbrev-ref HEAD)}"

  echo "recent $WORKFLOW runs on $branch"
  gh run list --branch "$branch" --workflow "$WORKFLOW" --limit "$LIMIT" \
    --json createdAt,conclusion,displayTitle,url \
    --jq '.[] | "\(.createdAt)  \(.conclusion // "running")  \(.displayTitle[0:60])  \(.url)"'

  exit 0
fi

# --- single job mode ---------------------------------------------------------
if [ -n "$JOB_ID" ]; then
  report "$(fetch_log "$JOB_ID")" "job $JOB_ID"
  exit 0
fi

# --- pull request mode -------------------------------------------------------
PR="$(resolve_pr)"
[ -n "$PR" ] || die "could not resolve a pull request"

echo "pull request #$PR"

CHECKS="$CACHE/checks.tsv"
gh pr checks "$PR" >"$CHECKS" 2>/dev/null || true

if [ ! -s "$CHECKS" ]; then
  die "no checks reported for #$PR yet"
fi

awk -F'\t' '{ printf "  %-22s %-8s %-9s %s\n", $1, $2, $3, $4 }' "$CHECKS"

if [ "$MODE" = status ]; then
  exit 0
fi

FAILED="$(awk -F'\t' '$2 == "fail" { print $4 }' "$CHECKS")"
PENDING="$(awk -F'\t' '$2 == "pending" { print $1 }' "$CHECKS" | tr '\n' ' ')"

if [ -z "$FAILED" ]; then
  echo
  if [ -n "${PENDING// /}" ]; then
    echo "nothing has failed; still running: ${PENDING}"
  else
    echo "all checks pass."
  fi

  exit 0
fi

RUN_IDS=""

for link in $FAILED; do
  job_id="$(printf '%s\n' "$link" | sed -nE 's#.*/job/([0-9]+).*#\1#p')"
  run_id="$(printf '%s\n' "$link" | sed -nE 's#.*/runs/([0-9]+).*#\1#p')"

  if [ -z "$job_id" ]; then
    echo
    echo "skipping a failed check without a job log: $link"
    continue
  fi

  case " $RUN_IDS " in
    *" $run_id "*) ;;
    *) RUN_IDS="$RUN_IDS $run_id" ;;
  esac

  report "$(fetch_log "$job_id")" "$link"
done

if [ "$DO_RERUN" = 1 ]; then
  for run_id in $RUN_IDS; do
    echo
    echo "rerunning the failed jobs of run $run_id"
    gh run rerun "$run_id" --failed
  done
fi
