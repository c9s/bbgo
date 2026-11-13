---
name: migration-status
description: Show the current migration status for all configured databases
disable-model-invocation: true
argument-hint: [config_file]
---

Show migration status using rockhopper.

## Steps

1. If `$ARGUMENTS` is provided, use it as the config file path. Otherwise, find all `rockhopper_*.yaml` config files.
2. For each config file, run:
   ```bash
   rockhopper --config <config_file> status
   ```
3. Present the results clearly, grouped by dialect.
