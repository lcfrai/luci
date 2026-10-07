package previewgateway

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

const BridgeProbePath = "/.well-known/luci-preview-gateway"
const BridgeNonceHeader = "X-Luci-Preview-Nonce"
const BridgeProofHeader = "X-Luci-Preview-Proof"

func NewBridgeKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// BridgeProof returns a challenge response, not the private key itself. The
// random nonce ensures an unrelated app reusing a stopped bridge port cannot
// replay or forge the ownership receipt.
func BridgeProof(key, nonce string) string {
	if !capability.MatchString(key) || !capability.MatchString(nonce) {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(nonce))
	return hex.EncodeToString(mac.Sum(nil))
}
func verifyBridge(ctx context.Context, t Target) error {
	nonce, err := NewBridgeKey()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", t.Upstream+BridgeProbePath, nil)
	if err != nil {
		return err
	}
	req.Header.Set(BridgeNonceHeader, nonce)
	transport := &http.Transport{Proxy: nil, ResponseHeaderTimeout: 2 * time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	expected := BridgeProof(t.BridgeKey, nonce)
	provided := res.Header.Get(BridgeProofHeader)
	if res.StatusCode != 204 || expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		return errors.New("owned preview bridge changed or stopped")
	}
	return nil
}
