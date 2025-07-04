package locker

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
)

// mockDriver implements the database/sql/driver interfaces for testing
type mockDriver struct {
	connectError error
	queryError   error
	queryResult  int64
}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	if d.connectError != nil {
		return nil, d.connectError
	}
	return &mockConn{driver: d}, nil
}

type mockConn struct {
	driver *mockDriver
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{conn: c, query: query}, nil
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

type mockStmt struct {
	conn  *mockConn
	query string
}

func (s *mockStmt) Close() error {
	return nil
}

func (s *mockStmt) NumInput() int {
	return -1
}

func (s *mockStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, errors.New("not implemented")
}

func (s *mockStmt) Query(args []driver.Value) (driver.Rows, error) {
	if s.conn.driver.queryError != nil {
		return nil, s.conn.driver.queryError
	}
	return &mockRows{result: s.conn.driver.queryResult, valid: true}, nil
}

type mockRows struct {
	result int64
	valid  bool
	read   bool
}

func (r *mockRows) Columns() []string {
	return []string{"result"}
}

func (r *mockRows) Close() error {
	return nil
}

func (r *mockRows) Next(dest []driver.Value) error {
	if r.read {
		return errors.New("EOF")
	}
	r.read = true
	if r.valid {
		dest[0] = r.result
	} else {
		dest[0] = nil
	}
	return nil
}

// pingableConn implements driver.Pinger
type pingableConn struct {
	mockConn
	pingErr error
}

func (c *pingableConn) Ping(ctx context.Context) error {
	return c.pingErr
}

// mockDriverWithPing extends mockDriver to support Ping
type mockDriverWithPing struct {
	mockDriver
	pingErr error
}

func (d *mockDriverWithPing) Open(name string) (driver.Conn, error) {
	if d.connectError != nil {
		return nil, d.connectError
	}
	return &pingableConn{
		mockConn: mockConn{driver: &d.mockDriver},
		pingErr:  d.pingErr,
	}, nil
}

func TestNewLocker_Coverage(t *testing.T) {
	// Register mock drivers
	sql.Register("mock-success", &mockDriverWithPing{})
	sql.Register("mock-ping-fail", &mockDriverWithPing{pingErr: errors.New("ping failed")})
	sql.Register("mock-connect-fail", &mockDriverWithPing{mockDriver: mockDriver{connectError: errors.New("connect failed")}})

	tests := []struct {
		name    string
		dsn     string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty DSN",
			dsn:     "",
			wantErr: true,
			errMsg:  "DSN is required",
		},
		{
			name:    "successful connection",
			dsn:     "mock-success://test",
			wantErr: false,
		},
		{
			name:    "ping failure",
			dsn:     "mock-ping-fail://test",
			wantErr: true,
			errMsg:  "failed to ping database",
		},
		{
			name:    "connection failure",
			dsn:     "mock-connect-fail://test",
			wantErr: true,
			errMsg:  "failed to open database",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			locker, err := NewLocker(tt.dsn)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewLocker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("NewLocker() error = %v, want to contain %v", err, tt.errMsg)
				}
			}
			if locker != nil {
				locker.Close()
			}
		})
	}
}

func TestLocker_Close_Coverage(t *testing.T) {
	t.Run("close with nil db", func(t *testing.T) {
		l := &Locker{db: nil}
		if err := l.Close(); err != nil {
			t.Errorf("Close() with nil db should not error, got %v", err)
		}
	})

	t.Run("close with valid db", func(t *testing.T) {
		sql.Register("mock-close", &mockDriverWithPing{})
		db, _ := sql.Open("mock-close", "test")
		l := &Locker{db: db}
		if err := l.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestLocker_AcquireLock_Coverage(t *testing.T) {
	// Register mock driver for queries
	md := &mockDriver{queryResult: 1}
	sql.Register("mock-query", md)
	
	db, _ := sql.Open("mock-query", "test")
	l := &Locker{db: db}
	defer l.Close()

	ctx := context.Background()

	tests := []struct {
		name        string
		lockName    string
		timeout     int
		queryResult int64
		queryError  error
		want        bool
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "successful acquisition",
			lockName:    "test-lock",
			timeout:     5,
			queryResult: 1,
			want:        true,
			wantErr:     false,
		},
		{
			name:        "lock not acquired (result 0)",
			lockName:    "test-lock",
			timeout:     5,
			queryResult: 0,
			want:        false,
			wantErr:     false,
		},
		{
			name:       "query error",
			lockName:   "test-lock",
			timeout:    5,
			queryError: errors.New("query failed"),
			want:       false,
			wantErr:    true,
			errMsg:     "failed to acquire lock",
		},
		{
			name:     "invalid lock name",
			lockName: "",
			timeout:  5,
			want:     false,
			wantErr:  true,
			errMsg:   "lock name is required",
		},
		{
			name:     "invalid timeout",
			lockName: "test-lock",
			timeout:  0,
			want:     false,
			wantErr:  true,
			errMsg:   "timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md.queryResult = tt.queryResult
			md.queryError = tt.queryError

			got, err := l.AcquireLock(ctx, tt.lockName, tt.timeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("AcquireLock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("AcquireLock() error = %v, want to contain %v", err, tt.errMsg)
				}
			}
			if got != tt.want {
				t.Errorf("AcquireLock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocker_ReleaseLock_Coverage(t *testing.T) {
	md := &mockDriver{queryResult: 1}
	sql.Register("mock-release", md)
	
	db, _ := sql.Open("mock-release", "test")
	l := &Locker{db: db}
	defer l.Close()

	ctx := context.Background()

	tests := []struct {
		name        string
		lockName    string
		queryResult int64
		queryError  error
		want        bool
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "successful release",
			lockName:    "test-lock",
			queryResult: 1,
			want:        true,
			wantErr:     false,
		},
		{
			name:        "lock not released (result 0)",
			lockName:    "test-lock",
			queryResult: 0,
			want:        false,
			wantErr:     false,
		},
		{
			name:       "query error",
			lockName:   "test-lock",
			queryError: errors.New("query failed"),
			want:       false,
			wantErr:    true,
			errMsg:     "failed to release lock",
		},
		{
			name:     "invalid lock name",
			lockName: "",
			want:     false,
			wantErr:  true,
			errMsg:   "lock name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md.queryResult = tt.queryResult
			md.queryError = tt.queryError

			got, err := l.ReleaseLock(ctx, tt.lockName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReleaseLock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ReleaseLock() error = %v, want to contain %v", err, tt.errMsg)
				}
			}
			if got != tt.want {
				t.Errorf("ReleaseLock() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocker_WithLock_Coverage(t *testing.T) {
	tests := []struct {
		name         string
		lockName     string
		timeout      int
		acquireOk    bool
		acquireErr   error
		fnErr        error
		releaseErr   error
		wantErr      bool
		wantErrType  error
	}{
		{
			name:      "successful execution",
			lockName:  "test-lock",
			timeout:   5,
			acquireOk: true,
			wantErr:   false,
		},
		{
			name:      "function returns error",
			lockName:  "test-lock",
			timeout:   5,
			acquireOk: true,
			fnErr:     errors.New("function error"),
			wantErr:   true,
		},
		{
			name:        "lock timeout",
			lockName:    "test-lock",
			timeout:     5,
			acquireOk:   false,
			wantErr:     true,
			wantErrType: ErrLockTimeout,
		},
		{
			name:       "acquire error",
			lockName:   "test-lock",
			timeout:    5,
			acquireErr: errors.New("acquire failed"),
			wantErr:    true,
		},
		{
			name:       "release error (should not affect result)",
			lockName:   "test-lock",
			timeout:    5,
			acquireOk:  true,
			releaseErr: errors.New("release failed"),
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock driver for this test
			md := &mockDriver{}
			driverName := "mock-withlock-" + tt.name
			sql.Register(driverName, md)
			
			db, _ := sql.Open(driverName, "test")
			l := &Locker{db: db}
			defer l.Close()

			// Setup mock behavior
			if tt.acquireErr != nil {
				md.queryError = tt.acquireErr
			} else if tt.acquireOk {
				md.queryResult = 1
			} else {
				md.queryResult = 0
			}

			ctx := context.Background()
			executed := false
			err := l.WithLock(ctx, tt.lockName, tt.timeout, func() error {
				executed = true
				// For release test, change the mock behavior
				if tt.releaseErr != nil {
					md.queryError = tt.releaseErr
				}
				return tt.fnErr
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("WithLock() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErrType != nil && !errors.Is(err, tt.wantErrType) {
				t.Errorf("WithLock() error = %v, want error type %v", err, tt.wantErrType)
			}
			if tt.acquireOk && tt.acquireErr == nil && !executed {
				t.Errorf("WithLock() function was not executed")
			}
		})
	}
}

func TestExitCode_Coverage(t *testing.T) {
	if got := ExitCode(nil); got != 0 {
		t.Errorf("ExitCode(nil) = %v, want 0", got)
	}
	if got := ExitCode(ErrLockTimeout); got != LockTimeout {
		t.Errorf("ExitCode(ErrLockTimeout) = %v, want %v", got, LockTimeout)
	}
	if got := ExitCode(errors.New("other")); got != InternalError {
		t.Errorf("ExitCode(other error) = %v, want %v", got, InternalError)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}