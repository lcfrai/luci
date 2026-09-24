package hubapi

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

// ── ids (contract §3.1) ─────────────────────────────────────────────────────

func TestValidIDTakesOnlyItsOwnPrefix(t *testing.T) {
	hex16 := "00000000000000a1"
	cases := []struct {
		id   string
		want bool
	}{
		{"bf-" + hex16, true},
		{"pj-" + hex16, false},                 // another entity's prefix
		{"bf-" + hex16[1:], false},             // 15 hex
		{"bf-" + hex16 + "0", false},           // 17 hex
		{"bf-00000000000000A1", false},         // uppercase hex
		{"bf-" + hex16 + "\n", false},          // a trailing newline
		{"xbf-" + hex16, false},                // the prefix found by Contains, not at the start
		{"bf-" + hex16 + "bf-" + hex16, false}, // unanchored at the end
		{"bf" + hex16, false},                  // no dash
	}
	for _, c := range cases {
		if got := ValidID(PrefixBrief, c.id); got != c.want {
			t.Errorf("ValidID(%q, %q) = %v, want %v", PrefixBrief, c.id, got, c.want)
		}
	}
}

// ── brief keys (contract §3.2) ──────────────────────────────────────────────

func TestBriefKeyRoundTrips(t *testing.T) {
	const in = "brief:bf-3f2a91c07b1e4d2a@5"
	k, err := ParseBriefKey(in)
	if err != nil {
		t.Fatalf("ParseBriefKey(%q): %v", in, err)
	}
	if want := (BriefKey{BriefID: "bf-3f2a91c07b1e4d2a", Revision: 5}); k != want {
		t.Fatalf("ParseBriefKey(%q) = %+v, want %+v", in, k, want)
	}
	if got := k.String(); got != in {
		t.Fatalf("BriefKey.String() = %q, want the input %q back", got, in)
	}
	// The largest revision the grammar admits, nine digits, round-trips too.
	const max = "brief:bf-3f2a91c07b1e4d2a@999999999"
	if k, err := ParseBriefKey(max); err != nil || k.Revision != 999_999_999 || k.String() != max {
		t.Fatalf("ParseBriefKey(%q) = %+v, %v; want revision 999999999 and the input back", max, k, err)
	}
}

func TestBriefKeyRejectsEverythingOutsideTheGrammar(t *testing.T) {
	const id = "bf-3f2a91c07b1e4d2a"
	for _, s := range []string{
		"brief:" + id + "@0",          // revision 0 never appears in a key
		"brief:" + id + "@01",         // a leading zero: two spellings of one key
		"brief:" + id + "@1000000000", // ten digits
		"brief:bf-3f2a91c07b1e4d2@1",  // 15 hex
		"brief:bf-3F2A91C07B1E4D2A@1", // uppercase hex
		"brief:b-42@1",                // a local id
		" brief:" + id + "@1",         // a leading space
		"brief:" + id + "@1\n",        // a trailing newline
		"brief:" + id + "@",           // no revision
		"brief:" + id,                 // no @
		"design:cv-7c01d2e3f4a5b6c7@1",
		"",
	} {
		if k, err := ParseBriefKey(s); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ParseBriefKey(%q) = %+v, %v; want ErrInvalidKey", s, k, err)
		}
	}
}

// ── design keys (contract §3.2) ─────────────────────────────────────────────

func TestDesignKeyGrammar(t *testing.T) {
	const in = "design:cv-7c01d2e3f4a5b6c7@11"
	k, err := ParseDesignKey(in)
	if err != nil {
		t.Fatalf("ParseDesignKey(%q): %v", in, err)
	}
	if want := (DesignKey{CanvasID: "cv-7c01d2e3f4a5b6c7", Revision: 11}); k != want {
		t.Fatalf("ParseDesignKey(%q) = %+v, want %+v", in, k, want)
	}
	if got := k.String(); got != in {
		t.Fatalf("DesignKey.String() = %q, want the input %q back", got, in)
	}
	// The bounds themselves are in: 2 and 4 letters, 4 and 32 hex.
	for _, s := range []string{
		"design:cv-abcd@1",
		"design:cvxy-abcd@1",
		"design:cv-" + strings.Repeat("a", 32) + "@1",
	} {
		if _, err := ParseDesignKey(s); err != nil {
			t.Errorf("ParseDesignKey(%q): %v, want it accepted (a bound of the grammar)", s, err)
		}
	}
	for _, s := range []string{
		"design:cv-abc@1", // 3 hex
		"design:cv-" + strings.Repeat("a", 33) + "@1", // 33 hex
		"design:c-7c01d2e3f4a5b6c7@1",                 // one letter
		"design:cvxyz-7c01d2e3f4a5b6c7@1",             // five letters
		"design:cv-7c01d2e3f4a5b6c7@0",                // revision 0
		"design:cv-7c01d2e3f4a5b6c7@011",              // a leading zero
		"design:cv-7c01d2e3f4a5b6c7@11\n",             // a trailing newline
		"brief:bf-3f2a91c07b1e4d2a@5",
	} {
		if k, err := ParseDesignKey(s); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("ParseDesignKey(%q) = %+v, %v; want ErrInvalidKey", s, k, err)
		}
	}
}

func TestParseKeyClassifies(t *testing.T) {
	cases := []struct {
		in   string
		kind KeyKind
		err  error
	}{
		{"brief:bf-3f2a91c07b1e4d2a@5", KeyBrief, nil},
		{"design:cv-7c01d2e3f4a5b6c7@11", KeyDesign, nil},
		{"b-42", "", ErrInvalidKey},
		{"", "", ErrInvalidKey},
	}
	for _, c := range cases {
		kind, err := ParseKey(c.in)
		if kind != c.kind || !errors.Is(err, c.err) || (c.err == nil) != (err == nil) {
			t.Errorf("ParseKey(%q) = (%q, %v), want (%q, %v)", c.in, kind, err, c.kind, c.err)
		}
	}
}

// ── digests ─────────────────────────────────────────────────────────────────

const abcDigest = "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"

func TestDigestWireForm(t *testing.T) {
	d := DigestOf([]byte("abc"))
	if got := d.String(); got != abcDigest {
		t.Fatalf("DigestOf(abc).String() = %q, want %q", got, abcDigest)
	}
	if got := d.Hex(); got != abcDigest[len("sha256:"):] {
		t.Fatalf("Hex() = %q, want the bare 64 hex", got)
	}
	p, err := ParseDigest(abcDigest)
	if err != nil || p != d {
		t.Fatalf("ParseDigest(%q) = %x, %v; want the same digest back", abcDigest, p, err)
	}

	// JSON carries the string form, never base64 or a byte array.
	type wire struct {
		D Digest `json:"digest"`
	}
	b, err := json.Marshal(wire{D: d})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if want := `{"digest":"` + abcDigest + `"}`; string(b) != want {
		t.Fatalf("json.Marshal = %s, want %s", b, want)
	}
	var back wire
	if err := json.Unmarshal(b, &back); err != nil || back.D != d {
		t.Fatalf("json.Unmarshal(%s) = %x, %v; want the digest back", b, back.D, err)
	}

	hex64 := abcDigest[len("sha256:"):]
	for _, s := range []string{
		"SHA256:" + hex64,                  // the prefix case-folded
		"sha256:" + strings.ToUpper(hex64), // uppercase hex
		"sha256:" + hex64[:63],             // 63 hex
		"sha256:" + hex64 + "0",            // 65 hex
		hex64,                              // the bare form is the blob route's, not the wire's
		"sha256:" + hex64 + "\n",           // a trailing newline
		"sha512:" + hex64,                  // another algorithm
		"",
	} {
		if _, err := ParseDigest(s); !errors.Is(err, ErrInvalidDigest) {
			t.Errorf("ParseDigest(%q): %v, want ErrInvalidDigest", s, err)
		}
		// The JSON path refuses the same, with the same error.
		var w wire
		if err := json.Unmarshal([]byte(`{"digest":`+strconv.Quote(s)+`}`), &w); !errors.Is(err, ErrInvalidDigest) {
			t.Errorf("json.Unmarshal of %q: %v, want ErrInvalidDigest", s, err)
		}
	}
}

func TestParseDigestHexTakesTheBareForm(t *testing.T) {
	hex64 := abcDigest[len("sha256:"):]
	d, err := ParseDigestHex(hex64)
	if err != nil || d != DigestOf([]byte("abc")) {
		t.Fatalf("ParseDigestHex(%q) = %x, %v; want the digest of abc", hex64, d, err)
	}
	for _, s := range []string{abcDigest, strings.ToUpper(hex64), hex64[:63], ""} {
		if _, err := ParseDigestHex(s); !errors.Is(err, ErrInvalidDigest) {
			t.Errorf("ParseDigestHex(%q): %v, want ErrInvalidDigest (a blob route's {sha} is the bare 64 hex)", s, err)
		}
	}
}

// ── branch refs (contract §3.3) ─────────────────────────────────────────────

func TestBranchRefRules(t *testing.T) {
	ref255 := "refs/heads/" + strings.Repeat("a", BranchRefMaxBytes-len("refs/heads/"))
	valid := []string{
		"", // project-scoped
		"refs/heads/main",
		"refs/heads/feat/evidence-upload",
		"refs/heads/a.b",      // a dot inside a component
		"refs/heads/lock",     // "lock" is fine; ".lock" at a component's end is not
		"refs/heads/a.locked", // so is a component that only contains ".lock"
		ref255,
	}
	for _, r := range valid {
		if !ValidBranchRef(r) {
			t.Errorf("ValidBranchRef(%q) = false, want true", r)
		}
	}
	invalid := []string{
		"main",         // not a full ref
		"b-3f2a91c0",   // a local BranchID, which never reaches the Hub
		"refs/heads/",  // no name
		"refs/tags/v1", // not a branch
		ref255 + "a",   // 256 bytes
		"refs/heads/a..b",
		"refs/heads/a@{b",
		"refs/heads/a//b",
		"refs/heads/a/",
		"refs/heads/a.",
		"refs/heads/a.lock",
		"refs/heads/a.lock/b", // git refuses ".lock" at the end of ANY component
		"refs/heads/.hidden",
		"refs/heads/a/.b",
		"refs/heads/a b",
		"refs/heads/a~b",
		"refs/heads/a^b",
		"refs/heads/a:b",
		"refs/heads/a?b",
		"refs/heads/a*b",
		"refs/heads/a[b",
		`refs/heads/a\b`,
		"refs/heads/a\x01b",
		"refs/heads/a\x7fb",
		"refs/heads/main\n",
	}
	for _, r := range invalid {
		if ValidBranchRef(r) {
			t.Errorf("ValidBranchRef(%q) = true, want false", r)
		}
	}
}

// ── repository keys (contract §7.4, plan correction A1) ─────────────────────

func TestRepoKeyTakesPhase3sForms(t *testing.T) {
	cases := []struct {
		host GitHost
		key  string
	}{
		{HostGitHub, "github.com/lcfrai/panaceaapp"},
		// Azure DevOps project names may contain spaces; the contract's old
		// "<org>.visualstudio.com/…" regex refused every key of this form.
		{HostAzureDevOps, "azure-devops/kpmgcomau/panacea app/panaceaapp"},
		{HostGitHub, "github.com/a-b/c.d_e-f"},
	}
	for _, c := range cases {
		if !ValidRepoKey(c.host, c.key) {
			t.Errorf("ValidRepoKey(%q, %q) = false, want true", c.host, c.key)
		}
		if h, ok := HostOfRepoKey(c.key); !ok || h != c.host {
			t.Errorf("HostOfRepoKey(%q) = (%q, %v), want (%q, true)", c.key, h, ok, c.host)
		}
	}
}

func TestRepoKeyRefusesTheOldAndTheNonCanonical(t *testing.T) {
	refused := []string{
		"kpmgcomau.visualstudio.com/p/r",                // the contract's old form
		"dev.azure.com/kpmgcomau/p/r",                   // the URL authority, not the key
		"github.com/LCFRAI/x",                           // not lowercase: refused, never folded
		"azure-devops/kpmgcomau/Panacea App/panaceaapp", // the same, on the other host
		"azure-devops/a/b",                              // three segments
		"azure-devops/a/b/c/d",                          // five
		"github.com/a/b/c",                              // a path, not a repository
		"github.com/a/b.git/",                           // a trailing slash
		"github.com/a",                                  // no repository
		"azure-devops/a/b/c\n",                          // a trailing newline: a second spelling
		"azure-devops/a/b\tb/c",                         // a control character inside a segment
		"azure-devops/a/b/c\x7f",                        // DEL
		"azure-devops/a/\xff/c",                         // not UTF-8
		" github.com/a/b",                               // a leading space
		"",
	}
	for _, k := range refused {
		for _, h := range []GitHost{HostGitHub, HostAzureDevOps} {
			if ValidRepoKey(h, k) {
				t.Errorf("ValidRepoKey(%q, %q) = true, want false", h, k)
			}
		}
		if h, ok := HostOfRepoKey(k); ok {
			t.Errorf("HostOfRepoKey(%q) = (%q, true), want no host", k, h)
		}
	}
	// A valid key under the wrong host, and an unknown host, are refused.
	if ValidRepoKey(HostAzureDevOps, "github.com/lcfrai/panaceaapp") {
		t.Error("a GitHub key validated as Azure DevOps")
	}
	if ValidRepoKey(HostGitHub, "azure-devops/kpmgcomau/p/r") {
		t.Error("an Azure DevOps key validated as GitHub")
	}
	if ValidRepoKey("gitlab", "github.com/lcfrai/panaceaapp") {
		t.Error("a host outside the two validated a key")
	}
}

const hubContract = "docs/superpowers/contracts/2026-09-23-hub-contract.md"

// repoRoot walks up from the test's directory to the directory holding the Hub
// contract, so the test reads the file every sibling phase reads.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(hubContract))); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no %s above the test's directory", hubContract)
		}
		dir = parent
	}
}

// The contract's §7.4 and this package say the same thing about repository
// keys, or a sibling phase builds against a regex the Hub does not run. That
// drift is what correction A1 had to fix once.
func TestTheContractsRepoKeyRegexIsTheCodes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(hubContract)))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	start := strings.Index(text, "\n### 7.4 ")
	if start < 0 {
		t.Fatal("the contract has no §7.4")
	}
	section := text[start+1:]
	if end := strings.Index(section, "\n### "); end >= 0 {
		section = section[:end]
	}
	found := regexp.MustCompile("`(\\^[^`]*\\$)`").FindAllStringSubmatch(section, -1)
	byHost := map[GitHost][]string{}
	for _, m := range found {
		switch {
		case strings.HasPrefix(m[1], `^github\.com/`):
			byHost[HostGitHub] = append(byHost[HostGitHub], m[1])
		case strings.HasPrefix(m[1], `^azure-devops/`):
			byHost[HostAzureDevOps] = append(byHost[HostAzureDevOps], m[1])
		default:
			t.Errorf("§7.4 carries the pattern %s, which is neither host's key form", m[1])
		}
	}
	for host, code := range map[GitHost]string{HostGitHub: githubKeyRe.String(), HostAzureDevOps: adoKeyRe.String()} {
		got := byHost[host]
		if len(got) != 1 || got[0] != code {
			t.Errorf("§7.4's %s repo-key pattern(s) = %q, want exactly the code's %s", host, got, code)
		}
	}
	// The lowercase rule is the other half of A1: stated, not only coded.
	if !strings.Contains(section, "strings.ToLower") {
		t.Error("§7.4 does not state the lowercase rule (repoKey == strings.ToLower(repoKey))")
	}
}

// ── codes (contract §4.2) ───────────────────────────────────────────────────

// httpx's sixteen codes (control-plane/internal/httpx/problem.go:45-60), listed
// by name so a seventeenth there is a decision here too.
var controlPlaneCodes = map[string]string{
	"CodeInvalidJSON":        httpx.CodeInvalidJSON,
	"CodeBadRequest":         httpx.CodeBadRequest,
	"CodeUnknownHarness":     httpx.CodeUnknownHarness,
	"CodeUnsupportedOption":  httpx.CodeUnsupportedOption,
	"CodeForbiddenOrigin":    httpx.CodeForbiddenOrigin,
	"CodeForbiddenHost":      httpx.CodeForbiddenHost,
	"CodeProjectNotFound":    httpx.CodeProjectNotFound,
	"CodeWorktreeNotFound":   httpx.CodeWorktreeNotFound,
	"CodeSessionNotFound":    httpx.CodeSessionNotFound,
	"CodeConflict":           httpx.CodeConflict,
	"CodeSandboxNotRunning":  httpx.CodeSandboxNotRunning,
	"CodeHarnessUnavailable": httpx.CodeHarnessUnavailable,
	"CodeUpgradeRequired":    httpx.CodeUpgradeRequired,
	"CodeInternal":           httpx.CodeInternal,
	"CodeNonLoopbackPublish": httpx.CodeNonLoopbackPublish,
	"CodeBackendUnavailable": httpx.CodeBackendUnavailable,
}

// The desktop switches on a code without knowing which process sent it: a Hub
// 503 spelled backend_unavailable would read as "Docker is not running"
// (desktop/src/api/problems.ts:39). So every Hub code is either one of the five
// shared ones, imported from httpx by name, or a string httpx does not use; the
// one deliberate reuse is project_not_found (codes.go says why).
//
// The test reads codes.go's source, not a list of its own, so a code added
// there is checked without anyone remembering to add it here.
func TestHubCodesNeverShadowAControlPlaneCode(t *testing.T) {
	if len(controlPlaneCodes) != 16 {
		t.Fatalf("the list holds %d httpx codes, want problem.go's sixteen", len(controlPlaneCodes))
	}
	shared := map[string]bool{"CodeInvalidJSON": true, "CodeBadRequest": true,
		"CodeForbiddenOrigin": true, "CodeConflict": true, "CodeInternal": true}

	f, err := parser.ParseFile(token.NewFileSet(), "codes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	own := map[string]string{} // value -> constant name
	sharedSeen := 0
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if !strings.HasPrefix(name.Name, "Code") {
					continue
				}
				switch v := vs.Values[i].(type) {
				case *ast.SelectorExpr: // httpx.CodeX: must be a shared code, by its own name
					if x, _ := v.X.(*ast.Ident); x == nil || x.Name != "httpx" || v.Sel.Name != name.Name || !shared[name.Name] {
						t.Errorf("%s imports httpx.%s; only the five shared codes are imported, each by its own name", name.Name, v.Sel.Name)
					}
					sharedSeen++
				case *ast.BasicLit:
					s, err := strconv.Unquote(v.Value)
					if err != nil {
						t.Fatalf("%s: %v", name.Name, err)
					}
					if prev, dup := own[s]; dup {
						t.Errorf("%s and %s both spell %q", prev, name.Name, s)
					}
					own[s] = name.Name
					for cpName, cp := range controlPlaneCodes {
						if s == cp && !(name.Name == "CodeProjectNotFound" && cpName == "CodeProjectNotFound") {
							t.Errorf("hubapi.%s = %q shadows httpx.%s", name.Name, s, cpName)
						}
					}
				default:
					t.Errorf("%s is neither a string literal nor an httpx constant", name.Name)
				}
			}
		}
	}
	if sharedSeen != len(shared) {
		t.Errorf("codes.go imports %d shared codes, want the five", sharedSeen)
	}
	// Contract §4.2's table: 38 codes, the five shared plus the Hub's 33. The
	// desktop's HubProblemCode (task 38) is the 33 without project_not_found.
	if len(own) != 33 {
		t.Errorf("codes.go spells %d Hub codes, want contract §4.2's 33", len(own))
	}
	if own[httpx.CodeProjectNotFound] != "CodeProjectNotFound" || CodeProjectNotFound != httpx.CodeProjectNotFound {
		t.Errorf("project_not_found is not spelled as httpx spells it")
	}
}
