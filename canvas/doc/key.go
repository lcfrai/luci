package doc

import (
	"errors"
	"strconv"
	"strings"
)

// ErrBadKey is every rejection of a design key.
var ErrBadKey = errors.New("doc: not a design key")

const (
	designKeyPrefix = "design:"
	// MaxDesignRevision is the Hub's grammar bound (contract §3.2).
	MaxDesignRevision = 999_999_999
)

// DesignKey is "design:<canvasID>@<revision>", the one form the provenance
// ledger stores (plan G9). It errs unless ValidCanvasID and
// 1 ≤ revision ≤ MaxDesignRevision: revision 0 is a canvas nothing has
// changed yet, which nothing may cite.
func DesignKey(canvasID string, revision int) (string, error) {
	if !ValidCanvasID(canvasID) || revision < 1 || revision > MaxDesignRevision {
		return "", ErrBadKey
	}
	return designKeyPrefix + canvasID + "@" + strconv.Itoa(revision), nil
}

// ParseDesignKey inverts DesignKey exactly; anything else is ErrBadKey. The
// revision is decimal with no sign, no spaces and no leading zero, because
// the Hub's grammar has one spelling per revision and a key is compared as a
// string.
func ParseDesignKey(key string) (canvasID string, revision int, err error) {
	rest, ok := strings.CutPrefix(key, designKeyPrefix)
	if !ok {
		return "", 0, ErrBadKey
	}
	id, revText, ok := strings.Cut(rest, "@")
	if !ok || !ValidCanvasID(id) {
		return "", 0, ErrBadKey
	}
	if revText == "" || (len(revText) > 1 && revText[0] == '0') {
		return "", 0, ErrBadKey
	}
	for i := 0; i < len(revText); i++ {
		if revText[i] < '0' || revText[i] > '9' {
			return "", 0, ErrBadKey
		}
	}
	rev, convErr := strconv.Atoi(revText)
	if convErr != nil || rev < 1 || rev > MaxDesignRevision {
		return "", 0, ErrBadKey
	}
	return id, rev, nil
}
