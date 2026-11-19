package store

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// ValidateProductionDSN rejects non-URL and non-verified PostgreSQL connections.
func ValidateProductionDSN(dsn string) error {
	parsed, err := url.Parse(strings.TrimSpace(dsn))
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || parsed.Hostname() == "" {
		return errors.New("production ARIMA_DATABASE_URL must be a PostgreSQL URL with a hostname")
	}
	values := parsed.Query()["sslmode"]
	if len(values) != 1 || values[0] != "verify-full" {
		return errors.New("production ARIMA_DATABASE_URL must use sslmode=verify-full")
	}
	return nil
}

// OpenPostgres opens and verifies PostgreSQL connectivity before the service listens.
func OpenPostgres(ctx context.Context, dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("ARIMA_DATABASE_URL is required")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
