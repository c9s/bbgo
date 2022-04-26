package sqlite3

import (
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/c9s/rockhopper"
)

var registeredGoMigrations map[int64]*rockhopper.Migration

func MergeMigrationsMap(ms map[int64]*rockhopper.Migration) {
	for k, m := range ms {
		if _, ok := registeredGoMigrations[k]; !ok {
			registeredGoMigrations[k] = m
		} else {
			log.Printf("the migration key %d is duplicated: %+v", k, m)
		}
	}
}

func GetMigrationsMap() map[int64]*rockhopper.Migration {
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
func AddMigration(up, down rockhopper.TransactionHandler) {
	pc, filename, _, _ := runtime.Caller(1)

	funcName := runtime.FuncForPC(pc).Name()
	packageName := _parseFuncPackageName(funcName)
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
	if registeredGoMigrations == nil {
		registeredGoMigrations = make(map[int64]*rockhopper.Migration)
	}

	v, _ := rockhopper.FileNumericComponent(filename)

	migration := &rockhopper.Migration{
		Package:    packageName,
		Registered: true,

		Version: v,
		UpFn:    up,
		DownFn:  down,
		Source:  filename,
		UseTx:   true,
	}

	if existing, ok := registeredGoMigrations[v]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with %q", filename, existing.Source))
	}
	registeredGoMigrations[v] = migration
}
