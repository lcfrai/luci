package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// sessionExecutionGeneration is an opaque display identity, not an attach ticket
// or a registry migration. Mutable status/output and private endpoint/ref are
// deliberately excluded. Restart mints a new SandboxID and AttachToken. Terminal
// cleanup erases AttachToken, so callers must treat omission as unknown, not as a
// new allocation. Legacy records lacking a current 256-bit token are omitted.
func sessionExecutionGeneration(s registry.Session) string {
	if s.ID == "" || s.SandboxID == "" || len(s.AttachToken) != 43 {
		return ""
	}
	key, err := base64.RawURLEncoding.Strict().DecodeString(s.AttachToken)
	if err != nil || len(key) != 32 {
		return ""
	}
	payload, _ := json.Marshal([]string{"luci.session-execution-generation.v1", s.ID, s.SandboxID})
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
