---
name: rollback-migration
description: Rollback the last applied database migration (rockhopper down)
disable-model-invocation: true
argument-hint: [config_file]
---

Rollback the last applied migration using rockhopper.

## Steps

1. If `$ARGUMENTS` is provided, use it as the config file path. Otherwise, find all `rockhopper_*.yaml` config files and ask the user which one to use.
2. First show current status so the user sees what will be rolled back:
   ```bash
   rockhopper --config <config_file> status
   ```
3. Confirm with the user before proceeding.
4. Run:
   ```bash
   rockhopper --config <config_file> down
   ```
5. Report the result.
