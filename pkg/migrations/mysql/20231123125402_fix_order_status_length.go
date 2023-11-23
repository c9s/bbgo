package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upFixOrderStatusLength, downFixOrderStatusLength)

}

func upFixOrderStatusLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `orders`\n    CHANGE `status` `status` varchar(20) NOT NULL;")
	if err != nil {
		return err
	}

	return err
}

func downFixOrderStatusLength(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "SELECT 1;")
	if err != nil {
		return err
	}

	return err
}
