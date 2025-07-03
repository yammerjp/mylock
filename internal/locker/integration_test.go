//go:build integration
// +build integration

package locker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func getTestDSN() string {
	host := os.Getenv("TEST_MYSQL_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("TEST_MYSQL_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("TEST_MYSQL_USER")
	if user == "" {
		user = "testuser"
	}
	password := os.Getenv("TEST_MYSQL_PASSWORD")
	if password == "" {
		password = "testpass"
	}
	database := os.Getenv("TEST_MYSQL_DATABASE")
	if database == "" {
		database = "testdb"
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, database)
}

func TestLocker_Integration_AcquireLock(t *testing.T) {
	dsn := getTestDSN()
	locker, err := NewLocker(dsn)
	if err != nil {
		t.Fatalf("Failed to create locker: %v", err)
	}
	defer locker.Close()

	ctx := context.Background()
	lockName := "test-lock-acquire"

	// Test successful lock acquisition
	acquired, err := locker.AcquireLock(ctx, lockName, 5)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire lock")
	}

	// Test that the same lock cannot be acquired again
	locker2, err := NewLocker(dsn)
	if err != nil {
		t.Fatalf("Failed to create second locker: %v", err)
	}
	defer locker2.Close()

	acquired2, err := locker2.AcquireLock(ctx, lockName, 1)
	if err != nil {
		t.Fatalf("Failed to attempt second lock: %v", err)
	}
	if acquired2 {
		t.Fatal("Should not acquire lock held by another connection")
	}

	// Release lock
	released, err := locker.ReleaseLock(ctx, lockName)
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}
	if !released {
		t.Fatal("Expected to release lock")
	}

	// Now the second locker should be able to acquire it
	acquired3, err := locker2.AcquireLock(ctx, lockName, 1)
	if err != nil {
		t.Fatalf("Failed to acquire lock after release: %v", err)
	}
	if !acquired3 {
		t.Fatal("Expected to acquire lock after release")
	}

	// Clean up
	locker2.ReleaseLock(ctx, lockName)
}

func TestLocker_Integration_ConcurrentLocking(t *testing.T) {
	dsn := getTestDSN()
	lockName := "test-concurrent-lock"
	numWorkers := 5
	workDuration := 100 * time.Millisecond

	var wg sync.WaitGroup
	results := make([]int, numWorkers)
	errors := make([]error, numWorkers)

	startTime := time.Now()

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			locker, err := NewLocker(dsn)
			if err != nil {
				errors[workerID] = err
				return
			}
			defer locker.Close()

			ctx := context.Background()
			err = locker.WithLock(ctx, lockName, 10, func() error {
				// Simulate work
				time.Sleep(workDuration)
				results[workerID] = 1
				return nil
			})

			if err != nil {
				errors[workerID] = err
			}
		}(i)
	}

	wg.Wait()

	// Check that all workers completed
	successCount := 0
	for i, result := range results {
		if errors[i] != nil {
			t.Logf("Worker %d error: %v", i, errors[i])
		}
		if result == 1 {
			successCount++
		}
	}

	if successCount != numWorkers {
		t.Errorf("Expected all %d workers to complete, but only %d did", numWorkers, successCount)
	}

	// Check that execution was serialized (should take at least numWorkers * workDuration)
	elapsed := time.Since(startTime)
	minExpected := time.Duration(numWorkers) * workDuration
	if elapsed < minExpected {
		t.Errorf("Expected execution to take at least %v, but took %v", minExpected, elapsed)
	}
}

func TestLocker_Integration_LockTimeout(t *testing.T) {
	dsn := getTestDSN()
	lockName := "test-timeout-lock"

	// First locker acquires the lock
	locker1, err := NewLocker(dsn)
	if err != nil {
		t.Fatalf("Failed to create locker1: %v", err)
	}
	defer locker1.Close()

	ctx := context.Background()
	acquired, err := locker1.AcquireLock(ctx, lockName, 5)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}
	if !acquired {
		t.Fatal("Expected to acquire lock")
	}
	defer locker1.ReleaseLock(ctx, lockName)

	// Second locker tries to acquire with short timeout
	locker2, err := NewLocker(dsn)
	if err != nil {
		t.Fatalf("Failed to create locker2: %v", err)
	}
	defer locker2.Close()

	start := time.Now()
	err = locker2.WithLock(ctx, lockName, 1, func() error {
		t.Fatal("Should not execute function when lock is not acquired")
		return nil
	})
	elapsed := time.Since(start)

	if err != ErrLockTimeout {
		t.Errorf("Expected ErrLockTimeout, got %v", err)
	}

	// Should timeout after approximately 1 second
	if elapsed < 900*time.Millisecond || elapsed > 1500*time.Millisecond {
		t.Errorf("Expected timeout after ~1 second, but took %v", elapsed)
	}
}

func TestLocker_Integration_MultipleLocksNonBlocking(t *testing.T) {
	dsn := getTestDSN()

	locker, err := NewLocker(dsn)
	if err != nil {
		t.Fatalf("Failed to create locker: %v", err)
	}
	defer locker.Close()

	ctx := context.Background()

	// Acquire multiple different locks
	locks := []string{"lock1", "lock2", "lock3"}
	for _, lockName := range locks {
		acquired, err := locker.AcquireLock(ctx, lockName, 5)
		if err != nil {
			t.Fatalf("Failed to acquire lock %s: %v", lockName, err)
		}
		if !acquired {
			t.Fatalf("Expected to acquire lock %s", lockName)
		}
	}

	// Release all locks
	for _, lockName := range locks {
		released, err := locker.ReleaseLock(ctx, lockName)
		if err != nil {
			t.Fatalf("Failed to release lock %s: %v", lockName, err)
		}
		if !released {
			t.Fatalf("Expected to release lock %s", lockName)
		}
	}
}
