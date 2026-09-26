package migrate_test

import (
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/migrate"
)

// TestEmbeddedMigrationsAreLF: a migration's sha256 is of its exact bytes, so a
// checkout that turned LF into CRLF would give one commit two digests, and a
// database migrated by a Linux build would be ErrEdited to a Windows one.
// sql/.gitattributes pins LF; this holds the build to it.
func TestEmbeddedMigrationsAreLF(t *testing.T) {
	ms := migrate.Embedded()
	if len(ms) == 0 || migrate.Latest() != ms[len(ms)-1].Version {
		t.Fatalf("Embedded holds %d migrations, Latest %d", len(ms), migrate.Latest())
	}
	for _, m := range ms {
		if strings.Contains(m.SQL, "\r") {
			t.Errorf("%04d_%s.sql holds a CR: its digest depends on the checkout's line endings", m.Version, m.Name)
		}
	}
}
