# 0002 — CI log inspection

## Context

When a pull request's `Go / build` check goes red, the question is always the
same: *is this my code, a flaky test, or GitHub having a bad day?* Answering it
by hand took a dozen `gh` calls and the same wrong turns each time.

This record captures what actually works on this repository, and
`agent/scripts/ci-logs.sh` implements it.

## Decision

Use `gh pr checks` to get the failing job links, then `gh run view --job <id>
--log` and grep for a fixed set of failure headlines. Classify the result into
one of five verdicts, each with a different next action:

| verdict | signature | what to do |
|---|---|---|
| Aikido block | `Install blocked by Aikido` | not a code failure; `aikidoctl events --json`, a `min_package_age` block clears on its own |
| infrastructure | `Unexpected HTTP response: 5xx`, `Failed to download` | not a code failure; rerun the failed jobs |
| dependency | `missing go.sum entry`, `no required module provides` | `go mod tidy`, or `go get` the named module |
| test | `--- FAIL:` | reproduce locally; note which test **step** failed |
| compile | `[build failed]`, `undefined:` | `go build ./... && go vet ./...` |

## Findings

These are the non-obvious parts, all of them learned the hard way.

**`gh run view --log-failed` is unusable here.** It attributes every line to
`UNKNOWN STEP` and then appends the whole post-job cleanup: the git credential
teardown, the Docker container removal and the MySQL service container's startup
log. The actual failure is buried hundreds of lines up. The full log plus a grep
is both shorter and more reliable.

**BSD grep rejects a pattern that starts with a dash.** `grep -nE "--- FAIL|x"`
fails with `grep: unrecognized option` on macOS, because the pattern looks like
a flag. Pass it as `grep -e "--- FAIL"`, or make the alternation start with
something else.

**Do not split the log lines on tabs.** The `gh` log format is
`job<TAB>step<TAB>timestamp message`, which invites `awk -F'\t' '{print $3}'`.
That silently truncates every testify failure, because testify indents its
`Error Trace:` / `Error:` detail with tabs of its own — exactly the lines worth
reading. Strip the three-field prefix with `sed` and leave the message alone.

**The coverage line says nothing about the build tags.** This repository runs
two test steps, `Test` (`go test -count 3 -race -coverprofile ...`) and
`TestDnum` (`go test -race -coverprofile ... -tags dnum ./pkg/...`). *Both*
write a coverage profile, so "coverage: 82.2%" in the output does not identify
which one failed. The step name is the only reliable signal.

**Step attribution decays.** Fresh logs carry real step names, so a failure can
be pinned to `Test` or `TestDnum` directly. Older logs come back with
`UNKNOWN STEP` for every line. When the step is unknown, both test sets have to
be tried locally — which is why the script prints both commands instead of
guessing.

**`-count 3` makes flakes visible.** The `Test` step runs each test three times.
A timing-dependent test that passes locally can fail in CI for that reason
alone, and it is not evidence that the change under review broke anything. Check
whether `main` is green before assuming otherwise:
`agent/scripts/ci-logs.sh main --list`.

**A rerun rewrites what the old job link shows.** `gh run rerun --failed` reuses
the run id, so `gh run view --job <old id> --log` afterwards returns the *new*,
passing attempt. A link from before a rerun therefore reports "no failure
signature found", which means the rerun succeeded, not that the tool broke.

**Pushing a commit supersedes a rerun.** A push to the branch triggers a fresh
run, so there is no point rerunning the old one as well.

## Rejected alternatives

- **`gh run view --log-failed`.** See above; it hides the failure it is named for.
- **Reading the web UI.** Fine for a human looking at one job, useless for
  classifying four pull requests, and it cannot be grepped.
- **`gh api` on the check-runs endpoint.** Returns the status but not the log
  text, so a second request is needed anyway.
- **Parsing the step grouping to locate the failure.** Tempting, but the
  grouping is exactly what is missing from older logs.

## Consequences

- One command answers the question: `agent/scripts/ci-logs.sh <pr>`.
- The verdict names the next action, so an infrastructure flake is not debugged
  as a code failure, which is the expensive mistake.
- The script has no repository-specific knowledge beyond the two test step names
  used for the reproduce hint, so it works on any Go repository with a
  `gh`-visible Actions workflow.
