package service

import (
	"context"
	"reflect"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/fatih/camelcase"
	gopluralize "github.com/gertd/go-pluralize"
	"github.com/jmoiron/sqlx"
	"github.com/sirupsen/logrus"
)

var pluralize = gopluralize.NewClient()

func tableNameOf(record interface{}) string {
	rt := reflect.TypeOf(record)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	typeName := rt.Name()
	tableName := strings.Join(camelcase.Split(typeName), "_")
	tableName = strings.ToLower(tableName)
	return pluralize.Plural(tableName)
}

func placeholdersOf(record interface{}) []string {
	rt := reflect.TypeOf(record)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	if rt.Kind() != reflect.Struct {
		return nil
	}

	var dbFields []string
	for i := 0; i < rt.NumField(); i++ {
		fieldType := rt.Field(i)
		if tag, ok := fieldType.Tag.Lookup("db"); ok {
			if tag == "gid" || tag == "-" || tag == "" {
				continue
			}

			dbFields = append(dbFields, ":"+tag)
		}
	}

	return dbFields
}

func fieldsNamesOf(record interface{}) []string {
	rt := reflect.TypeOf(record)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	if rt.Kind() != reflect.Struct {
		return nil
	}

	var dbFields []string
	for i := 0; i < rt.NumField(); i++ {
		fieldType := rt.Field(i)
		if tag, ok := fieldType.Tag.Lookup("db"); ok {
			if tag == "gid" || tag == "-" || tag == "" {
				continue
			}

			dbFields = append(dbFields, tag)
		}
	}

	return dbFields
}

func ParseStructTag(s string) (string, map[string]string) {
	opts := make(map[string]string)
	ss := strings.Split(s, ",")
	if len(ss) > 1 {
		for _, opt := range ss[1:] {
			aa := strings.SplitN(opt, "=", 2)
			if len(aa) == 2 {
				opts[aa[0]] = aa[1]
			} else {
				opts[aa[0]] = ""
			}
		}
	}

	return ss[0], opts
}

type ReflectCache struct {
	tableNames   map[string]string
	fields       map[string][]string
	placeholders map[string][]string
	insertSqls   map[string]string
}

func NewReflectCache() *ReflectCache {
	return &ReflectCache{
		tableNames:   make(map[string]string),
		fields:       make(map[string][]string),
		placeholders: make(map[string][]string),
		insertSqls:   make(map[string]string),
	}
}

func (c *ReflectCache) InsertSqlOf(t interface{}) string {
	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	typeName := rt.Name()
	sql, ok := c.insertSqls[typeName]
	if ok {
		return sql
	}

	tableName := dbCache.TableNameOf(t)
	fields := dbCache.FieldsOf(t)
	placeholders := dbCache.PlaceholderOf(t)
	fieldClause := strings.Join(fields, ", ")
	placeholderClause := strings.Join(placeholders, ", ")

	sql = `INSERT INTO ` + tableName + ` (` + fieldClause + `) VALUES (` + placeholderClause + `)`
	c.insertSqls[typeName] = sql
	return sql
}

func (c *ReflectCache) TableNameOf(t interface{}) string {
	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	typeName := rt.Name()
	tableName, ok := c.tableNames[typeName]
	if ok {
		return tableName
	}

	tableName = tableNameOf(t)
	c.tableNames[typeName] = tableName
	return tableName
}

func (c *ReflectCache) PlaceholderOf(t interface{}) []string {
	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	typeName := rt.Name()
	placeholders, ok := c.placeholders[typeName]
	if ok {
		return placeholders
	}

	placeholders = placeholdersOf(t)
	c.placeholders[typeName] = placeholders
	return placeholders
}

func (c *ReflectCache) FieldsOf(t interface{}) []string {
	rt := reflect.TypeOf(t)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	typeName := rt.Name()
	fields, ok := c.fields[typeName]
	if ok {
		return fields
	}

	fields = fieldsNamesOf(t)
	c.fields[typeName] = fields
	return fields
}

// scanRowsOfType use the given type to scan rows
// this is usually slower than the native one since it uses reflect.
func scanRowsOfType(rows *sqlx.Rows, tpe interface{}) (interface{}, error) {
	refType := reflect.TypeOf(tpe)

	if refType.Kind() == reflect.Ptr {
		refType = refType.Elem()
	}

	sliceRef := reflect.MakeSlice(reflect.SliceOf(refType), 0, 100)
	// sliceRef := reflect.New(reflect.SliceOf(refType))
	for rows.Next() {
		var recordRef = reflect.New(refType)
		var record = recordRef.Interface()
		if err := rows.StructScan(record); err != nil {
			return sliceRef.Interface(), err
		}

		sliceRef = reflect.Append(sliceRef, recordRef.Elem())
	}

	return sliceRef.Interface(), rows.Err()
}

func insertType(db *sqlx.DB, record interface{}) error {
	sql := dbCache.InsertSqlOf(record)
	_, err := db.NamedExec(sql, record)
	return err
}

func selectAndScanType(ctx context.Context, db *sqlx.DB, sel squirrel.SelectBuilder, tpe interface{}) (interface{}, error) {
	sql, args, err := sel.ToSql()
	if err != nil {
		return nil, err
	}

	logrus.Debugf("selectAndScanType: %T <- %s", tpe, sql)
	logrus.Debugf("queryArgs: %v", args)

	rows, err := db.QueryxContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return scanRowsOfType(rows, tpe)
}
