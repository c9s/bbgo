# Design Records

One numbered record per agent-facing decision or workflow. A record explains
*why* something is the way it is, and what was tried and rejected, so the next
agent does not rediscover the same dead ends.

| # | record | summary |
|---|---|---|
| 0001 | [agent workspace layout](0001-agent-workspace-layout.md) | what `agent/` is for and what goes where |
| 0002 | [CI log inspection](0002-ci-log-inspection.md) | how to find out why a GitHub Actions run failed, and why `--log-failed` is not the answer |

## Convention

- File name: `NNNN-kebab-case-title.md`, numbered in creation order.
- Structure: **Context** (what problem), **Decision** (what we do),
  **Findings** (the non-obvious things learned, which is usually the most
  valuable part), **Rejected alternatives**, **Consequences**.
- Record the gotchas with enough detail to act on. "BSD grep rejects a pattern
  starting with a dash" is worth a line; "be careful with grep" is not.
- A record is not a changelog. Update it in place when the decision changes and
  say what changed.
