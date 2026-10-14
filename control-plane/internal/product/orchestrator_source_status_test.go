package product

import (
	"strings"
	"testing"
)

func TestOrchestratorSourceStatusRequiresOneCurrentGitIdentity(t *testing.T) {
	head := strings.Repeat("a", 40)
	for _, test := range []struct {
		name, output string
		known, dirty bool
		commit       string
	}{
		{"clean", "# branch.oid " + head + "\n# branch.head main\n", true, false, head},
		{"tracked-edit", "# branch.oid " + head + "\n# branch.head main\n1 .M N... 100644 100644 100644 " + head + " " + head + " file.txt\n", true, true, head},
		{"untracked", "# branch.oid " + head + "\n# branch.head main\n? new.txt\n", true, true, head},
		{"unborn", "# branch.oid (initial)\n# branch.head main\n", true, false, ""},
		{"missing-identity", "# branch.head main\n", false, false, ""},
		{"duplicate-identity", "# branch.oid " + head + "\n# branch.oid " + head + "\n", false, false, ""},
		{"malformed-identity", "# branch.oid (unknown)\n", false, false, ""},
		{"unknown-record", "# branch.oid " + head + "\nforeign result\n", false, false, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			known, dirty, commit := orchestratorSourceStatus(test.output)
			if known != test.known || dirty != test.dirty || commit != test.commit {
				t.Fatalf("source snapshot known=%t dirty=%t commit=%q", known, dirty, commit)
			}
		})
	}
}
