package mysql

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/c9s/rockhopper/v2"
)

var registeredGoMigrations = map[rockhopper.RegistryKey]*rockhopper.Migration{}

func MergeMigrationsMap(ms map[rockhopper.RegistryKey]*rockhopper.Migration) {
	for k, m := range ms {
		if _, ok := registeredGoMigrations[k]; !ok {
			registeredGoMigrations[k] = m
		} else {
			log.Printf("the migration key %+v is duplicated: %+v", k, m)
		}
	}
}

func GetMigrationsMap() map[rockhopper.RegistryKey]*rockhopper.Migration {
	return registeredGoMigrations
}

// SortedMigrations builds up the migration objects, sort them by timestamp and return as a slice
func SortedMigrations() rockhopper.MigrationSlice {
	return Migrations()
}

// Migrations builds up the migration objects, sort them by timestamp and return as a slice
func Migrations() rockhopper.MigrationSlice {
	var migrations = rockhopper.MigrationSlice{}
	for _, migration := range registeredGoMigrations {
		migrations = append(migrations, migration)
	}

	return migrations.SortAndConnect()
}

// AddMigration adds a migration with its runtime caller information
func AddMigration(packageName string, up, down rockhopper.TransactionHandler) {
	pc, filename, _, _ := runtime.Caller(1)

	if packageName == "" {
		funcName := runtime.FuncForPC(pc).Name()
		packageName = _parseFuncPackageName(funcName)
	}

	AddNamedMigration(packageName, filename, up, down)
}

// parseFuncPackageName parses the package name from a given runtime caller function name
func _parseFuncPackageName(funcName string) string {
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}

	lastDot := strings.LastIndexByte(funcName[lastSlash:], '.') + lastSlash
	packageName := funcName[:lastDot]
	return packageName
}

// AddNamedMigration adds a named migration to the registered go migration map
func AddNamedMigration(packageName, filename string, up, down rockhopper.TransactionHandler) {
	v, err := rockhopper.FileNumericComponent(filename)
	if err != nil {
		panic(fmt.Errorf("unable to parse numeric component from filename %s: %v", filename, err))
	}

	migration := &rockhopper.Migration{
		Package:    packageName,
		Registered: true,

		Version: v,
		UpFn:    up,
		DownFn:  down,
		Source:  filename,
		UseTx:   true,
	}

	key := rockhopper.RegistryKey{Package: packageName, Version: v}
	if existing, ok := registeredGoMigrations[key]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with key %+v: %+v", filename, key, existing))
	}

	registeredGoMigrations[key] = migration
}

// AddStatementMigration registers a migration that was compiled from a .sql file.
// The SQL statements are kept as data (rather than baked into a function body) so
// the console can preview each statement while the migration runs.
func AddStatementMigration(packageName string, version int64, source string, useTx bool, upStatements, downStatements []rockhopper.Statement) {
	migration := &rockhopper.Migration{
		Package:    packageName,
		Registered: true,

		Version: version,
		Source:  source,
		UseTx:   useTx,

		UpStatements:   upStatements,
		DownStatements: downStatements,
	}

	key := rockhopper.RegistryKey{Package: packageName, Version: version}
	if existing, ok := registeredGoMigrations[key]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with key %+v: %+v", source, key, existing))
	}

	registeredGoMigrations[key] = migration
}
