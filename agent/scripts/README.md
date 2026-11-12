# `agent/scripts/`

Helper scripts for agents working on this repository. They are plain shell, safe
to run by hand, and read-only unless a flag says otherwise.

| script | what it does | mutates anything? |
|---|---|---|
| [`ci-logs.sh`](ci-logs.sh) | explains why a GitHub Actions run failed | only with `--rerun` |

Requirements: [`gh`](https://cli.github.com) authenticated against this
repository (`gh auth status`). `ci-logs.sh` also uses `git` when no target is
given.

---

## `ci-logs.sh`

Turns a red check into a verdict and the few log lines that matter, instead of
the ~2000 lines a raw job log contains.

```
ci-logs.sh [options] [<pr-number> | <branch>]
```

With no target it uses the pull request of the current branch.

### Options

| option | meaning |
|---|---|
| `--job <id>` | inspect one job id directly, skipping the check lookup |
| `--status` | print only the check status table |
| `--full` | print the whole job log, prefix-stripped, instead of the extracted failures |
| `--context <n>` | log lines printed after each failure headline (default 6) |
| `--max-lines <n>` | cap the extracted output (default 200; `0` means no cap) |
| `--list` | list the recent workflow runs of the branch |
| `--limit <n>` | how many runs `--list` shows (default 10) |
| `--workflow <f>` | workflow file for `--list` (default `go.yml`) |
| `--rerun` | rerun the failed jobs of the run — **the only mutating flag** |
| `-h`, `--help` | usage |

### Typical use

```bash
# why is my PR red?
./agent/scripts/ci-logs.sh 2772

# just the status of the current branch's PR
./agent/scripts/ci-logs.sh --status

# one job, with more context around each failure
./agent/scripts/ci-logs.sh --job 129054160489 --context 12

# is main green, or is this failure pre-existing?
./agent/scripts/ci-logs.sh main --list

# the failure was a GitHub 504, not the code
./agent/scripts/ci-logs.sh 2767 --rerun
```

### Output

For a passing pull request it prints the check table and stops:

```
pull request #2772
  build (6.2, 1.27)      pass     11m59s    https://github.com/c9s/bbgo/actions/...
  lint                   pass     1m4s      https://github.com/c9s/bbgo/actions/...

all checks pass.
```

For a failure it prints a verdict, the next action, and the failure itself:

```
==============================================================
JOB      https://github.com/c9s/bbgo/actions/runs/.../job/129054160489
LOG      /tmp/ci-logs.XXXX/job-129054160489.log
VERDICT  test failure
STEP     TestDnum
HINT     reproduce with: go test -tags dnum -count=1 ./pkg/...
TESTS    TestToFixedPoint
PACKAGES github.com/c9s/bbgo/pkg/chain/evm/token
==============================================================
--- FAIL: TestToFixedPoint (0.00s)
    amount_test.go:103:
        	Error Trace:	/home/runner/work/bbgo/bbgo/pkg/chain/evm/token/amount_test.go:103
        	Error:      	Should be true
        	Test:       	TestToFixedPoint
```

The `LOG` line is the downloaded log, kept for the lifetime of the process; pass
`--full` to read all of it.

### Verdicts

| verdict | meaning | next action |
|---|---|---|
| `dependency blocked by Aikido Device Protection` | a package was blocked by policy, not a code failure | `aikidoctl events --json`; a `min_package_age` block clears on its own. Do not work around it. |
| `GitHub Actions infrastructure failure` | a 5xx or a failed tool download | `--rerun` |
| `dependency resolution failure` | `go.sum` or module resolution | `go mod tidy`, or `go get` the named module |
| `test failure` | one or more tests failed | reproduce with the printed command |
| `compile failure` | the package does not build | `go build ./... && go vet ./...` |
| `no failure signature found` | the job is still running, or the run was rerun since the link was made and now passes | re-check the status |

### Notes

- **Which test step failed matters.** CI runs `Test` (default build tags,
  `-count 3`) and `TestDnum` (`-tags dnum`). A test can pass in one and fail in
  the other, which is a real class of bug in this repository. When the log is
  old enough that the step name is lost, the script prints both reproduce
  commands rather than guessing.
- **`-count 3` surfaces flakes.** Before assuming a failure is yours, check
  whether `main` is green with `ci-logs.sh main --list`.
- **A rerun invalidates the old job link.** `gh run rerun --failed` reuses the
  run id, so inspecting a pre-rerun job link afterwards shows the passing
  attempt.
- **Pushing supersedes a rerun.** A push triggers a fresh run; there is no need
  to also rerun the old one.
