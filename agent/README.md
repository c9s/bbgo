# `agent/`

Workspace for coding agents working on this repository: the design records they
should read before changing something, and the helper scripts they are expected
to use instead of re-deriving a workflow from scratch.

```
agent/
  design/    design records: the why behind agent-facing tooling and conventions
  scripts/   helper scripts meant to be run by an agent (and by humans)
```

- **`agent/design/`** — one numbered record per decision or workflow. Read
  `agent/design/README.md` for the index and the convention.
- **`agent/scripts/`** — executable helpers. Every script is documented in
  `agent/scripts/README.md` with its flags and its output.

This directory is not imported by any Go package and nothing in `pkg/` depends
on it. It is documentation and tooling only.

## Relationship to `CLAUDE.md` and `.claude/skills/`

| where | what belongs there |
|---|---|
| `CLAUDE.md` | facts an agent needs in context on every task: build commands, architecture, traps |
| `.claude/skills/<name>/SKILL.md` | a procedure to invoke on demand, usually a thin wrapper that calls a script here |
| `agent/design/` | the reasoning and the findings behind a procedure, too long for either of the above |
| `agent/scripts/` | the executable itself, so the procedure is one command rather than a dozen |

A skill that needs more than a handful of steps should call a script in
`agent/scripts/` and keep the long explanation in `agent/design/`.
