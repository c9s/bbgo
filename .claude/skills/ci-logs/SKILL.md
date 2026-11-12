---
name: ci-logs
description: Find out why a GitHub Actions run failed for a pull request or branch, and tell a code failure apart from a flaky test or a GitHub outage. Use when a PR check is red, when asked why CI failed, or before assuming a failure was caused by the change under review.
user-invocable: true
---

# CI Log Inspection

Explain a red `Go / build` or `lint` check: what failed, whether it is the code,
and what to do next.

Do not hand-roll `gh` pipelines for this. `agent/scripts/ci-logs.sh` already
encodes the gotchas, which are documented in
`agent/design/0002-ci-log-inspection.md`; the flags are in
`agent/scripts/README.md`.

## Steps

### 1. Run the script

```bash
./agent/scripts/ci-logs.sh                 # the current branch's PR
./agent/scripts/ci-logs.sh <pr-number>     # a specific PR
./agent/scripts/ci-logs.sh --job <job-id>  # one job
```

It prints the check table, then for each failed check a `VERDICT`, a `HINT` and
the extracted failure lines.

### 2. Act on the verdict

| verdict | action |
|---|---|
| `GitHub Actions infrastructure failure` | not the code. Rerun: `./agent/scripts/ci-logs.sh <pr> --rerun`. If a fix is being pushed to the same branch anyway, the push triggers a fresh run and the rerun is unnecessary. |
| `dependency blocked by Aikido Device Protection` | not the code. Report it, do not work around it; load the `aikido-device-protection` skill. |
| `dependency resolution failure` | run `go mod tidy` locally, or `go get` the module named in the log. |
| `test failure` | go to step 3. |
| `compile failure` | reproduce with `go build ./... && go vet ./...`. |
| `no failure signature found` | the job is still running, or the run was rerun since the link was made and now passes. Re-check with `--status`. |

### 3. For a test failure, reproduce it before changing anything

Run the command the script prints. **Which test step failed matters**: CI runs
`Test` with the default build tags and `-count 3`, and `TestDnum` with
`-tags dnum`. A test that passes under one and fails under the other is a real
class of bug here, usually an assertion that depends on `fixedpoint`'s
precision.

```bash
go test -count 3 -race ./pkg/<package>      # as the Test step runs it
go test -tags dnum -count=1 ./pkg/<package> # as the TestDnum step runs it
```

If it does not reproduce, check whether the failure is pre-existing rather than
caused by the change under review:

```bash
./agent/scripts/ci-logs.sh main --list   # is main green?
git log --oneline -1 -- <the failing test file>
```

A test that is green on `main`, passes locally under both tag sets, and failed
on a timing assertion is a flake. Say so plainly, name the race, and do not
"fix" it inside an unrelated pull request without asking.

### 4. Report

State for each failing check: the verdict, whether it is attributable to the
change under review, and the evidence. Quote the few relevant log lines, not the
whole block. If something was rerun or pushed, say which.

## Notes

- The script is read-only except for `--rerun`.
- `--full` prints the whole log when the extraction missed the cause; `--context
  <n>` widens each block.
- Requires `gh` authenticated for the repository.
