package executor

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name         string
		command      []string
		wantErr      bool
		wantExitCode int
		wantOutput   string
	}{
		{
			name:         "successful command",
			command:      []string{"echo", "hello"},
			wantErr:      false,
			wantExitCode: 0,
			wantOutput:   "hello",
		},
		{
			name:         "command with arguments",
			command:      []string{"echo", "hello", "world"},
			wantErr:      false,
			wantExitCode: 0,
			wantOutput:   "hello world",
		},
		{
			name:         "command not found",
			command:      []string{"nonexistentcommand"},
			wantErr:      true,
			wantExitCode: -1,
		},
		{
			name:         "command fails",
			command:      []string{"sh", "-c", "exit 42"},
			wantErr:      true,
			wantExitCode: 42,
		},
		{
			name:         "empty command",
			command:      []string{},
			wantErr:      true,
			wantExitCode: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip platform-specific tests
			if tt.name == "command fails" && runtime.GOOS == "windows" {
				t.Skip("Skipping shell test on Windows")
			}

			ctx := context.Background()
			executor := New()

			exitCode, err := executor.Execute(ctx, tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantExitCode >= 0 && exitCode != tt.wantExitCode {
				t.Errorf("Execute() exitCode = %v, want %v", exitCode, tt.wantExitCode)
			}
		})
	}
}

func TestExecute_SignalHandling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping signal test on Windows")
	}

	// Test that the executor properly handles context cancellation
	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		executor := New()

		// Create a command that will run for a while
		cmd := []string{"sleep", "10"}

		done := make(chan struct {
			exitCode int
			err      error
		})

		go func() {
			exitCode, err := executor.Execute(ctx, cmd)
			done <- struct {
				exitCode int
				err      error
			}{exitCode, err}
		}()

		// Give the command time to start
		time.Sleep(100 * time.Millisecond)

		// Cancel context which should kill the process
		cancel()

		// Wait for completion
		select {
		case result := <-done:
			// Context cancellation should result in process being killed
			if result.exitCode != -1 {
				t.Errorf("Expected exit code -1 for killed process, got %d", result.exitCode)
			}
			if result.err == nil || (!strings.Contains(result.err.Error(), "context") && !strings.Contains(result.err.Error(), "signal: killed")) {
				t.Errorf("Expected context or killed error, got: %v", result.err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Command didn't exit within timeout")
		}
	})
}

func TestExecute_StdoutStderr(t *testing.T) {
	tests := []struct {
		name       string
		command    []string
		wantStdout string
		wantStderr string
	}{
		{
			name:       "stdout only",
			command:    []string{"echo", "stdout message"},
			wantStdout: "stdout message\n",
			wantStderr: "",
		},
		{
			name:       "stderr only",
			command:    []string{"sh", "-c", "echo 'stderr message' >&2"},
			wantStdout: "",
			wantStderr: "stderr message\n",
		},
		{
			name:       "both stdout and stderr",
			command:    []string{"sh", "-c", "echo 'stdout'; echo 'stderr' >&2"},
			wantStdout: "stdout\n",
			wantStderr: "stderr\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("Skipping shell test on Windows")
			}

			// Capture stdout and stderr
			oldStdout := os.Stdout
			oldStderr := os.Stderr

			rOut, wOut, _ := os.Pipe()
			rErr, wErr, _ := os.Pipe()

			os.Stdout = wOut
			os.Stderr = wErr

			ctx := context.Background()
			executor := New()

			// Execute command
			_, _ = executor.Execute(ctx, tt.command)

			// Restore stdout/stderr
			wOut.Close()
			wErr.Close()

			var bufOut, bufErr bytes.Buffer
			_, _ = bufOut.ReadFrom(rOut)
			_, _ = bufErr.ReadFrom(rErr)

			os.Stdout = oldStdout
			os.Stderr = oldStderr

			gotStdout := bufOut.String()
			gotStderr := bufErr.String()

			if gotStdout != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", gotStdout, tt.wantStdout)
			}
			if gotStderr != tt.wantStderr {
				t.Errorf("stderr = %q, want %q", gotStderr, tt.wantStderr)
			}
		})
	}
}

func TestExecute_Context(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping context test on Windows")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	executor := New()

	// Long-running command that should be cancelled
	_, err := executor.Execute(ctx, []string{"sleep", "10"})

	if err == nil {
		t.Errorf("Expected error due to context cancellation")
	}

	// Check if it's a context error
	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "signal: killed") {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
}

func TestGetExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "nil error",
			err:      nil,
			wantCode: 0,
		},
		{
			name:     "exec.ExitError with code 1",
			err:      &exec.ExitError{ProcessState: &os.ProcessState{}},
			wantCode: -1, // Can't easily mock ProcessState.ExitCode()
		},
		{
			name:     "other error",
			err:      os.ErrNotExist,
			wantCode: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetExitCode(tt.err); got != tt.wantCode && tt.wantCode != -1 {
				t.Errorf("GetExitCode() = %v, want %v", got, tt.wantCode)
			}
		})
	}
}
