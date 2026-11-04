// Package db holds the schema, the sqlc queries and the code sqlc generates
// from them. Edit migrations/ and queries/, then run `go generate ./db`.
package db

//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate -f ../sqlc.yaml

import (
	"context"
	"embed"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies any migrations that haven't run yet. A Postgres advisory
// lock keeps two instances starting at once from both running them.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	provider, closeDB, err := migrator(pool)
	if err != nil {
		return err
	}
	defer closeDB()

	_, err = provider.Up(ctx)
	return err
}

func migrator(pool *pgxpool.Pool) (*goose.Provider, func() error, error) {
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, nil, err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, nil, err
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, files, goose.WithSessionLocker(locker))
	if err != nil {
		sqlDB.Close()
		return nil, nil, err
	}
	return provider, sqlDB.Close, nil
}
