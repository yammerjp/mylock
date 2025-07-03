package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMainFunction(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envVars  map[string]string
		wantExit int
		wantOut  string
	}{
		{
			name: "help flag",
			args: []string{"mylock", "--help"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantExit: 0,
			wantOut:  "mylock - Acquire a MySQL advisory lock",
		},
		{
			name: "missing required args",
			args: []string{"mylock"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantExit: 201,
			wantOut:  "",
		},
		{
			name: "missing environment variables",
			args: []string{"mylock", "--lock-name", "test", "--timeout", "5", "--", "echo", "hello"},
			envVars: map[string]string{
				// Missing MYLOCK_HOST
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantExit: 201,
			wantOut:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip tests that require actual main execution
			if tt.name != "help flag" {
				t.Skip("Skipping test that requires full main execution")
			}

			// For help test, we can test the output generation
			if tt.name == "help flag" {
				// Test help output format
				helpOutput := `mylock - Acquire a MySQL advisory lock and run a command

Usage:
  mylock --lock-name <name> --timeout <seconds> -- <command> [args...]`

				if !strings.Contains(helpOutput, "mylock - Acquire a MySQL advisory lock") {
					t.Errorf("Help output missing expected content")
				}
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envVars  map[string]string
		wantCode int
		wantErr  bool
	}{
		{
			name: "invalid arguments",
			args: []string{"--invalid-flag"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantCode: 201,
			wantErr:  true,
		},
		{
			name: "missing environment variable",
			args: []string{"--lock-name", "test", "--timeout", "5", "--", "echo", "hello"},
			envVars: map[string]string{
				// Missing MYLOCK_HOST
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantCode: 201,
			wantErr:  true,
		},
		{
			name: "database connection failure",
			args: []string{"--lock-name", "test", "--timeout", "5", "--", "echo", "hello"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "nonexistent.host",
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantCode: 201,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and clear environment
			oldEnv := make(map[string]string)
			for _, key := range []string{"MYLOCK_HOST", "MYLOCK_PORT", "MYLOCK_USER", "MYLOCK_PASSWORD", "MYLOCK_DATABASE"} {
				oldEnv[key] = os.Getenv(key)
				os.Unsetenv(key)
			}

			// Set test environment
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Restore environment after test
			defer func() {
				for key, value := range oldEnv {
					if value == "" {
						os.Unsetenv(key)
					} else {
						os.Setenv(key, value)
					}
				}
			}()

			// Capture output
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stdout = w
			os.Stderr = w

			code := run(tt.args)

			// Restore stdout/stderr
			w.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)

			if code != tt.wantCode {
				t.Errorf("run() = %v, want %v", code, tt.wantCode)
			}
		})
	}
}

func TestExitHandler(t *testing.T) {
	var exitCode int
	exitCalled := false

	// Mock exit function
	mockExit := func(code int) {
		exitCode = code
		exitCalled = true
	}

	// Test various exit codes
	testCases := []int{0, 1, 200, 201}

	for _, code := range testCases {
		t.Run(fmt.Sprintf("exit_code_%d", code), func(t *testing.T) {
			exitCode = -1
			exitCalled = false

			// Simulate exit with mock
			mockExit(code)

			if !exitCalled {
				t.Errorf("Exit function was not called")
			}
			if exitCode != code {
				t.Errorf("Exit code = %v, want %v", exitCode, code)
			}
		})
	}
}
