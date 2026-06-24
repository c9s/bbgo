---
name: compile-migrations
description: Compile SQL migration files into Go source code for embedding in binaries
disable-model-invocation: true
argument-hint: [output_dir]
---

Compile SQL migrations into Go code for all configured dialects.

## Steps

1. Find all `rockhopper_*.yaml` config files in the project root.
2. If `$ARGUMENTS` is provided, use it as the base output directory. Otherwise, use `pkg/migrations/<dialect>` as the default output path for each dialect.
3. For each config file, run:
   ```bash
   rockhopper compile --config <config_file> --output <output_dir>
   ```
4. Show the generated Go files and confirm compilation succeeded.
