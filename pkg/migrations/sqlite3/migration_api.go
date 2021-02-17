package sqlite3

import (
	"github.com/c9s/rockhopper"

	"fmt"
	"runtime"
	"strings"
)

var registeredGoMigrations map[int64]*rockhopper.Migration

// AddMigration adds a migration.
func AddMigration(up, down rockhopper.TransactionHandler) {
	pc, filename, _, _ := runtime.Caller(1)

	funcName := runtime.FuncForPC(pc).Name()
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}
	lastDot := strings.LastIndexByte(funcName[lastSlash:], '.') + lastSlash
	packageName := funcName[:lastDot]
	AddNamedMigration(packageName, filename, up, down)
}

// AddNamedMigration : Add a named migration.
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
