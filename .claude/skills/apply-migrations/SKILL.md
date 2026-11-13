---
name: apply-migrations
description: Apply pending database migrations (rockhopper up)
disable-model-invocation: true
argument-hint: [config_file]
---

Apply pending migrations using rockhopper.

## Steps

1. If `$ARGUMENTS` is provided, use it as the config file path. Otherwise, find all `rockhopper_*.yaml` config files and ask the user which one to use.
2. Run:
   ```bash
   rockhopper --config <config_file> up
   ```
3. Report the result.

## Handling out-of-order migrations

If `up` fails with an "out-of-order migrations detected" error, it means a pending migration has a lower version than one that is already applied (common after merging parallel branches). Do **not** blindly re-run with `--allow-out-of-order`. Instead:

1. Show the user the offending migration(s) from the error.
2. Explain the two options:
   - **Renumber** the new migration above the latest applied version (safe default — keeps history linear).
   - **Apply in place** with `rockhopper --config <config_file> up --allow-out-of-order`, only if the older migration is independent of the newer ones.
3. Ask the user which they prefer before proceeding.

## Environment variable overrides

If the user needs to override the DSN or driver, remind them they can set:
- `ROCKHOPPER_DRIVER` - database driver name
- `ROCKHOPPER_DIALECT` - SQL dialect name
- `ROCKHOPPER_DSN` - data source name connection string
