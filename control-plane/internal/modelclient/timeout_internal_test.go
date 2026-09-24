package modelclient

import (
	"testing"
	"time"
)

// TestTheDefaultTimeoutIsBelowTheLongestDesktopBudget: the HTTP ceiling must
// sit below the desktop's longest Discover wait (task 15's 60 s for analyse),
// or a slow model reaches the desktop as its own timeout instead of the
// server's typed degrade (I4). The old plan's 120 s failed that. This file is
// package-internal so it can read the built http.Client's Timeout rather than
// wait 45 s for it.
func TestTheDefaultTimeoutIsBelowTheLongestDesktopBudget(t *testing.T) {
	const longestDesktopBudget = 60 * time.Second
	if DefaultTimeout >= longestDesktopBudget {
		t.Fatalf("DefaultTimeout = %v, want below %v", DefaultTimeout, longestDesktopBudget)
	}

	c, err := New(Config{APIKey: "k", BaseURL: "http://x"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.http.Timeout != DefaultTimeout {
		t.Errorf("http.Client.Timeout = %v, want DefaultTimeout %v", c.http.Timeout, DefaultTimeout)
	}
}
