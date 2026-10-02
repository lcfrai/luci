package api_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/api"
)

// CABundle is optional, so empty is fine and only relative is wrong — for the
// reason a relative ConversationsDir is: it becomes a bind mount's host path,
// and Docker's refusal would name neither this field nor the variable.
func TestNewRefusesARelativeCABundleAndAcceptsNoneOrAnAbsoluteOne(t *testing.T) {
	d, _ := catalogNewDeps(t)
	d.CABundle = "corp-root.pem"
	if a, err := api.New(d); err == nil || !strings.Contains(err.Error(), "CABundle") || !strings.Contains(err.Error(), "is not absolute") {
		t.Fatalf("New with a relative CABundle = %v, %v; want a refusal naming CABundle", a, err)
	}
	for _, ok := range []string{"", filepath.Join(t.TempDir(), "corp-root.pem")} {
		d, _ := catalogNewDeps(t)
		d.CABundle = ok
		if _, err := api.New(d); err != nil {
			t.Errorf("New with CABundle %q: %v", ok, err)
		}
	}
}
