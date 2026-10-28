package database

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"sor.hosting/backend/migrations"
)

func Open(ctx context.Context, path string) (*sql.DB, error) {
	absolute, pathErr := filepath.Abs(path)
	if pathErr != nil {
		return nil, pathErr
	}
	path = absolute
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", (&url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: url.Values{"_pragma": {"foreign_keys(1)", "busy_timeout(5000)", "journal_mode(WAL)"}}.Encode()}).String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, e := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY)"); e != nil {
		return e
	}
	var v int
	if e := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_migrations").Scan(&v); e != nil {
		return e
	}
	if v > 2 {
		return fmt.Errorf("schema incompatible: %d", v)
	}
	for next := v + 1; next <= 2; next++ {
		name := map[int]string{1: "000001_initial.sql", 2: "000002_site_lifecycle.sql"}[next]
		b, e := migrations.Files.ReadFile(name)
		if e != nil {
			return e
		}
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, string(b)); e == nil {
			_, e = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version) VALUES(?)", next)
		}
		if e != nil {
			tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	return nil
}
