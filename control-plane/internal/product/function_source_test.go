package product

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/platform"
)

func TestFunctionSourceEditsRetainFilesAndRefuseStaleOrPrivatePaths(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "function-source")
	ctx := context.Background()
	callbacks := s.FunctionSources()
	initial := platform.DataFunctionSave{AuthMode: "authenticated", Files: []platform.DataFunctionFile{{Path: "index.ts", Content: "export default {fetch:()=>Response.json({ok:true})};\n"}}}
	v, err := callbacks.Save(ctx, p.ID, "example", initial)
	if err != nil || v.SourceDigest == "" || !v.WorkingChanged || v.AuthMode != "authenticated" {
		t.Fatalf("new source: %+v %v", v, err)
	}
	patch := platform.DataFunctionSave{ExpectedDigest: v.SourceDigest, AuthMode: "project-key", Files: []platform.DataFunctionFile{{Path: "lib/value.ts", Content: "export const value=42;\n"}}}
	v, err = callbacks.Save(ctx, p.ID, "example", patch)
	if err != nil || len(v.Files) != 2 || v.AuthMode != "project-key" {
		t.Fatalf("patch must retain entry and change committed auth policy: %+v %v", v, err)
	}
	file := filepath.Join(p.Path, "supabase", "functions", "example", "index.ts")
	if err = os.WriteFile(file, []byte("// agent changed this file\nexport default {fetch:()=>new Response('changed')};"), 0600); err != nil {
		t.Fatal(err)
	}
	patch.ExpectedDigest = v.SourceDigest
	if _, err = callbacks.Save(ctx, p.ID, "example", patch); err == nil {
		t.Fatal("external source edit overwritten")
	}
	b, _ := os.ReadFile(file)
	if !strings.HasPrefix(string(b), "// agent changed") {
		t.Fatal("stale save altered external edit")
	}
	for _, path := range []string{"../outside.ts", "/absolute.ts", "private/credentials.ts", ".env", "node_modules/a.ts", "index.TS", "luci.function.json", "INDEX.ts"} {
		in := initial
		in.Files = append(in.Files, platform.DataFunctionFile{Path: path, Content: "export const nope=1;"})
		if _, err = callbacks.Save(ctx, p.ID, "blocked", in); err == nil {
			t.Fatalf("private, ambiguous or unsupported path accepted: %q", path)
		}
	}
	large := initial
	large.Files = append([]platform.DataFunctionFile(nil), initial.Files...)
	large.Files[0].Content = strings.Repeat("x", platform.DataFunctionMaxFileBytes+1)
	if _, err = callbacks.Save(ctx, p.ID, "large", large); err == nil {
		t.Fatal("oversize source accepted")
	}
	if _, err = callbacks.Save(ctx, p.ID, "hello", initial); err == nil {
		t.Fatal("built-in source overwritten")
	}
	initial.AuthMode = "off"
	if _, err = callbacks.Save(ctx, p.ID, "public", initial); err == nil {
		t.Fatal("JWT verification disabled")
	}
	if _, err = callbacks.Read(ctx, "foreign", "example"); err == nil {
		t.Fatal("foreign project read")
	}
	// A linked file cannot disclose an unrelated private path through the editor.
	private := filepath.Join(t.TempDir(), "outside.ts")
	if err = os.WriteFile(private, []byte("private fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(p.Path, "supabase", "functions", "example", "linked.ts")
	if err = os.Link(private, linked); err != nil {
		t.Fatal("hard-link fixture unavailable", err)
	}
	if _, err = callbacks.Read(ctx, p.ID, "example"); err == nil {
		t.Fatal("linked private file admitted")
	}
}

func TestFunctionPublicationUsesCleanExactCommitAndApprovedContext(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "committed-function")
	ctx := context.Background()
	callbacks := s.FunctionSources()
	source := "export default {fetch:()=>Response.json({version:1})};\n"
	files := []platform.DataFunctionFile{{Path: "index.ts", Content: source}}
	for n := 1; n < platform.DataFunctionMaxFiles; n++ {
		files = append(files, platform.DataFunctionFile{Path: "z" + strconv.Itoa(n) + ".ts", Content: "export const value=" + strconv.Itoa(n) + ";\n"})
	}
	if _, err := callbacks.Save(ctx, p.ID, "example", platform.DataFunctionSave{AuthMode: "project-key", Files: files}); err != nil {
		t.Fatal(err)
	}
	base := "/v1/product/projects/" + p.ID
	spec := specificationFromBrief(Brief{ProjectID: p.ID, Purpose: "Invoke saved functions", Audience: "Application users"})
	spec.Requirements = []SpecificationRequirement{{ID: "AC-001", Title: "Saved function", Priority: "must", Acceptance: "A published function returns its committed version"}}
	journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": 0, "expectedBriefRevision": 0, "specification": spec}, 200)
	journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": 1, "briefRevision": 1}, 200)
	var commit map[string]string
	if err := json.Unmarshal(journeyRequest(t, mux, "POST", base+"/git/commit", map[string]string{"message": "Commit reviewed function"}, 200).Body.Bytes(), &commit); err != nil {
		t.Fatal(err)
	}
	sha := commit["commit"]
	if _, err := callbacks.Committed(ctx, p.ID, "example", sha); err == nil {
		t.Fatal("function source was published without a reviewed design handoff")
	}
	reviewDeploymentFixture(t, mux, p.ID)
	if _, err := callbacks.Committed(ctx, p.ID, "example", sha); err == nil {
		t.Fatal("function source was published before committing the reviewed handoff")
	}
	if err := json.Unmarshal(journeyRequest(t, mux, "POST", base+"/git/commit", map[string]string{"message": "Commit approved function design"}, 200).Body.Bytes(), &commit); err != nil {
		t.Fatal(err)
	}
	sha = commit["commit"]
	actual, err := callbacks.Committed(ctx, p.ID, "example", sha)
	if err != nil || actual.Commit != sha || actual.AuthMode != "project-key" || actual.Files[0].Content != source || len(actual.Files) != platform.DataFunctionMaxFiles {
		t.Fatalf("actual clean commit not admitted: %+v %v", actual, err)
	}
	working, err := callbacks.Read(ctx, p.ID, "example")
	if err != nil || working.WorkingChanged || working.CommittedDigest != actual.SourceDigest {
		t.Fatalf("clean source status wrong: %+v %v", working, err)
	}
	if err = os.WriteFile(filepath.Join(p.Path, "supabase", "functions", "example", "index.ts"), []byte("export default {fetch:()=>Response.json({version:2})};\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = callbacks.Committed(ctx, p.ID, "example", sha); err == nil {
		t.Fatal("dirty working tree was published")
	}
	retained, err := s.functionCommitBytes(ctx, p.ID, "example", sha)
	if err != nil || retained.Files[0].Content != source || retained.SourceDigest != actual.SourceDigest {
		t.Fatal("exact committed bytes changed with working tree", err)
	}
	working, err = callbacks.Read(ctx, p.ID, "example")
	if err != nil || !working.WorkingChanged || working.CommittedDigest != actual.SourceDigest {
		t.Fatal("changed source was not reported", err)
	}
	if _, err = callbacks.Committed(ctx, p.ID, "example", strings.Repeat("a", 40)); err == nil {
		t.Fatal("foreign source revision admitted")
	}
}
