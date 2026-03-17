package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"sync"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var fs embed.FS

// DB wraps the standard sql.DB with a mutex for write operations to ensure
// a single writer in SQLite WAL mode.
type DB struct {
	*sql.DB
	writeMu sync.Mutex
}

// Open initializes and returns a new SQLite database connection with WAL mode
// and other optimized PRAGMAs.
func Open(path string) (*DB, error) {
	// Enable busy_timeout and foreign keys via DSN
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(10000)&_pragma=foreign_keys(1)", path)
	
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	// PRAGMA settings optimized for P2P/WAL from docs
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA synchronous = NORMAL;",
		"PRAGMA cache_size = -64000;", // 64MB cache
		"PRAGMA mmap_size = 268435456;", // 256MB mmap
		"PRAGMA temp_store = MEMORY;",
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("exec pragma %q: %w", p, err)
		}
	}

	// Connection pool settings
	db.SetMaxOpenConns(16) // Multiple readers
	db.SetMaxIdleConns(4)

	return &DB{DB: db}, nil
}

// WriteTransaction executes the given function within a database transaction,
// protected by the write mutex to ensure single-writer concurrency.
func (db *DB) WriteTransaction(fn func(*sql.Tx) error) error {
	db.writeMu.Lock()
	defer db.writeMu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction failed: %v (rollback failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// RunMigrations applies all pending migrations using the embedded SQL files.
func RunMigrations(db *DB) error {
	d, err := iofs.New(fs, "migrations")
	if err != nil {
		return fmt.Errorf("create iofs source: %w", err)
	}

	driver, err := sqlite.WithInstance(db.DB, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("create sqlite driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "sqlite", driver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
