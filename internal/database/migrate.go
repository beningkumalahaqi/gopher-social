package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/haqi/gophersocial/migrations"
)

// Migrate applies migrations embedded in the binary. direction is "up", "down"
// (one step), "drop" (all down) or "version".
func Migrate(databaseURL, direction string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	// golang-migrate's driver wants a *sql.DB. pgx's database/sql adapter uses
	// the same driver and DSN as the application.
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()

	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("create migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}

	switch direction {
	case "up":
		err = m.Up()
	case "down":
		err = m.Steps(-1)
	case "drop":
		err = m.Down()
	case "version":
		v, dirty, verr := m.Version()
		if verr != nil && !errors.Is(verr, migrate.ErrNilVersion) {
			return fmt.Errorf("read migration version: %w", verr)
		}
		fmt.Printf("version=%d dirty=%t\n", v, dirty)
		return nil
	default:
		return fmt.Errorf("unknown migration direction %q", direction)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate %s: %w", direction, err)
	}
	return nil
}

// MigrationFiles exposes the embedded SQL filesystem for tests and docs tools.
func MigrationFiles() fs.FS { return migrations.FS }

// MigrateForce marks the schema as being at version without running SQL. It is
// a recovery tool for a dirty migration; use it only after inspecting the
// database.
func MigrateForce(databaseURL string, version int) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()
	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("create migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	if err := m.Force(version); err != nil {
		return fmt.Errorf("force version %d: %w", version, err)
	}
	return nil
}
