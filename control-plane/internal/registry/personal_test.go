package registry

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonalSessionStaysPrivateAcrossReopenAndMissingOwnerField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	row := sampleSession()
	row.PersonalOwner, row.ContextRef = strings.Repeat("a", 64), "luci:personal"
	if err := store.Create(row); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Update(row.ID, func(current *Session) error { current.PersonalOwner, current.ContextRef = "", "ordinary"; return nil }); err == nil {
		t.Fatal("public downgrade permitted")
	}
	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(row.ID)
	if err != nil || !loaded.IsPersonalAssistant() || loaded.PersonalOwner != row.PersonalOwner {
		t.Fatal("owner lost on reopen", err)
	}
	raw, _ := json.Marshal(loaded)
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		t.Fatal("invalid fixture")
	}
	delete(fields, "personalOwner")
	raw, _ = json.Marshal(fields)
	var older Session
	if json.Unmarshal(raw, &older) != nil || !older.IsPersonalAssistant() {
		t.Fatal("missing newer field downgraded fixed private context")
	}
	if sampleSession().IsPersonalAssistant() {
		t.Fatal("legacy generic session changed")
	}
}
