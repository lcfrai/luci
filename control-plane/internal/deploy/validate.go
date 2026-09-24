package deploy

// ValidCommit reports whether c is 40 lowercase hex — the one commit form phase 7 ever sends.
func ValidCommit(c string) bool {
	if len(c) != 40 {
		return false
	}
	for i := 0; i < len(c); i++ {
		b := c[i]
		if (b < '0' || b > '9') && (b < 'a' || b > 'f') {
			return false
		}
	}
	return true
}

// ParsePostureMode maps a word to one of the five modes. An unknown or empty word returns
// (PostureKPMGSSO, false): fail closed to the safe default, and say it was unknown. This is
// close to the target's canonicalAccessMode, not the same: Panacea maps an unknown mode to
// "none" when ssoEnforced == "false" (access_mode.go:75-77). The domain has no ssoEnforced input
// and never opens a branch on an unknown word. The difference is harmless, because Panacea's
// GET already returns a canonical mode. It never maps an unknown or empty word to
// PosturePublic; only "none" does. The match is exact: no case folding, no trimming.
func ParsePostureMode(s string) (PostureMode, bool) {
	switch m := PostureMode(s); m {
	case PosturePanaceaSSO, PostureKPMGSSO, PosturePassword, PostureBypassToken, PosturePublic:
		return m, true
	}
	return PostureKPMGSSO, false
}
