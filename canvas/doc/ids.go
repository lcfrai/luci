package doc

import (
	"crypto/rand"
	"encoding/hex"
)

const (
	canvasIDPrefix = "cv-"
	canvasIDHex    = 16 // the Hub contract §3.2 width: minted wide so ids stay unique when v2 moves them
	idHex          = 12 // every other id in this package
	assetRefPrefix = "asset:sha256-"
	assetRefHex    = 64
)

// randomHex is n bytes of crypto/rand as 2n lowercase hex digits.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand does not fail on any platform this ships to; a panic
		// is the only honest answer to an id that cannot be made unique.
		panic("doc: crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// NewCanvasID is "cv-" + 16 lowercase hex from crypto/rand.
func NewCanvasID() string { return canvasIDPrefix + randomHex(canvasIDHex/2) }

// NewID is prefix + "-" + 12 lowercase hex from crypto/rand. Prefixes in use:
// "pg" page, "n" node, "th" thread, "msg" message, "rv" review request.
func NewID(prefix string) string { return prefix + "-" + randomHex(idHex/2) }

// lowerHex reports whether s is exactly n lowercase hex digits.
func lowerHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for i := 0; i < n; i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ValidCanvasID reports whether s matches ^cv-[0-9a-f]{16}$.
func ValidCanvasID(s string) bool {
	return len(s) == len(canvasIDPrefix)+canvasIDHex &&
		s[:len(canvasIDPrefix)] == canvasIDPrefix &&
		lowerHex(s[len(canvasIDPrefix):], canvasIDHex)
}

// ValidID reports whether s matches ^<prefix>-[0-9a-f]{12}$.
func ValidID(prefix, s string) bool {
	if prefix == "" {
		return false
	}
	head := prefix + "-"
	return len(s) == len(head)+idHex && s[:len(head)] == head && lowerHex(s[len(head):], idHex)
}

// AssetRef is "asset:sha256-" + lowercase hex of sum.
func AssetRef(sum [32]byte) string { return assetRefPrefix + hex.EncodeToString(sum[:]) }

// ParseAssetRef inverts AssetRef exactly; uppercase hex is not a ref.
func ParseAssetRef(ref string) (sum [32]byte, ok bool) {
	if !ValidAssetRef(ref) {
		return sum, false
	}
	b, err := hex.DecodeString(ref[len(assetRefPrefix):])
	if err != nil {
		return sum, false
	}
	copy(sum[:], b)
	return sum, true
}

// ValidAssetRef reports whether ref is a well-formed asset reference.
func ValidAssetRef(ref string) bool {
	return len(ref) == len(assetRefPrefix)+assetRefHex &&
		ref[:len(assetRefPrefix)] == assetRefPrefix &&
		lowerHex(ref[len(assetRefPrefix):], assetRefHex)
}
