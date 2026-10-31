package assistance

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPersonalJobOwnershipSurvivesSerializationAndCannotDowngrade(t *testing.T) {
	files := map[string][]byte{}
	store := NewStore(func(id string) ([]byte, error) {
		raw, ok := files[id]
		if !ok {
			return nil, os.ErrNotExist
		}
		return append([]byte(nil), raw...), nil
	}, func(id string, raw []byte) error { files[id] = append([]byte(nil), raw...); return nil })
	job := Job{ID: "assist-personal", Kind: "personal-luci", PersonalOwner: strings.Repeat("a", 64), PersonalTeamID: "team-one", PersonalTurnID: "personal-turn-one", PersonalContext: json.RawMessage(`{"version":1}`), RequestedBy: "alice", ProjectID: "project-one", Status: "queued", CreatedAt: "2026-10-31T00:00:00Z"}
	if err := store.Create(job); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Job){func(j *Job) { j.PersonalOwner = "" }, func(j *Job) { j.Kind = "orchestrator" }, func(j *Job) { j.RequestedBy = "bob" }, func(j *Job) { j.PersonalContext[0] = '[' }} {
		if _, err := store.Update(job.ID, func(j *Job) error { change(j); return nil }); err == nil {
			t.Fatal("private job was downgraded or its captured scope rewritten")
		}
	}
	retained, err := store.Read(job.ID)
	if err != nil || !IsPersonal(retained) || retained.PersonalOwner != job.PersonalOwner {
		t.Fatal("owner not durable", retained, err)
	}
	retained.PersonalOwner, retained.PersonalTeamID, retained.PersonalTurnID, retained.PersonalContext = "", "", "", nil
	if !IsPersonal(retained) {
		t.Fatal("older/missing new fields downgraded the fixed personal kind")
	}
	if IsPersonal(Job{Kind: "orchestrator"}) {
		t.Fatal("old generic jobs changed meaning")
	}
}
