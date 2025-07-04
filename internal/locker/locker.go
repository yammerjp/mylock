package locker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	LockTimeout   = 200
	InternalError = 201
)

var (
	ErrLockTimeout = errors.New("failed to acquire lock within timeout")
	// Safe pattern for lock names: alphanumeric, underscore, hyphen, dot
	lockNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)
)

// validateLockName ensures the lock name is safe for MySQL
func validateLockName(lockName string) error {
	if lockName == "" {
		return errors.New("lock name is required")
	}
	if len(lockName) > 64 {
		return errors.New("lock name too long (max 64 characters)")
	}
	if !lockNamePattern.MatchString(lockName) {
		return errors.New("lock name contains invalid characters (use only alphanumeric, underscore, hyphen, dot)")
	}
	if strings.Contains(lockName, "..") {
		return errors.New("lock name contains consecutive dots")
	}
	if strings.Contains(lockName, "--") {
		return errors.New("lock name contains consecutive hyphens")
	}
	return nil
}

type Locker struct {
	db *sql.DB
}

func NewLocker(dsn string) (*Locker, error) {
	if dsn == "" {
		return nil, errors.New("DSN is required")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Locker{db: db}, nil
}

func (l *Locker) Close() error {
	if l.db != nil {
		return l.db.Close()
	}
	return nil
}

func (l *Locker) AcquireLock(ctx context.Context, lockName string, timeout int) (bool, error) {
	if err := validateLockName(lockName); err != nil {
		return false, err
	}
	if timeout <= 0 {
		return false, errors.New("timeout must be positive")
	}

	var result sql.NullInt64
	query := "SELECT GET_LOCK(?, ?)"
	err := l.db.QueryRowContext(ctx, query, lockName, timeout).Scan(&result)
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !result.Valid || result.Int64 != 1 {
		return false, nil
	}

	return true, nil
}

func (l *Locker) ReleaseLock(ctx context.Context, lockName string) (bool, error) {
	if err := validateLockName(lockName); err != nil {
		return false, err
	}

	var result sql.NullInt64
	query := "SELECT RELEASE_LOCK(?)"
	err := l.db.QueryRowContext(ctx, query, lockName).Scan(&result)
	if err != nil {
		return false, fmt.Errorf("failed to release lock: %w", err)
	}

	if !result.Valid || result.Int64 != 1 {
		return false, nil
	}

	return true, nil
}

func (l *Locker) WithLock(ctx context.Context, lockName string, timeout int, fn func() error) error {
	acquired, err := l.AcquireLock(ctx, lockName, timeout)
	if err != nil {
		return err
	}

	if !acquired {
		return ErrLockTimeout
	}

	defer func() {
		releaseCtx := context.Background()
		_, releaseErr := l.ReleaseLock(releaseCtx, lockName)
		if releaseErr != nil {
			// Log error but don't override the function error
			fmt.Printf("Warning: failed to release lock: %v\n", releaseErr)
		}
	}()

	return fn()
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrLockTimeout) {
		return LockTimeout
	}
	return InternalError
}
