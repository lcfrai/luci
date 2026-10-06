package registry

import (
	"path/filepath"
	"testing"
)

func TestProjectRunChildBindingSurvivesRegistryRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store := newStore(t, path)
	sess := sampleSession()
	sess.ProjectRunID, sess.ProjectRunOperationID = "run-001", "op-001"
	if err := store.Create(sess); err != nil {
		t.Fatal(err)
	}
	restored, err := newStore(t, path).Get(sess.ID)
	if err != nil || restored.ProjectRunID != sess.ProjectRunID || restored.ProjectRunOperationID != sess.ProjectRunOperationID {
		t.Fatal("private registry lost the reserved run child binding", err)
	}
}
