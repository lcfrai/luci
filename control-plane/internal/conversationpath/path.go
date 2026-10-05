// Package conversationpath gives private agent homes compact physical names.
// Docker Desktop cannot reliably bind deeply nested Windows host directories.
// Logical conversation IDs remain unchanged in the session registry.
package conversationpath

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"path/filepath"
)

func name(identity ...string) string {
	data, _ := json.Marshal(identity)
	sum := sha256.Sum256(data)
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

// UserRoot stays inside the gateway's protected state and separates accounts.
func UserRoot(gatewayDir, userID string) string {
	return filepath.Join(gatewayDir, "c", name("user-v1", userID))
}

// NativeRelative includes every component of the legacy identity in its name.
func NativeRelative(owner, worktree, provider, conversation string) string {
	return filepath.Join("n", name("native-v1", owner, worktree, provider, conversation))
}

func LegacyRelative(owner, worktree, provider, conversation string) string {
	return filepath.Join("native", owner, worktree, provider, conversation)
}
