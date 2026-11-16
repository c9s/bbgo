# 0001 — Agent workspace layout

## Context

Agent-facing material was accumulating in three places with no rule for which
to use: `CLAUDE.md` (loaded into every session), `.claude/skills/` (invoked on
demand), and ad-hoc shell pipelines retyped from scratch each time a workflow
came up again.

The third one is the real cost. A workflow like "find out why CI failed" took a
dozen `gh` invocations to rediscover, including the same wrong turns every
time, and the knowledge gained evaporated when the session ended.

## Decision

A top-level `agent/` directory with two subdirectories:

- **`agent/design/`** — numbered design records. The *why*, the findings, and
  the rejected alternatives. These are too long for `CLAUDE.md`, which has to
  stay short because it is loaded into every session.
- **`agent/scripts/`** — the executables. A workflow that is more than a couple
  of commands becomes a script here, so it is reproducible and so its gotchas
  are encoded rather than remembered.

`.claude/skills/<name>/SKILL.md` stays the entry point an agent or a user
invokes; it should be thin and delegate to a script in `agent/scripts/`.

Top level rather than under `.claude/` because this is project material, not
tool configuration: it is useful to a human reading the repository and to any
agent, not only to Claude Code.

## Rejected alternatives

- **Everything in `CLAUDE.md`.** It is loaded into every session, so every line
  costs context on every unrelated task. Long procedures do not belong there.
- **Everything in `.claude/skills/`.** A skill is a procedure, not a place for
  design reasoning, and skill files are not a natural home for executables that
  humans also want to run.
- **`scripts/` at the top level.** The repository already has `scripts/` and
  `utils/` for build and release tooling; mixing agent helpers into them would
  blur what is part of the build and what is not.

## Consequences

- Nothing in `pkg/` may depend on `agent/`; it is documentation and tooling only.
- A new workflow worth keeping gets three things: a script, a design record, and
  a one-line pointer from `CLAUDE.md` or a skill.
