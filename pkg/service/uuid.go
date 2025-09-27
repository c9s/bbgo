package service

import "fmt"

// binUuidSelector is deprecated - use dialect.UUIDSelector instead
// Kept for backward compatibility with MySQL-only code
func binUuidSelector(tableName, uuidColName string) string {
	qualifiedColName := tableName + "." + uuidColName
	return fmt.Sprintf(
		`IF(%s != '', BIN_TO_UUID(%s, true), '') AS %s`,
		qualifiedColName,
		qualifiedColName,
		uuidColName,
	)
}

// dialectUuidSelector returns a database-agnostic UUID selector using the dialect system
func dialectUuidSelector(dialect DatabaseDialect, tableName, uuidColName string) string {
	return dialect.UUIDSelector(tableName, uuidColName)
}
