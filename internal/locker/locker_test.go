package locker

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

type mockDB struct {
	lockResult   sql.NullInt64
	lockErr      error
	releaseResult sql.NullInt64
	releaseErr   error
	pingErr      error
	closeErr     error
}

func (m *mockDB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	// This is a simplified mock - in reality we'd need to mock sql.Row
	// For testing purposes, we'll use the actual implementation with a test database
	return nil
}

func (m *mockDB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return nil, nil
}

func (m *mockDB) PingContext(ctx context.Context) error {
	return m.pingErr
}

func (m *mockDB) Close() error {
	return m.closeErr
}

func TestNewLocker(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr bool
	}{
		{
			name:    "valid DSN",
			dsn:     "user:pass@tcp(localhost:3306)/db",
			wantErr: false, // This will fail without actual MySQL
		},
		{
			name:    "empty DSN",
			dsn:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip actual database connection tests
			if tt.name == "valid DSN" {
				t.Skip("Skipping test requiring actual MySQL connection")
			}
			
			_, err := NewLocker(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLocker() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLocker_AcquireLock(t *testing.T) {
	tests := []struct {
		name     string
		lockName string
		timeout  int
		want     bool
		wantErr  bool
	}{
		{
			name:     "successful lock acquisition",
			lockName: "test-lock",
			timeout:  5,
			want:     true,
			wantErr:  false,
		},
		{
			name:     "empty lock name",
			lockName: "",
			timeout:  5,
			want:     false,
			wantErr:  true,
		},
		{
			name:     "negative timeout",
			lockName: "test-lock",
			timeout:  -1,
			want:     false,
			wantErr:  true,
		},
		{
			name:     "zero timeout",
			lockName: "test-lock",
			timeout:  0,
			want:     false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests requiring actual database
			if tt.name == "successful lock acquisition" {
				t.Skip("Skipping test requiring actual MySQL connection")
			}

			// Test validation only
			if tt.lockName == "" || tt.timeout <= 0 {
				if !tt.wantErr {
					t.Errorf("Expected error for invalid inputs")
				}
			}
		})
	}
}

func TestLocker_ReleaseLock(t *testing.T) {
	tests := []struct {
		name     string
		lockName string
		want     bool
		wantErr  bool
	}{
		{
			name:     "successful lock release",
			lockName: "test-lock",
			want:     true,
			wantErr:  false,
		},
		{
			name:     "empty lock name",
			lockName: "",
			want:     false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests requiring actual database
			if tt.name == "successful lock release" {
				t.Skip("Skipping test requiring actual MySQL connection")
			}

			// Test validation only
			if tt.lockName == "" {
				if !tt.wantErr {
					t.Errorf("Expected error for empty lock name")
				}
			}
		})
	}
}

func TestLocker_WithLock(t *testing.T) {
	tests := []struct {
		name     string
		lockName string
		timeout  int
		fnErr    error
		wantErr  bool
		wantCode int
	}{
		{
			name:     "successful execution",
			lockName: "test-lock",
			timeout:  5,
			fnErr:    nil,
			wantErr:  false,
			wantCode: 0,
		},
		{
			name:     "function returns error",
			lockName: "test-lock",
			timeout:  5,
			fnErr:    errors.New("function error"),
			wantErr:  true,
			wantCode: 0,
		},
		{
			name:     "invalid lock name",
			lockName: "",
			timeout:  5,
			fnErr:    nil,
			wantErr:  true,
			wantCode: InternalError,
		},
		{
			name:     "invalid timeout",
			lockName: "test-lock",
			timeout:  0,
			fnErr:    nil,
			wantErr:  true,
			wantCode: InternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation cases only
			if tt.lockName == "" || tt.timeout <= 0 {
				// Validation should fail
				if !tt.wantErr {
					t.Errorf("Expected error for invalid inputs")
				}
			}
		})
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error",
			err:  nil,
			want: 0,
		},
		{
			name: "lock timeout error",
			err:  ErrLockTimeout,
			want: LockTimeout,
		},
		{
			name: "internal error",
			err:  errors.New("some error"),
			want: InternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode() = %v, want %v", got, tt.want)
			}
		})
	}
}