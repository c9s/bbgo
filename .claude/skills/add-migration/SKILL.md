---
name: add-migration
description: Add new database migration for the bbgo trading bot framework
user-invocable: true
---

# Add Migration Skill

Create a new database migration for the bbgo project covering both MySQL and SQLite3.

## Input

You will be given a `[migration_name]` from the user. If not provided, ask the user for it.
Before writing the migration SQL, ask the user what the migration should do (e.g., "Add a new table for storing user preferences" or "Alter the orders table to add a new column for order source").

## Steps

### 1. Create migration files with rockhopper

Run both commands from the repository root (`/Users/dboy/Work/bbgo`):

```bash
rockhopper --config ./rockhopper_sqlite.yaml create --type sql [migration_name]
rockhopper --config ./rockhopper_mysql.yaml create --type sql [migration_name]
```

This creates timestamped `.sql` files under `migrations/mysql/` and `migrations/sqlite3/`.

### 2. Write the migration SQL

Each migration file uses this format:

```sql
-- +up
-- +begin
<UP migration SQL here>
-- +end

-- +down
-- +begin
<DOWN migration SQL here>
-- +end
```

**Write the MySQL migration first**, then translate it to SQLite3-compatible SQL.

Key differences when translating MySQL → SQLite3:

- if the `gid` column is presented: `BIGINT UNSIGNED NOT NULL AUTO_INCREMENT` → `INTEGER PRIMARY KEY AUTOINCREMENT`
- `VARCHAR(N)` → `TEXT`
- `DECIMAL(M, N)` → `REAL`
- SQLite does not support `KEY` or named index definitions inline in CREATE TABLE; use separate `CREATE INDEX` statements if needed
- SQLite does not support `UNIQUE KEY` inline; use `CREATE UNIQUE INDEX` instead or a UNIQUE constraint
- `DATETIME(3)` works in both but SQLite ignores the precision
- SQLite does not support `ALTER TABLE ... ADD KEY/INDEX`; use `CREATE INDEX` separately
- SQLite has limited `ALTER TABLE` support (no DROP COLUMN before 3.35, no MODIFY COLUMN)

Ask the user what the migration should do if not obvious from the name, then write the SQL for both files.

### 3. Compile migrations to Go code

```bash
rockhopper --config ./rockhopper_sqlite.yaml compile --output ./pkg/migrations/sqlite3
rockhopper --config ./rockhopper_mysql.yaml compile --output ./pkg/migrations/mysql
```

This generates Go files in `pkg/migrations/mysql/` and `pkg/migrations/sqlite3/`.

### 4. Verify the migration compiles

Run a build to ensure the generated Go code compiles:

```bash
go build ./pkg/migrations/...
```

### 5. (Optional) Test with rockhopper up

If the user has a local database configured, test the migration:

```bash
rockhopper --config ./rockhopper_mysql.yaml up
rockhopper --config ./rockhopper_sqlite.yaml up
```

Only do this if the user explicitly requests it or has a local database available, since it requires a running database connection.

## Notes

- Migration filenames are auto-generated with timestamps by `rockhopper create`
- Always provide both `-- +up` and `-- +down` sections
- The down migration should reverse the up migration (e.g., `DROP TABLE IF EXISTS` for a `CREATE TABLE`)
- For `ALTER TABLE` migrations, the down should reverse the alteration
- After compilation, commit both the `.sql` files in `migrations/` and the generated `.go` files in `pkg/migrations/`
