package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddGlassnodeTable, downAddGlassnodeTable)

}

func upAddGlassnodeTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `glassnode`\n(\n    `gid`       BIGINT UNSIGNED     NOT NULL AUTO_INCREMENT,\n    `category`  VARCHAR(64)         NOT NULL,\n    `metric`    VARCHAR(64)         NOT NULL,\n    `asset`     VARCHAR(20)         NOT NULL,\n    `interval`  VARCHAR(6)          NOT NULL,\n    `time`      DATETIME(3)         NOT NULL,\n    `key`       VARCHAR(64)         NOT NULL,\n    `value`     DECIMAL(32, 8)      NOT NULL,\n    PRIMARY KEY (`gid`)\n);")
	if err != nil {
		return err
	}

	return err
}

func downAddGlassnodeTable(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `glassnode`;")
	if err != nil {
		return err
	}

	return err
}
