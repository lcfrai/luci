package product

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestSkillMetadataPreservesLegacyIdentityAndExactCapturedRevision(t *testing.T) {
	legacy := ProjectSkillVersion{ID: "legacy", Revision: 1, Title: "Legacy", Provenance: "team", Roles: []string{"coding"}, Instructions: "Keep work focused."}
	raw := `{"id":"legacy","revision":1,"title":"Legacy","provenance":"team","roles":["coding"],"instructions":"Keep work focused.","digest":""}`
	sum := sha256.Sum256([]byte(raw))
	if skillVersionDigest(legacy) != hex.EncodeToString(sum[:]) {
		t.Fatal("optional metadata changed old immutable identity")
	}
	encoded, _ := json.Marshal(legacy)
	if string(encoded) != raw {
		t.Fatal("legacy encoding changed")
	}
	state, _ := taskBriefFixture()
	draft := skillDraft("source-aware", "Use the approved design tokens.")
	draft.Metadata = &ProjectSkillMetadata{License: "MIT", SourceURL: "https://example.com/skills/source#usage", SourceRevision: "v2.1", DeclaredTools: []string{"git", "mcp:glasses.search"}}
	saved, err := applySkillsSave(state, ProjectSkillsSave{Skill: draft, Assignments: []ProjectSkillAssignment{{Role: "coding", TaskID: "save", SkillID: draft.ID, SkillRevision: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	original := saved.Versions[0].Digest
	draft.Metadata.DeclaredTools[0] = "changed"
	if saved.Versions[0].Metadata.DeclaredTools[0] != "git" {
		t.Fatal("request memory mutates saved version")
	}
	state.Skills = &saved
	selected, err := selectedSkills(state, []string{"save"})
	if err != nil || len(selected) != 1 || selected[0].Metadata.SourceRevision != "v2.1" || selected[0].Digest != original {
		t.Fatal("task lost exact metadata", err)
	}
	catalogue, err := orchestratorSkillsContext(state)
	if err != nil || len(catalogue.Versions) != 1 || catalogue.Versions[0].Metadata.License != "MIT" {
		t.Fatal("orchestrator lost metadata", err)
	}
	selected[0].Metadata.DeclaredTools[0] = "other"
	catalogue.Versions[0].Metadata.DeclaredTools[0] = "another"
	if saved.Versions[0].Metadata.DeclaredTools[0] != "git" {
		t.Fatal("context changed immutable metadata")
	}
	tampered := currentSkills(state)
	tampered.Versions[0].Metadata.License = "different"
	if validateSkillRegistry(tampered) == nil {
		t.Fatal("changed metadata retained old digest")
	}
}

func TestSkillMetadataRejectsMalformedAttributionAndToolDeclarations(t *testing.T) {
	for _, metadata := range []*ProjectSkillMetadata{
		{SourceURL: "javascript:alert(1)"}, {SourceURL: "https://user:password@example.com/skill"},
		{License: strings.Repeat("x", 129)}, {SourceRevision: "revision\ncommand"}, {License: "MIT\u202e"},
		{DeclaredTools: []string{"git", "git"}}, {DeclaredTools: []string{"shell;rm"}}, {DeclaredTools: []string{""}},
	} {
		if validProjectSkillMetadata(metadata) {
			t.Fatalf("invalid metadata accepted: %#v", metadata)
		}
	}
	if !validProjectSkillMetadata(nil) || !validProjectSkillMetadata(&ProjectSkillMetadata{License: "unknown", DeclaredTools: []string{"git", "node", "mcp:glasses.search"}}) {
		t.Fatal("valid explicit attribution refused")
	}
}
