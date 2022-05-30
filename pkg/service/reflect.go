package service

import (
	"reflect"
	"strings"

	"github.com/fatih/camelcase"
	gopluralize "github.com/gertd/go-pluralize"
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
			dbFields = append(dbFields, tag)
		}
	}

	return dbFields
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
