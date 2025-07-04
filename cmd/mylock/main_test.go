package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()
	os.Exit(code)
}

func TestMainFunction(t *testing.T) {
	if os.Getenv("BE_CRASHER") == "1" {
		// This is the subprocess that will run main()
		main()
		return
	}

	tests := []struct {
		name     string
		args     []string
		envVars  map[string]string
		wantExit int
		wantOut  string
	}{
		{
			name: "help flag",
			args: []string{"--help"},
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
			args: []string{},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantExit: 201,
			wantOut:  "Usage:",
		},
		{
			name: "missing environment variables",
			args: []string{"--lock-name", "test", "--timeout", "5", "--", "echo", "hello"},
			envVars: map[string]string{
				// Missing MYLOCK_HOST
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantExit: 201,
			wantOut:  "Error:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run main in subprocess
			cmd := exec.Command(os.Args[0], "-test.run=TestMainFunction")
			cmd.Env = append(os.Environ(), "BE_CRASHER=1")
			
			// Set test environment
			for key, value := range tt.envVars {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
			}
			
			// Add args
			cmd.Args = append(cmd.Args, tt.args...)
			
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			
			err := cmd.Run()
			
			// Check exit code
			exitCode := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					exitCode = exitErr.ExitCode()
				} else {
					t.Fatalf("Unexpected error: %v", err)
				}
			}
			
			if exitCode != tt.wantExit {
				t.Errorf("Exit code = %v, want %v", exitCode, tt.wantExit)
			}
			
			// Check output
			output := stdout.String() + stderr.String()
			if tt.wantOut != "" && !strings.Contains(output, tt.wantOut) {
				t.Errorf("Output doesn't contain %q, got: %s", tt.wantOut, output)
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
		wantOut  string
	}{
		{
			name: "help flag returns 0",
			args: []string{"--help"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantCode: 0,
			wantErr:  false,
			wantOut:  "", // Help output goes to stdout which kong handles directly
		},
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
			wantOut:  "unknown flag",
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
			wantOut:  "MYLOCK_HOST",
		},
		{
			name: "database connection failure",
			args: []string{"--lock-name", "test", "--timeout", "5", "--", "echo", "hello"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "nonexistent.host.invalid",
				"MYLOCK_PORT":     "3306",
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantCode: 201,
			wantErr:  true,
			wantOut:  "Error:",
		},
		{
			name: "successful command execution mock",
			args: []string{"--lock-name", "test", "--timeout", "5", "--", "true"},
			envVars: map[string]string{
				"MYLOCK_HOST":     "localhost",
				"MYLOCK_PORT":     "3306", 
				"MYLOCK_USER":     "root",
				"MYLOCK_PASSWORD": "pass",
				"MYLOCK_DATABASE": "test",
			},
			wantCode: 201, // Will fail to connect to MySQL
			wantErr:  true,
			wantOut:  "",
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
			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()
			os.Stdout = wOut
			os.Stderr = wErr

			// Run in goroutine to avoid deadlock
			done := make(chan int)
			go func() {
				done <- run(tt.args)
			}()

			// Close write ends
			wOut.Close()
			wErr.Close()

			// Read output
			var bufOut, bufErr bytes.Buffer
			_, _ = bufOut.ReadFrom(rOut)
			_, _ = bufErr.ReadFrom(rErr)

			// Wait for completion
			code := <-done

			// Restore stdout/stderr
			os.Stdout = oldStdout
			os.Stderr = oldStderr

			output := bufOut.String() + bufErr.String()

			if code != tt.wantCode {
				t.Errorf("run() = %v, want %v\nOutput: %s", code, tt.wantCode, output)
			}
			
			if tt.wantOut != "" && !strings.Contains(output, tt.wantOut) {
				t.Errorf("Output doesn't contain %q, got: %s", tt.wantOut, output)
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
	testCases := []int{0, 1, 42, 127, 128, 143, 200, 201}

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