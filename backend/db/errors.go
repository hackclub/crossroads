package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// IsViolation reports whether err is Postgres rejecting a write because of the
// named constraint or index.
func IsViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == constraint
}

// IsNotEnoughStamps reports whether err came from a purchase that would have
// taken someone's stamps below zero.
func IsNotEnoughStamps(err error) bool {
	return IsViolation(err, "stamps_not_negative")
}
