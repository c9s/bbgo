package service

import "fmt"

func binUuidSelector(tableName, uuidColName string) string {
	qualifiedColName := tableName + "." + uuidColName
	return fmt.Sprintf(`
IF(%s != '', BIN_TO_UUID(%s, true), '') AS %s`, qualifiedColName, qualifiedColName, uuidColName,
	)
}
