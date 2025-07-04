package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// HashCommand generates a deterministic lock name from a command
// The format is "mylock-<hash>" where hash is the SHA256 of the joined command
// The result is truncated to 64 characters to fit MySQL's lock name limit
func HashCommand(command []string) string {
	// Join the command with null bytes to avoid ambiguity
	// e.g., ["echo", "hello world"] vs ["echo hello", "world"]
	joined := strings.Join(command, "\x00")
	
	// Calculate SHA256 hash
	hash := sha256.Sum256([]byte(joined))
	hashStr := hex.EncodeToString(hash[:])
	
	// Prefix with "mylock-" and truncate to 64 chars if needed
	lockName := "mylock-" + hashStr
	if len(lockName) > 64 {
		lockName = lockName[:64]
	}
	
	return lockName
}