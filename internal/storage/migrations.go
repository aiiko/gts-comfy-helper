package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed migrations/0001_init.sql
var migration0001 string

func runMigrations(ctx context.Context, conn *sql.DB) error {
	if _, err := conn.ExecContext(ctx, migration0001); err != nil {
		return fmt.Errorf("run 0001_init.sql: %w", err)
	}
	return nil
}
