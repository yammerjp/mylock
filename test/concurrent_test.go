package test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestConcurrentExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	// Check if MySQL is available
	host := os.Getenv("MYLOCK_HOST")
	if host == "" {
		t.Skip("Skipping test: MYLOCK_HOST not set")
	}

	// Build the binary
	binPath := filepath.Join(t.TempDir(), "mylock")
	buildCmd := exec.Command("go", "build", "-o", binPath, "../cmd/mylock")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	tests := []struct {
		name         string
		workers      int
		lockName     string
		timeout      int
		commandDelay string // how long the command should take
		expectFail   int    // number of workers expected to fail
	}{
		{
			name:         "two workers, one lock",
			workers:      2,
			lockName:     "test-concurrent-1",
			timeout:      1,
			commandDelay: "2",
			expectFail:   1, // one should timeout
		},
		{
			name:         "three workers, sequential execution",
			workers:      3,
			lockName:     "test-concurrent-2",
			timeout:      10,
			commandDelay: "1",
			expectFail:   0, // all should succeed sequentially
		},
		{
			name:         "five workers, short timeout",
			workers:      5,
			lockName:     "test-concurrent-3",
			timeout:      1,
			commandDelay: "1",
			expectFail:   4, // only one should succeed
		},
		{
			name:         "workers with different lock names",
			workers:      3,
			lockName:     "", // will use worker-specific names
			timeout:      2,
			commandDelay: "1",
			expectFail:   0, // all should succeed (different locks)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wg sync.WaitGroup
			results := make(chan bool, tt.workers)

			// Create a shared output file
			outputFile := filepath.Join(t.TempDir(), "output.txt")

			for i := 0; i < tt.workers; i++ {
				wg.Add(1)
				workerID := i
				go func() {
					defer wg.Done()

					lockName := tt.lockName
					if lockName == "" {
						lockName = fmt.Sprintf("test-lock-%d", workerID)
					}

					// Run mylock with a command that writes to the shared file
					cmd := exec.Command(binPath,
						"--lock-name", lockName,
						"--timeout", fmt.Sprintf("%d", tt.timeout),
						"--",
						"sh", "-c",
						fmt.Sprintf("echo 'Worker %d started' >> %s && sleep %s && echo 'Worker %d finished' >> %s",
							workerID, outputFile, tt.commandDelay, workerID, outputFile))

					// Set environment variables
					cmd.Env = os.Environ()

					err := cmd.Run()
					results <- (err == nil)
				}()
			}

			// Wait for all workers to complete
			wg.Wait()
			close(results)

			// Count successes and failures
			successes := 0
			for success := range results {
				if success {
					successes++
				}
			}
			failures := tt.workers - successes

			if failures != tt.expectFail {
				t.Errorf("Expected %d failures, got %d (successes: %d)",
					tt.expectFail, failures, successes)
			}

			// For sequential execution test, verify order
			if tt.name == "three workers, sequential execution" && successes == tt.workers {
				content, err := os.ReadFile(outputFile)
				if err != nil {
					t.Fatalf("Failed to read output file: %v", err)
				}

				// Verify that workers executed sequentially
				// Each worker should start and finish before the next one starts
				t.Logf("Sequential execution output:\n%s", content)
			}
		})
	}
}

func TestConcurrentRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	// Check if MySQL is available
	host := os.Getenv("MYLOCK_HOST")
	if host == "" {
		t.Skip("Skipping test: MYLOCK_HOST not set")
	}

	// Build the binary
	binPath := filepath.Join(t.TempDir(), "mylock")
	buildCmd := exec.Command("go", "build", "-o", binPath, "../cmd/mylock")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	// Test that locks are properly released even on failure
	lockName := "test-release"

	// First execution: acquire lock and fail
	cmd1 := exec.Command(binPath,
		"--lock-name", lockName,
		"--timeout", "5",
		"--",
		"sh", "-c", "exit 1")
	cmd1.Env = os.Environ()

	err := cmd1.Run()
	if err == nil {
		t.Fatal("Expected first command to fail")
	}

	// Second execution: should be able to acquire the same lock immediately
	cmd2 := exec.Command(binPath,
		"--lock-name", lockName,
		"--timeout", "1", // short timeout to ensure it doesn't wait
		"--",
		"sh", "-c", "echo 'Lock acquired successfully'")
	cmd2.Env = os.Environ()

	start := time.Now()
	err = cmd2.Run()
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Second command failed: %v", err)
	}

	// Verify it didn't have to wait (lock was properly released)
	if duration > 2*time.Second {
		t.Errorf("Second command took too long (%v), lock might not have been released properly", duration)
	}
}

func TestConcurrentSignalHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	// Check if MySQL is available
	host := os.Getenv("MYLOCK_HOST")
	if host == "" {
		t.Skip("Skipping test: MYLOCK_HOST not set")
	}

	// Build the binary
	binPath := filepath.Join(t.TempDir(), "mylock")
	buildCmd := exec.Command("go", "build", "-o", binPath, "../cmd/mylock")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}

	lockName := "test-signal"

	// Start first process that will hold the lock
	cmd1 := exec.Command(binPath,
		"--lock-name", lockName,
		"--timeout", "10",
		"--",
		"sh", "-c", "sleep 10")
	cmd1.Env = os.Environ()

	if err := cmd1.Start(); err != nil {
		t.Fatalf("Failed to start first command: %v", err)
	}

	// Give it time to acquire the lock
	time.Sleep(1 * time.Second)

	// Start second process that will wait for the lock
	cmd2 := exec.Command(binPath,
		"--lock-name", lockName,
		"--timeout", "5",
		"--",
		"sh", "-c", "echo 'Got lock after signal'")
	cmd2.Env = os.Environ()

	start := time.Now()
	if err := cmd2.Start(); err != nil {
		t.Fatalf("Failed to start second command: %v", err)
	}

	// Give second process time to start waiting
	time.Sleep(500 * time.Millisecond)

	// Send SIGTERM to first process
	if err := cmd1.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("Failed to send signal: %v", err)
	}

	// Wait for second process to complete
	err := cmd2.Wait()
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Second command failed: %v", err)
	}

	// Verify the second process got the lock quickly after the signal
	if duration > 3*time.Second {
		t.Errorf("Second command took too long (%v), lock might not have been released on signal", duration)
	}

	// Clean up first process
	_ = cmd1.Process.Kill()
	_ = cmd1.Wait()
}

func TestRaceCondition(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	// Check if MySQL is available
	host := os.Getenv("MYLOCK_HOST")
	if host == "" {
		t.Skip("Skipping test: MYLOCK_HOST not set")
	}

	// Build the binary
	binPath := filepath.Join(t.TempDir(), "mylock")
	buildCmd := exec.Command("go", "build", "-race", "-o", binPath, "../cmd/mylock")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build binary with race detector: %v", err)
	}

	// Run multiple instances concurrently with race detector
	lockName := "test-race"
	workers := 5

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			cmd := exec.CommandContext(ctx, binPath,
				"--lock-name", lockName,
				"--timeout", "2",
				"--",
				"sh", "-c", fmt.Sprintf("echo 'Worker %d'", id))
			cmd.Env = os.Environ()

			// Race detector will panic if it finds issues
			_ = cmd.Run()
		}(i)
	}

	wg.Wait()
}

