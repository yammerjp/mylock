//go:build integration
// +build integration

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainIntegration(t *testing.T) {
	// Build the binary
	binPath := filepath.Join(t.TempDir(), "mylock")
	if err := exec.Command("go", "build", "-o", binPath, ".").Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
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
				"MYLOCK_HOST":     "127.0.0.1",
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
				"MYLOCK_HOST":     "127.0.0.1",
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
			cmd := exec.Command(binPath, tt.args...)
			
			// Set test environment
			cmd.Env = os.Environ()
			for key, value := range tt.envVars {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
			
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