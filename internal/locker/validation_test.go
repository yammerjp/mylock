package locker

import (
	"strings"
	"testing"
)

func TestValidateLockName(t *testing.T) {
	tests := []struct {
		name     string
		lockName string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid simple name",
			lockName: "my-lock",
			wantErr:  false,
		},
		{
			name:     "valid with underscore",
			lockName: "my_lock_name",
			wantErr:  false,
		},
		{
			name:     "valid with dot",
			lockName: "app.module.lock",
			wantErr:  false,
		},
		{
			name:     "valid alphanumeric",
			lockName: "lock123ABC",
			wantErr:  false,
		},
		{
			name:     "valid with all allowed chars",
			lockName: "my-app_v1.2.3_lock",
			wantErr:  false,
		},
		{
			name:     "empty name",
			lockName: "",
			wantErr:  true,
			errMsg:   "lock name is required",
		},
		{
			name:     "too long name",
			lockName: strings.Repeat("a", 65),
			wantErr:  true,
			errMsg:   "lock name too long",
		},
		{
			name:     "with spaces",
			lockName: "my lock",
			wantErr:  true,
			errMsg:   "invalid characters",
		},
		{
			name:     "with semicolon (SQL injection attempt)",
			lockName: "lock; DROP TABLE users",
			wantErr:  true,
			errMsg:   "invalid characters",
		},
		{
			name:     "with quotes",
			lockName: "lock'name",
			wantErr:  true,
			errMsg:   "invalid characters",
		},
		{
			name:     "with backticks",
			lockName: "lock`name",
			wantErr:  true,
			errMsg:   "invalid characters",
		},
		{
			name:     "with parentheses",
			lockName: "lock()",
			wantErr:  true,
			errMsg:   "invalid characters",
		},
		{
			name:     "consecutive dots",
			lockName: "app..lock",
			wantErr:  true,
			errMsg:   "consecutive dots",
		},
		{
			name:     "with hash for prefix+hash pattern",
			lockName: "myapp-job-d41d8cd98f00b204e9800998ecf8427e",
			wantErr:  false,
		},
		{
			name:     "with SHA256 hash suffix (truncated)",
			lockName: "job.e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852",
			wantErr:  false,
		},
		{
			name:     "SHA256 full hash too long",
			lockName: "cronjob.e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr:  true,
			errMsg:   "too long",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLockName(tt.lockName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateLockName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("validateLockName() error message = %v, want to contain %v", err.Error(), tt.errMsg)
			}
		})
	}
}
func TestLockNamePatternSecurity(t *testing.T) {
	// Test various SQL injection attempts
	dangerousNames := []string{
		"'; DROP TABLE locks; --",
		"1' OR '1'='1",
		"lock\"; DELETE FROM mysql.user; --",
		"lock\\0",
		"lock\nSELECT * FROM information_schema.tables",
		"lock/*comment*/name",
		"lock--comment",
		"lock#comment",
		"lock\x00null",
		"lock\r\ninjection",
		"${INJECTION}",
		"$(command)",
		"`command`",
	}

	for _, name := range dangerousNames {
		t.Run("dangerous: "+name, func(t *testing.T) {
			err := validateLockName(name)
			if err == nil {
				t.Errorf("validateLockName() should reject dangerous name: %q", name)
			}
		})
	}
}
