package service

import (
	"fmt"

	sq "github.com/Masterminds/squirrel"
)

// DatabaseDialect provides database-specific SQL syntax and functions
type DatabaseDialect interface {
	// Date extraction functions
	ExtractYear(column string) string
	ExtractMonth(column string) string
	ExtractDay(column string) string

	// Null coalescing function
	Coalesce(expr, defaultVal string) string

	// Upsert syntax
	UpsertSQL(tableName, insertClause, valuesClause, updateClause string) string

	// Specific upsert methods for common tables
	OrderUpsertSQL(insertClause, valuesClause, updateClause string) string
	TradeUpsertSQL(insertClause, valuesClause, updateClause string) string
	KLineInsertSQL(tableName, insertClause, valuesClause string) string

	// UUID handling
	UUIDBinaryConversion(uuidExpr string) string
	UUIDSelector(tableName, columnName string) string

	// Conditional expression
	IfExpr(condition, trueExpr, falseExpr string) string

	// Name escaping
	EscapeColumnName(name string) string
	EscapeTableName(name string) string

	// Query builder configuration
	ConfigurePlaceholder(builder sq.SelectBuilder) sq.SelectBuilder
}

// GetDialect returns the appropriate dialect for the given driver name
func GetDialect(driverName string) DatabaseDialect {
	switch driverName {
	case "mysql":
		return &MySQLDialect{}
	case "postgres":
		return &PostgreSQLDialect{}
	case "sqlite3":
		return &SQLiteDialect{}
	default:
		return &SQLiteDialect{} // default fallback
	}
}

// MySQLDialect implements MySQL-specific SQL syntax
type MySQLDialect struct{}

func (d *MySQLDialect) ExtractYear(column string) string {
	return fmt.Sprintf("YEAR(%s)", column)
}

func (d *MySQLDialect) ExtractMonth(column string) string {
	return fmt.Sprintf("MONTH(%s)", column)
}

func (d *MySQLDialect) ExtractDay(column string) string {
	return fmt.Sprintf("DAY(%s)", column)
}

func (d *MySQLDialect) Coalesce(expr, defaultVal string) string {
	return fmt.Sprintf("IFNULL(%s, %s)", expr, defaultVal)
}

func (d *MySQLDialect) UpsertSQL(tableName, insertClause, valuesClause, updateClause string) string {
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		tableName, insertClause, valuesClause, updateClause)
}

func (d *MySQLDialect) UUIDBinaryConversion(uuidExpr string) string {
	return fmt.Sprintf("UUID_TO_BIN(%s, true)", uuidExpr)
}

func (d *MySQLDialect) UUIDSelector(tableName, columnName string) string {
	qualifiedColName := tableName + "." + columnName
	return fmt.Sprintf("IF(%s != '', BIN_TO_UUID(%s, true), '') AS %s",
		qualifiedColName, qualifiedColName, columnName)
}

func (d *MySQLDialect) IfExpr(condition, trueExpr, falseExpr string) string {
	return fmt.Sprintf("IF(%s, %s, %s)", condition, trueExpr, falseExpr)
}

func (d *MySQLDialect) EscapeColumnName(name string) string {
	return "`" + name + "`"
}

func (d *MySQLDialect) EscapeTableName(name string) string {
	return "`" + name + "`"
}

func (d *MySQLDialect) ConfigurePlaceholder(builder sq.SelectBuilder) sq.SelectBuilder {
	// MySQL uses default placeholder format (?)
	return builder
}

func (d *MySQLDialect) OrderUpsertSQL(insertClause, valuesClause, updateClause string) string {
	return fmt.Sprintf("INSERT INTO orders (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		insertClause, valuesClause, updateClause)
}

func (d *MySQLDialect) TradeUpsertSQL(insertClause, valuesClause, updateClause string) string {
	return fmt.Sprintf("INSERT INTO trades (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		insertClause, valuesClause, updateClause)
}

func (d *MySQLDialect) KLineInsertSQL(tableName, insertClause, valuesClause string) string {
	// MySQL uses INSERT IGNORE to skip duplicate entries
	return fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tableName, insertClause, valuesClause)
}

// PostgreSQLDialect implements PostgreSQL-specific SQL syntax
type PostgreSQLDialect struct{}

func (d *PostgreSQLDialect) ExtractYear(column string) string {
	return fmt.Sprintf("EXTRACT(YEAR FROM %s)", column)
}

func (d *PostgreSQLDialect) ExtractMonth(column string) string {
	return fmt.Sprintf("EXTRACT(MONTH FROM %s)", column)
}

func (d *PostgreSQLDialect) ExtractDay(column string) string {
	return fmt.Sprintf("EXTRACT(DAY FROM %s)", column)
}

func (d *PostgreSQLDialect) Coalesce(expr, defaultVal string) string {
	return fmt.Sprintf("COALESCE(%s, %s)", expr, defaultVal)
}

func (d *PostgreSQLDialect) UpsertSQL(tableName, insertClause, valuesClause, updateClause string) string {
	// WARNING: Generic upsert for PostgreSQL requires knowing conflict columns
	// This method should only be used by legacy code - prefer table-specific upsert methods
	// Since we don't know the conflict columns, we fall back to INSERT with DO NOTHING
	// which prevents errors but won't update existing records
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
		tableName, insertClause, valuesClause)
}

func (d *PostgreSQLDialect) UUIDBinaryConversion(uuidExpr string) string {
	// PostgreSQL stores UUIDs natively, no conversion needed
	return uuidExpr
}

func (d *PostgreSQLDialect) UUIDSelector(tableName, columnName string) string {
	// PostgreSQL stores UUIDs as text, so no conversion needed
	return tableName + "." + columnName + " AS " + columnName
}

func (d *PostgreSQLDialect) IfExpr(condition, trueExpr, falseExpr string) string {
	return fmt.Sprintf("CASE WHEN %s THEN %s ELSE %s END", condition, trueExpr, falseExpr)
}

func (d *PostgreSQLDialect) EscapeColumnName(name string) string {
	return `"` + name + `"`
}

func (d *PostgreSQLDialect) EscapeTableName(name string) string {
	return `"` + name + `"`
}

func (d *PostgreSQLDialect) ConfigurePlaceholder(builder sq.SelectBuilder) sq.SelectBuilder {
	// PostgreSQL uses dollar placeholder format ($1, $2, etc.)
	return builder.PlaceholderFormat(sq.Dollar)
}

func (d *PostgreSQLDialect) OrderUpsertSQL(insertClause, valuesClause, updateClause string) string {
	return fmt.Sprintf("INSERT INTO orders (%s) VALUES (%s) ON CONFLICT (order_id, exchange) DO UPDATE SET %s",
		insertClause, valuesClause, updateClause)
}

func (d *PostgreSQLDialect) TradeUpsertSQL(insertClause, valuesClause, updateClause string) string {
	return fmt.Sprintf("INSERT INTO trades (%s) VALUES (%s) ON CONFLICT (exchange, symbol, side, id) DO UPDATE SET %s",
		insertClause, valuesClause, updateClause)
}

func (d *PostgreSQLDialect) KLineInsertSQL(tableName, insertClause, valuesClause string) string {
	// PostgreSQL uses ON CONFLICT DO NOTHING to skip duplicate entries
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING", tableName, insertClause, valuesClause)
}

// SQLiteDialect implements SQLite-specific SQL syntax
type SQLiteDialect struct{}

func (d *SQLiteDialect) ExtractYear(column string) string {
	return fmt.Sprintf("strftime('%%Y', %s)", column)
}

func (d *SQLiteDialect) ExtractMonth(column string) string {
	return fmt.Sprintf("strftime('%%m', %s)", column)
}

func (d *SQLiteDialect) ExtractDay(column string) string {
	return fmt.Sprintf("strftime('%%d', %s)", column)
}

func (d *SQLiteDialect) Coalesce(expr, defaultVal string) string {
	return fmt.Sprintf("IFNULL(%s, %s)", expr, defaultVal)
}

func (d *SQLiteDialect) UpsertSQL(tableName, insertClause, valuesClause, updateClause string) string {
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO UPDATE SET %s",
		tableName, insertClause, valuesClause, updateClause)
}

func (d *SQLiteDialect) UUIDBinaryConversion(uuidExpr string) string {
	// SQLite doesn't have native UUID support, store as text
	return uuidExpr
}

func (d *SQLiteDialect) UUIDSelector(tableName, columnName string) string {
	// SQLite stores UUIDs as text, so no conversion needed
	return tableName + "." + columnName + " AS " + columnName
}

func (d *SQLiteDialect) IfExpr(condition, trueExpr, falseExpr string) string {
	return fmt.Sprintf("CASE WHEN %s THEN %s ELSE %s END", condition, trueExpr, falseExpr)
}

func (d *SQLiteDialect) EscapeColumnName(name string) string {
	return "`" + name + "`"
}

func (d *SQLiteDialect) EscapeTableName(name string) string {
	return "`" + name + "`"
}

func (d *SQLiteDialect) ConfigurePlaceholder(builder sq.SelectBuilder) sq.SelectBuilder {
	// SQLite uses default placeholder format (?)
	return builder
}

func (d *SQLiteDialect) OrderUpsertSQL(insertClause, valuesClause, updateClause string) string {
	return fmt.Sprintf("INSERT INTO orders (%s) VALUES (%s) ON CONFLICT (order_id, exchange) DO UPDATE SET %s",
		insertClause, valuesClause, updateClause)
}

func (d *SQLiteDialect) TradeUpsertSQL(insertClause, valuesClause, updateClause string) string {
	return fmt.Sprintf("INSERT INTO trades (%s) VALUES (%s) ON CONFLICT (exchange, symbol, side, id) DO UPDATE SET %s",
		insertClause, valuesClause, updateClause)
}

func (d *SQLiteDialect) KLineInsertSQL(tableName, insertClause, valuesClause string) string {
	// SQLite uses INSERT OR IGNORE to skip duplicate entries
	return fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tableName, insertClause, valuesClause)
}

// GenerateTimeRangeClauses generates database-specific time range clauses
func GenerateTimeRangeClauses(dialect DatabaseDialect, timeRangeColumn, period string) (selectors []string, groupBys []string, orderBys []string) {
	switch period {
	case "month":
		selectors = append(selectors,
			dialect.ExtractYear(timeRangeColumn)+" AS year",
			dialect.ExtractMonth(timeRangeColumn)+" AS month")
		groupBys = append([]string{
			dialect.ExtractMonth(timeRangeColumn),
			dialect.ExtractYear(timeRangeColumn)}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC")

	case "year":
		selectors = append(selectors, dialect.ExtractYear(timeRangeColumn)+" AS year")
		groupBys = append([]string{dialect.ExtractYear(timeRangeColumn)}, groupBys...)
		orderBys = append(orderBys, "year ASC")

	case "day":
		fallthrough

	default:
		selectors = append(selectors,
			dialect.ExtractYear(timeRangeColumn)+" AS year",
			dialect.ExtractMonth(timeRangeColumn)+" AS month",
			dialect.ExtractDay(timeRangeColumn)+" AS day")
		groupBys = append([]string{
			dialect.ExtractDay(timeRangeColumn),
			dialect.ExtractMonth(timeRangeColumn),
			dialect.ExtractYear(timeRangeColumn)}, groupBys...)
		orderBys = append(orderBys, "year ASC", "month ASC", "day ASC")
	}

	return
}
