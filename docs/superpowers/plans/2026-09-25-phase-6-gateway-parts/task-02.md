### Task 2: Model catalogue and the generated LiteLLM config

Slice A, 2.5 focused hours, no dependencies (it imports task 01's `dataclass.Route`, which lands in the same wave). No seams.

This task fixes **which models exist and where each one runs**, once, as data. Every other part asks the catalogue and never spells a model string: the gatekeeper rewrites `model` through `Resolve` (task 11), keysmith scopes a key to `Groups()` (task 07), the gatekeeper answers `GET /v1/models` from `ModelsResponse()` (task 12), and LiteLLM's own `config.yaml` is **generated** from it (this task), never hand-edited. A model added to `models.yaml` reaches all four in one commit, and a hand edit to a generated file is a red test.

Three rules are enforced here, not described:

1. **The AU route never leaves Australia.** Every AU upstream is a Bedrock `au.` inference profile in `ap-southeast-2`. An `apac.` or `global.` profile routes across regions by design, so the loader refuses it, and so does a region other than `ap-southeast-2` (05-design's CI rule, `docs/superpowers/planning/2026-09-24-team-collaboration/05-design.md:224-227`).
2. **No bare-host `api_base`** (the Panacea 405, skeleton "What Panacea's LiteLLM taught"). The cloud Global route renders **no** `api_base` at all, so LiteLLM uses its own default for the Anthropic provider. Where an `api_base` is rendered (local mode's fakes, or an operator override), its form is a recorded fact, `full` (ends in `/v1/messages`) until task 18's P07 measures otherwise, and flipping it to a bare host needs a named piece of evidence.
3. **Zero data retention in our own config** (P6-D9). No callback of any kind, prompt storage in spend logs off, message logging off, every credential an `os.environ/` reference and never a value.

Six decisions the skeleton leaves open, made here so no later part re-derives them:

- **(a) YAML through `sigs.k8s.io/yaml` v1.4.0, promoted from `// indirect` to a direct `require`.** It is already in the module graph through `client-go` (`control-plane/go.mod:68`, `control-plane/go.sum:196-197`), so the promotion adds no module and no `go.sum` line. It decodes YAML into the same `json` tags the rest of the code uses, and `UnmarshalStrict` refuses an unknown key, so a misspelt `verifed:` in `models.yaml` is a load error, not a silently unverified model. Global constraint 1 names `pgx/v5` as the only planned dependency; this is listed as a skeleton gap in the return.
- **(b) The generated YAML is rendered from Go structs** by `yaml.Marshal`, which goes through `encoding/json` and so sorts every map's keys. The output is byte-for-byte deterministic without a template, which is what makes the drift test honest.
- **(c) Three targets, not two: `local`, `local-anthropic`, `cloud`.** Task 17 offers `PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=anthropic` (real Global spend from a developer's key), and a single local config pointing at the fakes cannot serve it. `local-anthropic` renders the Global group exactly as `cloud` does (no `api_base`, `os.environ/ANTHROPIC_API_KEY`) and the AU group at the `au` fake, because there is no Bedrock locally. Its output is `deploy/local/gateway/litellm.anthropic.config.yaml`, a third generated file the skeleton's file map does not list (skeleton gap; task 17 picks the file by the variable).
- **(d) The AU route's AWS credential is not in the config.** The cloud render gives the AU group only `aws_region_name: ap-southeast-2`. The role is assumed through the standard AWS web-identity chain, `AWS_ROLE_ARN` plus `AWS_WEB_IDENTITY_TOKEN_FILE` set on the LiteLLM pod, with a projected service-account token of audience `sts.amazonaws.com` (task 20's trust). That LiteLLM's Bedrock client falls back to boto3's default chain when no `aws_*` credential parameter is given is **training knowledge, UNVERIFIED** until task 24's P04; it keeps every LiteLLM-specific credential parameter name out of this file. Task 19 must set the two variables and the projected token (skeleton gap: task 19's scope names neither).
- **(e) Aliases.** Claude Code may send a dated id (`claude-haiku-4-5-20251001`) as well as the undated one. An entry may list `aliases`; `Resolve` accepts them, `ModelsResponse` lists only the canonical id, and every id and alias is unique across the file. No id or alias may contain `/`, so a LiteLLM group name (`global/claude-opus-4-8`) can never resolve as a public id (task 11 relies on that).
- **(f) Every group carries its own price, and a price of zero does not load** (review I-5). LiteLLM prices a call from its built-in cost table, and an id the pinned table lacks (`claude-opus-5`, `claude-sonnet-5`, every `bedrock/au.anthropic.*` profile, and every id at a local fake) is priced at US$0. Spend then stays 0, and all three OD-24 budgets are silently off. So each model carries `price: {global: …, au: …}`, four figures per route in US dollars per million tokens (`input`, `output`, `cacheRead`, `cacheWrite`), and `gatewaygen` renders them per token into each group's `model_info` on every target, in fixed notation (`0.000005`): `yaml.Marshal` writes `5e-06`, which PyYAML, the YAML 1.1 loader LiteLLM reads its config with, resolves to a string, so `fixedCosts` rewrites each price and a test holds every one to PyYAML's float pattern. `Parse` refuses a route whose figure is missing, zero, or outside US$0.01–1,000 per million tokens; the floor catches a per-token figure typed into a per-million field, which would price near zero and fail open just the same. **Every price is UNVERIFIED.** Global is Anthropic's list price as cached on 2026-06-24 (the `claude-api` reference). Cache read is 0.1× input and cache write 1.25× input, the usual ratios, also unverified. AU uses the same figures until an operator compares them with the AWS price list for `ap-southeast-2` and raises any that is lower (task 24's "does not claim" list; its P06 proves only that every group has a price above zero). An unverified price may err high, which makes a budget bite early; never low. The `model_info` key names are LiteLLM's from training knowledge, **UNVERIFIED** until task 18 shows spend above zero (P03–P05, P12).

**The model ids are not claimed current.** `claude-opus-4-8` is the docs' own example (https://code.claude.com/docs/en/llm-gateway-protocol, read 2026-09-25, per the skeleton). The other ids come from Anthropic's model notes as cached on 2026-06-24 (step 4), and **every Bedrock profile id is UNVERIFIED** and carries `verified: false` until task 24 lists the account's `au.` profiles. The generator writes the unverified list into each output's header, so an operator reading the deployed config sees it.

**Files:**
- `control-plane/internal/inference/catalogue/catalogue.go` — `Model`, `Upstream`, `APIBaseForm`, `Catalogue`; `Load`, `Parse`, `Resolve`, `Groups`, `Public`, `ModelsResponse`, `Unverified`, `CheckAPIBase` (new)
- `control-plane/internal/inference/catalogue/models.yaml` — the catalogue, embedded with `go:embed` (new)
- `control-plane/internal/inference/catalogue/catalogue_test.go` (new)
- `control-plane/cmd/gatewaygen/main.go` — `-target local|local-anthropic|cloud -out <file> [-api-base-form full|host] [-global-api-base <url>]` (new)
- `control-plane/cmd/gatewaygen/main_test.go` (new)
- `deploy/local/gateway/litellm.config.yaml`, `deploy/local/gateway/litellm.anthropic.config.yaml`, `deploy/charts/gateway/files/litellm.config.yaml` — generated, checked in (new)
- `control-plane/go.mod` — `sigs.k8s.io/yaml v1.4.0` moves to the direct `require` block (modified)

**Interfaces:**
- Produces:
  - `catalogue.Model{ID, DisplayName string; Aliases []string; CreatedAt string; Price Prices; Global, AU Upstream}`, `catalogue.Upstream{Model, Region string; Verified bool}`, `catalogue.Prices{Global, AU Cost}`, `catalogue.Cost{InputPerMTok, OutputPerMTok, CacheReadPerMTok, CacheWritePerMTok float64}`, US dollars per million tokens (decision (f))
  - `catalogue.APIBaseForm` with `FormFull` (`"full"`) and `FormHost` (`"host"`); `catalogue.CheckAPIBase(form APIBaseForm, raw string) error`
  - `catalogue.Load() (*Catalogue, error)` (the embedded file), `catalogue.Parse([]byte) (*Catalogue, error)`
  - `(*Catalogue) Resolve(publicID string, r dataclass.Route) (group string, ok bool)`: `global/<id>` or `au/<id>`, `ok=false` for an unknown id, a group name, or a route that is not valid
  - `(*Catalogue) Groups() []string` (every `global/*` and `au/*` group, sorted, for task 07's key scope), `Public() []Model`, `ModelsResponse() []byte`, `Unverified() []string`, `LiteLLM() LiteLLMFacts`
  - `catalogue.GroupPrefixGlobal` (`"global/"`), `catalogue.GroupPrefixAU` (`"au/"`)
- Consumes: `dataclass.Route`, `RouteGlobal`, `RouteAU`, `(Route) Valid()` (task 01); `sigs.k8s.io/yaml`.
- Required by: task 07 (`Groups`), task 11 (`Resolve`, through its `Resolver` interface), task 12 (`ModelsResponse`), task 17 (the two local outputs), task 18 (renders both `api_base` forms for P07, then sets `apiBaseForm`), task 19 (the chart output), task 20 (the `au.` profile ids), task 24 (lists the account's profiles and flips `verified`).

- [ ] **Step 1: Write the failing catalogue tests**

Every test names, in its comment, the mutant it kills. The fixture is a small catalogue in a string, so the tests do not change when `models.yaml` gains a model; one test loads the embedded file to prove it passes the same rules.

Create `control-plane/internal/inference/catalogue/catalogue_test.go`:

```go
package catalogue

import (
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
)

// A Bedrock id carries a ":" ("…-v1:0"), so inside a flow mapping it is
// quoted: YAML 1.1 parsers disagree on a bare ":" in flow context.
const fixture = `
litellm:
  apiBaseForm: full
models:
  - id: claude-opus-4-8
    displayName: Claude Opus 4.8
    createdAt: "2026-01-01T00:00:00Z"
    price:
      global: {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25}
      au: {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25}
    global: {model: anthropic/claude-opus-4-8, verified: true}
    au: {model: "bedrock/au.anthropic.claude-opus-4-8-v1:0", region: ap-southeast-2, verified: false}
  - id: claude-haiku-4-5
    aliases: [claude-haiku-4-5-20251001]
    displayName: Claude Haiku 4.5
    createdAt: "2025-10-01T00:00:00Z"
    price:
      global: {input: 1, output: 5, cacheRead: 0.1, cacheWrite: 1.25}
      au: {input: 1, output: 5.5, cacheRead: 0.1, cacheWrite: 1.25}
    global: {model: anthropic/claude-haiku-4-5, verified: true}
    au: {model: "bedrock/au.anthropic.claude-haiku-4-5-20251001-v1:0", region: ap-southeast-2, verified: false}
`

func mustParse(t *testing.T, y string) *Catalogue {
	t.Helper()
	c, err := Parse([]byte(y))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return c
}

// swap returns the fixture with one substring replaced, failing if it is absent,
// so a test can never pass because its edit silently missed.
func swap(t *testing.T, old, new string) string {
	t.Helper()
	if !strings.Contains(fixture, old) {
		t.Fatalf("fixture has no %q", old)
	}
	return strings.Replace(fixture, old, new, 1)
}

// Kills: an embedded models.yaml that breaks a rule the fixture obeys.
func TestTheEmbeddedCatalogueObeysEveryRule(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if g, ok := c.Resolve("claude-opus-4-8", dataclass.RouteAU); !ok || g != "au/claude-opus-4-8" {
		t.Fatalf("the docs' own example id resolves to %q, %v", g, ok)
	}
}

// Kills: Resolve ignoring the route (always global/), and any default that turns
// an unparsed route into Global (P6-D5).
func TestResolve(t *testing.T) {
	c := mustParse(t, fixture)
	cases := []struct {
		id     string
		r      dataclass.Route
		want   string
		wantOK bool
	}{
		{"claude-opus-4-8", dataclass.RouteGlobal, "global/claude-opus-4-8", true},
		{"claude-opus-4-8", dataclass.RouteAU, "au/claude-opus-4-8", true},
		{"claude-haiku-4-5-20251001", dataclass.RouteAU, "au/claude-haiku-4-5", true}, // alias → canonical group
		{"global/claude-opus-4-8", dataclass.RouteGlobal, "", false},                  // a group is not a public id (task 11)
		{"au/claude-opus-4-8", dataclass.RouteAU, "", false},
		{"claude-unreleased-9", dataclass.RouteGlobal, "", false},
		{"claude-opus-4-8", dataclass.Route(""), "", false},
		{"claude-opus-4-8", dataclass.Route("eu"), "", false},
		{"", dataclass.RouteGlobal, "", false},
	}
	for _, tc := range cases {
		got, ok := c.Resolve(tc.id, tc.r)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("Resolve(%q, %q) = %q, %v; want %q, %v", tc.id, tc.r, got, ok, tc.want, tc.wantOK)
		}
	}
}
```

Then the loader's refusals and the `api_base` rule, in the same file:

```go
// Kills: accepting an apac. or global. profile (cross-region by design), any
// region but Sydney, or a Global upstream that is not the Anthropic provider.
func TestParseRefusesARouteThatLeavesItsLane(t *testing.T) {
	au := func(model, region string) string { return `au: {model: "` + model + `", region: ` + region }
	opusAU := au("bedrock/au.anthropic.claude-opus-4-8-v1:0", "ap-southeast-2")
	opusGlobal := "global: {model: anthropic/claude-opus-4-8, verified: true}"
	cases := map[string][2]string{
		"apac profile":         {opusAU, au("bedrock/apac.anthropic.claude-opus-4-8-v1:0", "ap-southeast-2")},
		"global profile":       {opusAU, au("bedrock/global.anthropic.claude-opus-4-8-v1:0", "ap-southeast-2")},
		"us profile":           {opusAU, au("bedrock/us.anthropic.claude-opus-4-8-v1:0", "ap-southeast-2")},
		"bare foundation id":   {opusAU, au("bedrock/anthropic.claude-opus-4-8-v1:0", "ap-southeast-2")},
		"Melbourne region":     {opusAU, au("bedrock/au.anthropic.claude-opus-4-8-v1:0", "ap-southeast-4")},
		"no region":            {opusAU + ",", `au: {model: "bedrock/au.anthropic.claude-opus-4-8-v1:0",`},
		"AU through Anthropic": {opusAU, au("anthropic/claude-opus-4-8", "ap-southeast-2")},
		"Global not Anthropic": {opusGlobal, `global: {model: "bedrock/au.anthropic.claude-opus-4-8-v1:0", verified: true}`},
		"Global with a region": {opusGlobal, "global: {model: anthropic/claude-opus-4-8, region: us-east-1, verified: true}"},
	}
	for name, e := range cases {
		if _, err := Parse([]byte(swap(t, e[0], e[1]))); err == nil {
			t.Errorf("%s: Parse accepted it", name)
		}
	}
}

// Kills: a duplicate id or alias (Resolve would pick one silently), an id with
// "/" (a group name could then resolve), a misspelt key (strict decoding), a
// missing field, and a bare-host api_base form adopted without evidence.
func TestParseRefusesAnAmbiguousOrMisspeltCatalogue(t *testing.T) {
	cases := map[string][2]string{
		"alias repeats an id":    {"aliases: [claude-haiku-4-5-20251001]", "aliases: [claude-opus-4-8]"},
		"id repeats":             {"- id: claude-haiku-4-5\n", "- id: claude-opus-4-8\n"},
		"slash in id":            {"- id: claude-haiku-4-5\n", "- id: vendor/claude-haiku-4-5\n"},
		"not served under ZDR":   {"- id: claude-haiku-4-5\n", "- id: claude-fable-5-1\n"},
		"misspelt key":           {"region: ap-southeast-2, verified: false}\n  - id", "region: ap-southeast-2, verifed: false}\n  - id"},
		"empty display name":     {"displayName: Claude Haiku 4.5", `displayName: ""`},
		"bad createdAt":          {`createdAt: "2025-10-01T00:00:00Z"`, `createdAt: "October"`},
		"host form, no evidence": {"apiBaseForm: full", "apiBaseForm: host"},
		"unknown form":           {"apiBaseForm: full", "apiBaseForm: relative"},
	}
	for name, e := range cases {
		if _, err := Parse([]byte(swap(t, e[0], e[1]))); err == nil {
			t.Errorf("%s: Parse accepted it", name)
		}
	}
	if _, err := Parse([]byte("litellm: {apiBaseForm: full}\nmodels: []\n")); err == nil {
		t.Error("an empty catalogue was accepted")
	}
	evidenced := swap(t, "apiBaseForm: full", "apiBaseForm: host\n  apiBaseEvidence: docs/verification/2026-09-25-phase-6-litellm-probe.md#p07")
	if _, err := Parse([]byte(evidenced)); err != nil {
		t.Errorf("host form with evidence refused: %v", err)
	}
}

// Review I-5, decision (f). Kills: a group that loads unpriced (LiteLLM's own
// table then charges US$0 and every budget is off), a zero or negative figure,
// a per-token figure typed into a per-million field, and checking Global only.
func TestParseRefusesAnUnpricedModel(t *testing.T) {
	haikuGlobal := "global: {input: 1, output: 5, cacheRead: 0.1, cacheWrite: 1.25}"
	haikuAU := "au: {input: 1, output: 5.5, cacheRead: 0.1, cacheWrite: 1.25}"
	cases := map[string][2]string{
		"no price":          {"    price:\n      " + haikuGlobal + "\n      " + haikuAU + "\n", ""},
		"no AU price":       {"\n      " + haikuAU, ""},
		"zero output":       {haikuGlobal, "global: {input: 1, output: 0, cacheRead: 0.1, cacheWrite: 1.25}"},
		"no AU cache read":  {haikuAU, "au: {input: 1, output: 5.5, cacheWrite: 1.25}"},
		"negative AU input": {haikuAU, "au: {input: -1, output: 5.5, cacheRead: 0.1, cacheWrite: 1.25}"},
		"per token":         {haikuGlobal, "global: {input: 0.000001, output: 0.000005, cacheRead: 0.0000001, cacheWrite: 0.00000125}"},
		"misspelt figure":   {haikuAU, "au: {input: 1, ouput: 5.5, cacheRead: 0.1, cacheWrite: 1.25}"},
	}
	for name, e := range cases {
		if _, err := Parse([]byte(swap(t, e[0], e[1]))); err == nil {
			t.Errorf("%s: Parse accepted it", name)
		}
	}
	if got := mustParse(t, fixture).Public()[1].Price.AU.OutputPerMTok; got != 5.5 {
		t.Errorf("haiku's AU output price = %v, want the au figure 5.5", got)
	}
}

// Kills: the Panacea bare-host 405 (full form accepting a bare host) and the
// double path (host form accepting a URL that already ends in /v1/messages).
func TestCheckAPIBase(t *testing.T) {
	cases := []struct {
		form APIBaseForm
		raw  string
		ok   bool
	}{
		{FormFull, "http://fakeanthropic-global:4900/v1/messages", true},
		{FormFull, "https://api.anthropic.com/v1/messages", true},
		{FormFull, "http://fakeanthropic-global:4900", false}, // the Panacea 405
		{FormFull, "http://fakeanthropic-global:4900/", false},
		{FormFull, "http://fakeanthropic-global:4900/v1/messages/", false},
		{FormFull, "http://fakeanthropic-global:4900/v1/messages?beta=true", false},
		{FormHost, "http://fakeanthropic-global:4900", true},
		{FormHost, "http://fakeanthropic-global:4900/v1/messages", false}, // would double-path
		{FormFull, "ftp://fakeanthropic-global:4900/v1/messages", false},
		{FormFull, "", false},
		{APIBaseForm("relative"), "http://h/v1/messages", false},
	}
	for _, tc := range cases {
		if err := CheckAPIBase(tc.form, tc.raw); (err == nil) != tc.ok {
			t.Errorf("CheckAPIBase(%q, %q) = %v, want ok=%v", tc.form, tc.raw, err, tc.ok)
		}
	}
}
```

And the three outputs other parts read, in the same file:

```go
// Kills: listing aliases or LiteLLM group names in the picker, a field renamed
// away from Anthropic's list shape, and has_more true (the CLI would page).
func TestModelsResponseIsTheAnthropicListShape(t *testing.T) {
	c := mustParse(t, fixture)
	want := `{"data":[` +
		`{"type":"model","id":"claude-opus-4-8","display_name":"Claude Opus 4.8","created_at":"2026-01-01T00:00:00Z"},` +
		`{"type":"model","id":"claude-haiku-4-5","display_name":"Claude Haiku 4.5","created_at":"2025-10-01T00:00:00Z"}],` +
		`"has_more":false,"first_id":"claude-opus-4-8","last_id":"claude-haiku-4-5"}`
	if got := string(c.ModelsResponse()); got != want {
		t.Fatalf("ModelsResponse:\n got %s\nwant %s", got, want)
	}
	b := c.ModelsResponse()
	b[0] = 'X'
	if c.ModelsResponse()[0] != '{' {
		t.Fatal("ModelsResponse returned its own buffer; a caller's write corrupted it")
	}
}

// Kills: a key scoped to one route only (a raise to AU mid-key would then fail,
// P6-D4), and an unsorted list (task 07's request golden would flap).
func TestGroupsAreEveryModelOnBothRoutes(t *testing.T) {
	got := strings.Join(mustParse(t, fixture).Groups(), ",")
	want := "au/claude-haiku-4-5,au/claude-opus-4-8,global/claude-haiku-4-5,global/claude-opus-4-8"
	if got != want {
		t.Fatalf("Groups = %s, want %s", got, want)
	}
}

// Kills: an unverified Bedrock profile id deployed with nothing saying so.
func TestUnverifiedNamesEveryUnverifiedUpstream(t *testing.T) {
	got := strings.Join(mustParse(t, fixture).Unverified(), "|")
	want := "au/claude-haiku-4-5 = bedrock/au.anthropic.claude-haiku-4-5-20251001-v1:0|" +
		"au/claude-opus-4-8 = bedrock/au.anthropic.claude-opus-4-8-v1:0"
	if got != want {
		t.Fatalf("Unverified = %s, want %s", got, want)
	}
}

// Kills: Public handing out the catalogue's own slice.
func TestPublicReturnsACopy(t *testing.T) {
	c := mustParse(t, fixture)
	p := c.Public()
	p[0].ID = "mutated"
	p[0].Aliases = append(p[0].Aliases, "x")
	if c.Public()[0].ID != "claude-opus-4-8" || len(c.Public()[0].Aliases) != 0 {
		t.Fatal("a caller's edit reached the catalogue")
	}
}
```

- [ ] **Step 2: Run the catalogue tests to verify they fail**

Run: `go test ./control-plane/internal/inference/catalogue/... -count=1`

Expected: the build fails, because nothing in the package exists yet:

```
# github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue [github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue.test]
control-plane/internal/inference/catalogue/catalogue_test.go:30:12: undefined: Catalogue
control-plane/internal/inference/catalogue/catalogue_test.go:32:12: undefined: Parse
...
FAIL	github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue [build failed]
```

The pass condition is `[build failed]` with `undefined:` lines; the line numbers are indicative.

- [ ] **Step 3: Promote the YAML module**

In `control-plane/go.mod`, move `sigs.k8s.io/yaml v1.4.0` from the `// indirect` block (line 68) into the first `require` block, without the comment, keeping that block sorted:

```
require (
	github.com/coder/websocket v1.8.12
	golang.org/x/sys v0.24.0
	k8s.io/api v0.31.1
	k8s.io/apimachinery v0.31.1
	k8s.io/client-go v0.31.1
	sigs.k8s.io/yaml v1.4.0
)
```

Do not run `go mod tidy` yet: nothing imports the module until step 4, and tidy would move it straight back to `// indirect`. Step 5 runs it. If task 05 has already added `pgx/v5` to the same block, keep both lines, sorted.

- [ ] **Step 4: Write the package**

Create `control-plane/internal/inference/catalogue/catalogue.go`. It is written here in three fenced blocks, one file.

```go
// Package catalogue is the one list of models the gateway offers and where each
// runs on each route. Everything that names a model reads it here: the
// gatekeeper's rewrite (task 11), keysmith's key scope (task 07), the
// /v1/models answer (task 12) and LiteLLM's generated config (cmd/gatewaygen).
//
// The AU route never leaves Australia: an AU upstream is a Bedrock "au."
// inference profile in ap-southeast-2 or the file does not load.
package catalogue

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass"
)

//go:embed models.yaml
var embedded []byte

const (
	// GroupPrefixGlobal and GroupPrefixAU name LiteLLM's model groups (the
	// skeleton's "Shared names"). A sandbox only ever sends the public id.
	GroupPrefixGlobal = "global/"
	GroupPrefixAU     = "au/"

	// AURegion is the only region an AU upstream may name.
	AURegion = "ap-southeast-2"

	auProfilePrefix = "bedrock/au.anthropic." // "apac." and "global." route abroad
	globalProvider  = "anthropic/"
	messagesPath    = "/v1/messages"
)

// notUnderZDR are model families that require 30-day retention: a ZDR org's
// request to them is a 400 (Anthropic's model notes, cached 2026-06-24, read
// through the claude-api reference; UNVERIFIED against the org's agreement).
var notUnderZDR = []string{"claude-fable-", "claude-mythos-"}

// APIBaseForm is how LiteLLM's Anthropic provider treats api_base, a measured
// fact (task 18's P07) recorded in models.yaml.
type APIBaseForm string

const (
	FormFull APIBaseForm = "full" // api_base ends in /v1/messages; LiteLLM posts to it verbatim
	FormHost APIBaseForm = "host" // api_base is scheme://host[:port]; LiteLLM appends the path
)

// Upstream is one route's LiteLLM model string.
type Upstream struct {
	Model    string `json:"model"`
	Region   string `json:"region,omitempty"`
	Verified bool   `json:"verified"` // false until the id is seen in a real account
}

// Cost is one route's price in US dollars per million tokens (decision (f),
// review I-5). Every figure is UNVERIFIED until checked against a price list.
type Cost struct {
	InputPerMTok      float64 `json:"input"`
	OutputPerMTok     float64 `json:"output"`
	CacheReadPerMTok  float64 `json:"cacheRead"`
	CacheWritePerMTok float64 `json:"cacheWrite"`
}

// Prices is a model's price on each route; gatewaygen renders each into its
// group's model_info, so LiteLLM never falls back to its own cost table.
type Prices struct {
	Global Cost `json:"global"`
	AU     Cost `json:"au"`
}

// Model is one public model. ID is what Claude Code sends as "model".
type Model struct {
	ID          string   `json:"id"`
	Aliases     []string `json:"aliases,omitempty"`
	DisplayName string   `json:"displayName"`
	CreatedAt   string   `json:"createdAt"` // RFC 3339, for /v1/models
	Price       Prices   `json:"price"`
	Global      Upstream `json:"global"`
	AU          Upstream `json:"au"`
}

// LiteLLMFacts are what this repository has measured about the pinned LiteLLM.
type LiteLLMFacts struct {
	APIBaseForm     APIBaseForm `json:"apiBaseForm"`
	APIBaseEvidence string      `json:"apiBaseEvidence,omitempty"`
}

type document struct {
	LiteLLM LiteLLMFacts `json:"litellm"`
	Models  []Model      `json:"models"`
}

// Catalogue is immutable after Parse; every method is safe for concurrent use.
type Catalogue struct {
	facts  LiteLLMFacts
	models []Model
	byID   map[string]string // public id or alias → canonical id
	list   []byte            // the /v1/models body, rendered once
}
```

```go
// Load parses the embedded models.yaml.
func Load() (*Catalogue, error) { return Parse(embedded) }

// Parse decodes and validates a catalogue. An unknown or duplicated key is an
// error (UnmarshalStrict), so a misspelt "verified" never loads as unverified.
func Parse(data []byte) (*Catalogue, error) {
	var doc document
	if err := yaml.UnmarshalStrict(data, &doc); err != nil {
		return nil, fmt.Errorf("catalogue: %w", err)
	}
	if err := doc.LiteLLM.validate(); err != nil {
		return nil, fmt.Errorf("catalogue: litellm: %w", err)
	}
	if len(doc.Models) == 0 {
		return nil, errors.New("catalogue: no models")
	}
	c := &Catalogue{facts: doc.LiteLLM, byID: make(map[string]string)}
	for i, m := range doc.Models {
		if err := m.validate(); err != nil {
			return nil, fmt.Errorf("catalogue: models[%d] %q: %w", i, m.ID, err)
		}
		for _, id := range append([]string{m.ID}, m.Aliases...) {
			if _, dup := c.byID[id]; dup {
				return nil, fmt.Errorf("catalogue: id %q appears twice", id)
			}
			c.byID[id] = m.ID
		}
		c.models = append(c.models, m.clone())
	}
	list, err := c.renderList()
	if err != nil {
		return nil, fmt.Errorf("catalogue: /v1/models body: %w", err)
	}
	c.list = list
	return c, nil
}

func (f LiteLLMFacts) validate() error {
	switch f.APIBaseForm {
	case FormFull:
		return nil
	case FormHost:
		if strings.TrimSpace(f.APIBaseEvidence) == "" {
			return errors.New(`apiBaseForm "host" needs apiBaseEvidence naming the probe that measured it (task 18, P07)`)
		}
		return nil
	}
	return fmt.Errorf("apiBaseForm %q is neither %q nor %q", f.APIBaseForm, FormFull, FormHost)
}

func (m Model) validate() error {
	for _, id := range append([]string{m.ID}, m.Aliases...) {
		if id == "" || strings.ContainsAny(id, "/ \t\r\n") {
			return fmt.Errorf("id or alias %q is empty or holds a '/' or a space", id)
		}
		for _, p := range notUnderZDR {
			if strings.HasPrefix(id, p) {
				return fmt.Errorf("id %q names a model not served under zero data retention (OD-19)", id)
			}
		}
	}
	if strings.TrimSpace(m.DisplayName) == "" {
		return errors.New("displayName is empty")
	}
	if _, err := time.Parse(time.RFC3339, m.CreatedAt); err != nil {
		return fmt.Errorf("createdAt %q is not RFC 3339", m.CreatedAt)
	}
	if !strings.HasPrefix(m.Global.Model, globalProvider) || m.Global.Model == globalProvider {
		return fmt.Errorf("global model %q is not %s<id>", m.Global.Model, globalProvider)
	}
	if m.Global.Region != "" {
		return fmt.Errorf("global model names region %q; the Anthropic API has none", m.Global.Region)
	}
	if !strings.HasPrefix(m.AU.Model, auProfilePrefix) || m.AU.Model == auProfilePrefix {
		return fmt.Errorf("au model %q is not a Bedrock %s… inference profile", m.AU.Model, auProfilePrefix)
	}
	if m.AU.Region != AURegion {
		return fmt.Errorf("au region %q is not %s", m.AU.Region, AURegion)
	}
	if err := m.Price.Global.validate(); err != nil {
		return fmt.Errorf("global price: %w", err)
	}
	if err := m.Price.AU.validate(); err != nil {
		return fmt.Errorf("au price: %w", err)
	}
	return nil
}

// Price bounds, US dollars per million tokens. The floor refuses a per-token
// figure typed into a per-million field, which would price near zero and turn
// every budget off just as a missing price does (review I-5).
const minPerMTok, maxPerMTok = 0.01, 1000

func (c Cost) validate() error {
	for _, f := range []struct {
		name string
		v    float64
	}{{"input", c.InputPerMTok}, {"output", c.OutputPerMTok},
		{"cacheRead", c.CacheReadPerMTok}, {"cacheWrite", c.CacheWritePerMTok}} {
		if !(f.v >= minPerMTok && f.v <= maxPerMTok) { // also refuses NaN
			return fmt.Errorf("%s is %v; want US$%v to %v per million tokens", f.name, f.v, minPerMTok, maxPerMTok)
		}
	}
	return nil
}

func (m Model) clone() Model {
	m.Aliases = append([]string(nil), m.Aliases...)
	return m
}
```

```go
// CheckAPIBase is the Panacea 405 rule. In full form an api_base is
// http(s)://host[:port]/…/v1/messages with no query; in host form it has no
// path at all, because LiteLLM appends one. Anything else posts to a wrong path.
func CheckAPIBase(form APIBaseForm, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("api_base %q is not an http(s) URL with a host", raw)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("api_base %q carries a query or a fragment", raw)
	}
	switch form {
	case FormFull:
		if !strings.HasSuffix(u.Path, messagesPath) {
			return fmt.Errorf("api_base %q does not end in %s (a bare host answered 405 in Panacea)", raw, messagesPath)
		}
		return nil
	case FormHost:
		if u.Path != "" {
			return fmt.Errorf("api_base %q has a path; in host form LiteLLM appends %s itself", raw, messagesPath)
		}
		return nil
	}
	return fmt.Errorf("api_base form %q is neither %q nor %q", form, FormFull, FormHost)
}

// Resolve maps the id Claude Code sent to the LiteLLM group for route r. A
// group name, an unknown id or an unparsed route is not offered (P6-D5).
func (c *Catalogue) Resolve(publicID string, r dataclass.Route) (group string, ok bool) {
	id, ok := c.byID[publicID]
	if !ok {
		return "", false
	}
	switch r {
	case dataclass.RouteGlobal:
		return GroupPrefixGlobal + id, true
	case dataclass.RouteAU:
		return GroupPrefixAU + id, true
	}
	return "", false
}

// Groups is every group on both routes, sorted: a key is scoped to all of them
// so that a raise to AU mid-key (P6-D4) is served on the same key.
func (c *Catalogue) Groups() []string {
	out := make([]string, 0, 2*len(c.models))
	for _, m := range c.models {
		out = append(out, GroupPrefixGlobal+m.ID, GroupPrefixAU+m.ID)
	}
	sort.Strings(out)
	return out
}

// Public returns a copy of the models in file order.
func (c *Catalogue) Public() []Model {
	out := make([]Model, len(c.models))
	for i, m := range c.models {
		out[i] = m.clone()
	}
	return out
}

// ModelsResponse is the GET /v1/models body, a fresh copy on every call.
func (c *Catalogue) ModelsResponse() []byte { return append([]byte(nil), c.list...) }

// LiteLLM returns the measured facts about the pinned LiteLLM.
func (c *Catalogue) LiteLLM() LiteLLMFacts { return c.facts }

// Unverified names every upstream id not yet seen in a real account, sorted.
func (c *Catalogue) Unverified() []string {
	var out []string
	for _, m := range c.models {
		if !m.Global.Verified {
			out = append(out, GroupPrefixGlobal+m.ID+" = "+m.Global.Model)
		}
		if !m.AU.Verified {
			out = append(out, GroupPrefixAU+m.ID+" = "+m.AU.Model)
		}
	}
	sort.Strings(out)
	return out
}

// The Anthropic list shape (training knowledge, UNVERIFIED; task 23 watches
// the CLI's model picker). Field order is the golden test's.
type listEntry struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

type listPage struct {
	Data    []listEntry `json:"data"`
	HasMore bool        `json:"has_more"`
	FirstID string      `json:"first_id"`
	LastID  string      `json:"last_id"`
}

func (c *Catalogue) renderList() ([]byte, error) {
	p := listPage{FirstID: c.models[0].ID, LastID: c.models[len(c.models)-1].ID}
	for _, m := range c.models {
		p.Data = append(p.Data, listEntry{Type: "model", ID: m.ID, DisplayName: m.DisplayName, CreatedAt: m.CreatedAt})
	}
	return json.Marshal(p)
}
```

Create `control-plane/internal/inference/catalogue/models.yaml`. The ids are the current set in Anthropic's model notes as cached on 2026-06-24 (`claude-opus-5`, `claude-opus-4-8`, `claude-sonnet-5`, `claude-haiku-4-5`), read through the `claude-api` reference, not re-read at `docs.anthropic.com`. Claude Fable 5 and 5.1 are left out on purpose: those notes say they need 30-day retention and answer a ZDR org with a `400`, so under OD-19 they cannot be offered, and `notUnderZDR` refuses them. The `createdAt` dates are placeholders (**UNVERIFIED**) that only order the CLI's picker.

```yaml
# The gateway's model catalogue. Every other file that names a model reads
# this one; LiteLLM's config.yaml is generated from it (cmd/gatewaygen).
#
# Rules the loader enforces (catalogue.go): AU upstreams are Bedrock "au."
# profiles in ap-southeast-2 only; Global upstreams are anthropic/<id> with no
# region; ids are unique and never contain "/"; no ZDR-excluded family.
#
# verified: false = the id has not been seen in a real account. Every Bedrock
# profile below is UNVERIFIED until task 24 lists the account's au. profiles
# (`aws bedrock list-inference-profiles --region ap-southeast-2`).
#
# price: US dollars per million tokens, per route, rendered into each group's
# model_info so LiteLLM never prices a call at 0 from its own table (review
# I-5). Every figure is UNVERIFIED: Global is Anthropic's list price as cached
# 2026-06-24; cacheRead = 0.1 x input and cacheWrite = 1.25 x input; AU copies
# Global until an operator checks the AWS price list for ap-southeast-2 (task 24).
# Err high, never low: a high price makes a budget bite early.
litellm:
  # How the pinned LiteLLM's Anthropic provider treats api_base. "full" is the
  # Panacea lesson (a bare host answered 405). Task 18's P07 measures it; a
  # switch to "host" must name that evidence in apiBaseEvidence.
  apiBaseForm: full
models:
  - id: claude-opus-5
    displayName: Claude Opus 5
    createdAt: "2026-06-01T00:00:00Z"
    price:
      global: {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25}
      au: {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25}
    global: {model: anthropic/claude-opus-5, verified: false}
    au: {model: "bedrock/au.anthropic.claude-opus-5", region: ap-southeast-2, verified: false}
  - id: claude-opus-4-8
    displayName: Claude Opus 4.8
    createdAt: "2026-04-01T00:00:00Z"
    price:
      global: {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25}
      au: {input: 5, output: 25, cacheRead: 0.5, cacheWrite: 6.25}
    global: {model: anthropic/claude-opus-4-8, verified: false}
    au: {model: "bedrock/au.anthropic.claude-opus-4-8", region: ap-southeast-2, verified: false}
  - id: claude-sonnet-5
    displayName: Claude Sonnet 5
    createdAt: "2026-06-01T00:00:00Z"
    price:
      global: {input: 2, output: 10, cacheRead: 0.2, cacheWrite: 2.5}
      au: {input: 2, output: 10, cacheRead: 0.2, cacheWrite: 2.5}
    global: {model: anthropic/claude-sonnet-5, verified: false}
    au: {model: "bedrock/au.anthropic.claude-sonnet-5", region: ap-southeast-2, verified: false}
  - id: claude-haiku-4-5
    aliases: [claude-haiku-4-5-20251001]
    displayName: Claude Haiku 4.5
    createdAt: "2025-10-01T00:00:00Z"
    price:
      global: {input: 1, output: 5, cacheRead: 0.1, cacheWrite: 1.25}
      au: {input: 1, output: 5, cacheRead: 0.1, cacheWrite: 1.25}
    global: {model: anthropic/claude-haiku-4-5, verified: false}
    au: {model: "bedrock/au.anthropic.claude-haiku-4-5-20251001-v1:0", region: ap-southeast-2, verified: false}
```

Every Global entry is `verified: false` too: no call has been made from this repository with the org's key (owner question Q3). Task 23's optional real Global call, or task 24, flips each one it sees.

The prices are Anthropic's list prices from the same notes (input / output per million tokens: Opus 5 and Opus 4.8 US$5 / US$25, Sonnet 5 US$2 / US$10, Haiku 4.5 US$1 / US$5). Those notes say Bedrock is priced separately, so every AU figure is a copy, not a fact (decision (f)).

- [ ] **Step 5: Tidy, then run the catalogue tests to verify they pass**

```bash
cd control-plane && go mod tidy && git diff --stat go.sum
go test ./internal/inference/catalogue/... -count=1 -v 2>&1 | grep -E '^(--- |ok|FAIL)'
```

Expected: the `git diff --stat go.sum` line prints nothing (the promotion added no module; if `go.sum` changed, stop and find out why), `go.mod` shows `sigs.k8s.io/yaml v1.4.0` in the direct block, and:

```
--- PASS: TestTheEmbeddedCatalogueObeysEveryRule (0.00s)
--- PASS: TestResolve (0.00s)
--- PASS: TestParseRefusesARouteThatLeavesItsLane (0.00s)
--- PASS: TestParseRefusesAnAmbiguousOrMisspeltCatalogue (0.00s)
--- PASS: TestCheckAPIBase (0.00s)
--- PASS: TestModelsResponseIsTheAnthropicListShape (0.00s)
--- PASS: TestGroupsAreEveryModelOnBothRoutes (0.00s)
--- PASS: TestUnverifiedNamesEveryUnverifiedUpstream (0.00s)
--- PASS: TestPublicReturnsACopy (0.00s)
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue
```

`TestTheEmbeddedCatalogueObeysEveryRule` asks for `claude-opus-4-8` because it is the docs' example; if a later edit removes that model, change the test's id in the same commit, never delete the test.

- [ ] **Step 6: Write the failing generator tests**

The generator is tested through `render`, a pure function, and `run`, the flag front. The checked-in outputs are compared after one normalisation, `\r\n` to `\n`: the repository has no `.gitattributes` (checked 2026-09-25), so a Windows checkout with `core.autocrlf=true` rewrites line endings, and a byte test would fail on every developer machine here for a reason nobody edited. Any other difference is a hand edit.

Create `control-plane/cmd/gatewaygen/main_test.go`:

```go
package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue"
)

var outputs = map[string]string{ // target → checked-in file, from the repository root
	"local":           "deploy/local/gateway/litellm.config.yaml",
	"local-anthropic": "deploy/local/gateway/litellm.anthropic.config.yaml",
	"cloud":           "deploy/charts/gateway/files/litellm.config.yaml",
}

func cat(t *testing.T) *catalogue.Catalogue {
	t.Helper()
	c, err := catalogue.Load()
	if err != nil {
		t.Fatalf("catalogue.Load: %v", err)
	}
	return c
}

func mustRender(t *testing.T, o options) (raw []byte, doc map[string]any) {
	t.Helper()
	raw, err := render(cat(t), o)
	if err != nil {
		t.Fatalf("render(%+v): %v", o, err)
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("rendered YAML does not parse: %v", err)
	}
	return raw, doc
}

// groups returns model_name → litellm_params for every rendered entry.
func groups(t *testing.T, doc map[string]any) map[string]map[string]any {
	t.Helper()
	out := map[string]map[string]any{}
	for _, e := range doc["model_list"].([]any) {
		m := e.(map[string]any)
		out[m["model_name"].(string)] = m["litellm_params"].(map[string]any)
	}
	return out
}

// Kills: a hand edit to a generated file, and a model added to models.yaml
// without regenerating.
func TestCheckedInConfigsAreTheGeneratorsOutput(t *testing.T) {
	for target, rel := range outputs {
		want, _ := mustRender(t, options{Target: target})
		got, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		got = bytes.ReplaceAll(got, []byte("\r\n"), []byte("\n"))
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from `gatewaygen -target %s`; regenerate it, never edit it", rel, target)
		}
	}
}

// Kills: a group LiteLLM lacks (task 07 scopes every key to Groups(), and a
// missing one fails the mint), and a group nobody resolves to.
func TestEveryCatalogueGroupIsRenderedOnEveryTarget(t *testing.T) {
	want := strings.Join(cat(t).Groups(), ",")
	for target := range outputs {
		_, doc := mustRender(t, options{Target: target})
		var names []string
		for n := range groups(t, doc) {
			names = append(names, n)
		}
		sort.Strings(names)
		if got := strings.Join(names, ","); got != want {
			t.Errorf("%s: groups %s, want %s", target, got, want)
		}
	}
}

// pricedYAML prices AU above Global, so a render that prices the AU group from
// Global's figures is caught (the embedded catalogue has them equal).
const pricedYAML = `
litellm: {apiBaseForm: full}
models:
  - id: claude-haiku-4-5
    displayName: Claude Haiku 4.5
    createdAt: "2025-10-01T00:00:00Z"
    price:
      global: {input: 1, output: 5, cacheRead: 0.1, cacheWrite: 1.25}
      au: {input: 1.1, output: 5.5, cacheRead: 0.11, cacheWrite: 1.375}
    global: {model: anthropic/claude-haiku-4-5, verified: false}
    au: {model: "bedrock/au.anthropic.claude-haiku-4-5-20251001-v1:0", region: ap-southeast-2, verified: false}
`

// yaml11Float is PyYAML's float resolver (YAML 1.1), which LiteLLM loads with.
var yaml11Float = regexp.MustCompile(`^[-+]?(\.[0-9]+|[0-9][0-9_]*\.[0-9_]*([eE][-+][0-9]+)?)$`)

// Review I-5, decision (f). Kills: a group without model_info (LiteLLM's own
// table then charges US$0 and every budget is off), a per-million figure
// rendered as per token, the AU group priced from Global, and fixedCosts
// skipped (PyYAML would read 5e-06 as a string).
func TestEveryGroupIsPricedFromItsOwnRoute(t *testing.T) {
	fixture, err := catalogue.Parse([]byte(pricedYAML))
	if err != nil {
		t.Fatal(err)
	}
	for target := range outputs {
		for _, c := range []*catalogue.Catalogue{fixture, cat(t)} {
			raw, err := render(c, options{Target: target})
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range regexp.MustCompile(`(?m)^ +\w+_cost\w*: (\S+)$`).FindAllSubmatch(raw, -1) {
				if !yaml11Float.Match(m[1]) {
					t.Errorf("%s: price %s is not a YAML 1.1 float", target, m[1])
				}
			}
			var doc map[string]any
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				t.Fatal(err)
			}
			prices := map[string]catalogue.Cost{}
			for _, m := range c.Public() {
				prices[catalogue.GroupPrefixGlobal+m.ID] = m.Price.Global
				prices[catalogue.GroupPrefixAU+m.ID] = m.Price.AU
			}
			for _, e := range doc["model_list"].([]any) {
				entry := e.(map[string]any)
				name := entry["model_name"].(string)
				p := prices[name]
				info, _ := entry["model_info"].(map[string]any)
				for key, perM := range map[string]float64{"input_cost_per_token": p.InputPerMTok,
					"output_cost_per_token": p.OutputPerMTok, "cache_read_input_token_cost": p.CacheReadPerMTok,
					"cache_creation_input_token_cost": p.CacheWritePerMTok} {
					got, _ := info[key].(float64)
					if want := perM / 1e6; want == 0 || math.Abs(got-want) > want*1e-9 {
						t.Errorf("%s %s: %s = %v, want %v", target, name, key, got, want)
					}
				}
			}
		}
	}
}
```

```go
var secretish = []string{"key", "secret", "token", "password", "database_url"}

// priceKeys hold numbers, not credentials, though each names "token" (decision
// (f)). They are exempt from secretish only while their value is a number.
var priceKeys = map[string]bool{"input_cost_per_token": true, "output_cost_per_token": true,
	"cache_read_input_token_cost": true, "cache_creation_input_token_cost": true}

// walk visits every key/value pair in a decoded YAML tree.
func walk(v any, visit func(key string, val any)) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			visit(k, val)
			walk(val, visit)
		}
	case []any:
		for _, val := range x {
			walk(val, visit)
		}
	}
}

// Kills: any logging callback (P6-D9), and a credential rendered as a value
// instead of an os.environ/ reference (global constraint 2).
func TestNoCallbackAndNoLiteralSecretOnAnyTarget(t *testing.T) {
	for target := range outputs {
		_, doc := mustRender(t, options{Target: target})
		walk(doc, func(k string, val any) {
			if strings.Contains(strings.ToLower(k), "callback") {
				t.Errorf("%s: key %q is a logging callback", target, k)
			}
			if priceKeys[k] {
				if _, ok := val.(float64); !ok {
					t.Errorf("%s: %s is %v, want a number", target, k, val)
				}
				return
			}
			for _, s := range secretish {
				if strings.Contains(strings.ToLower(k), s) {
					if str, ok := val.(string); !ok || !strings.HasPrefix(str, "os.environ/") {
						t.Errorf("%s: %s is %v, want an os.environ/ reference", target, k, val)
					}
				}
			}
		})
	}
}

// Kills: omitempty dropping the two ZDR switches (LiteLLM's defaults would
// then apply, and they are not ours to assume).
func TestZDRSwitchesAreRenderedExplicitly(t *testing.T) {
	for target := range outputs {
		raw, _ := mustRender(t, options{Target: target})
		for _, line := range []string{"store_prompts_in_spend_logs: false", "turn_off_message_logging: true"} {
			if !bytes.Contains(raw, []byte(line)) {
				t.Errorf("%s: no %q", target, line)
			}
		}
	}
}

// Kills: a bare-host api_base on the cloud Global route (the Panacea 405), an AU
// group that leaves Sydney, and drop_params missing where P6-D3 needs it.
func TestCloudRoutes(t *testing.T) {
	_, doc := mustRender(t, options{Target: "cloud"})
	for name, p := range groups(t, doc) {
		switch {
		case strings.HasPrefix(name, catalogue.GroupPrefixGlobal):
			if _, has := p["api_base"]; has {
				t.Errorf("%s renders api_base %v; the cloud Global route has none", name, p["api_base"])
			}
			if !strings.HasPrefix(p["model"].(string), "anthropic/") || p["drop_params"] != nil {
				t.Errorf("%s: %v", name, p)
			}
		case strings.HasPrefix(name, catalogue.GroupPrefixAU):
			if !strings.HasPrefix(p["model"].(string), "bedrock/au.anthropic.") ||
				p["aws_region_name"] != catalogue.AURegion || p["drop_params"] != true {
				t.Errorf("%s: %v", name, p)
			}
			if _, has := p["api_key"]; has {
				t.Errorf("%s carries api_key; the AU route's credential is the pod's web identity", name)
			}
		}
	}
}

// Kills: an api_base that does not follow the recorded form, and an override
// that would reproduce the Panacea 405.
func TestAPIBaseFollowsTheForm(t *testing.T) {
	for form, suffix := range map[catalogue.APIBaseForm]string{catalogue.FormFull: ":4900/v1/messages", catalogue.FormHost: ":4900"} {
		_, doc := mustRender(t, options{Target: "local", APIBaseForm: form})
		if got := groups(t, doc)["global/claude-opus-4-8"]["api_base"].(string); !strings.HasSuffix(got, suffix) {
			t.Errorf("form %s: api_base %q, want suffix %q", form, got, suffix)
		}
	}
	if _, err := render(cat(t), options{Target: "cloud", GlobalAPIBase: "https://egress.example.internal"}); err == nil {
		t.Error("a bare-host Global override was rendered")
	}
	_, doc := mustRender(t, options{Target: "cloud", GlobalAPIBase: "https://egress.example.internal/v1/messages"})
	if got := groups(t, doc)["global/claude-opus-4-8"]["api_base"]; got != "https://egress.example.internal/v1/messages" {
		t.Errorf("override not rendered: %v", got)
	}
}

// Kills: an unknown target silently rendering something, and a local target
// accepting a Global override it would ignore.
func TestRunRefusesBadFlags(t *testing.T) {
	for _, args := range [][]string{
		{"-target", "prod", "-out", "-"},
		{"-target", "local", "-out", "-", "-global-api-base", "https://x/v1/messages"},
		{"-target", "local", "-out", "-", "-api-base-form", "relative"},
		{"-target", "local"},
	} {
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 2 || out.Len() != 0 {
			t.Errorf("run(%q) = %d with %d bytes out; want 2 and nothing", args, code, out.Len())
		}
	}
}
```

- [ ] **Step 7: Run the generator tests to verify they fail**

Run: `go test ./control-plane/cmd/gatewaygen/... -count=1`

Expected: `[build failed]` with `undefined: options`, `undefined: render` and `undefined: run`.

- [ ] **Step 8: Write the generator**

Create `control-plane/cmd/gatewaygen/main.go`, in two fenced blocks, one file. The key names under `litellm_settings` and `general_settings` follow LiteLLM's documented proxy config as the skeleton gives them, **UNVERIFIED** against the pinned image until task 18 (P06 for the two ZDR switches); `master_key` and `database_url` as `os.environ/` references are the form Panacea's values file shows (`~/Projects/Panacea/kubernetes/helm-values/litellm-values.yaml:222-223`, verified 2026-09-25, a commented example there).

```go
// Command gatewaygen renders LiteLLM's config.yaml from the model catalogue.
// The outputs are checked in and never hand-edited; a test compares them.
//
//	gatewaygen -target local|local-anthropic|cloud -out <file|-> \
//	    [-api-base-form full|host] [-global-api-base <url>]
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue"
)

// The local fakes (task 16), by their compose service names (task 17).
const (
	localGlobalHost = "http://fakeanthropic-global:4900"
	localAUHost     = "http://fakeanthropic-au:4901"
)

type options struct {
	Target        string                // local | local-anthropic | cloud
	APIBaseForm   catalogue.APIBaseForm // "" means the catalogue's recorded form
	GlobalAPIBase string                // cloud and local-anthropic only; "" means none
}

// The rendered shape. JSON tags, because sigs.k8s.io/yaml marshals through
// encoding/json, which sorts keys: the output is deterministic by construction.
type config struct {
	ModelList       []modelEntry    `json:"model_list"`
	LiteLLMSettings litellmSettings `json:"litellm_settings"`
	GeneralSettings generalSettings `json:"general_settings"`
}

type modelEntry struct {
	ModelName string    `json:"model_name"`
	Params    params    `json:"litellm_params"`
	Info      modelInfo `json:"model_info"` // every group, every target (decision (f))
}

// modelInfo is the group's price per token, so LiteLLM never prices a call
// from its own table, which lacks these ids and charges US$0 (review I-5). The
// key names are LiteLLM's from training knowledge, UNVERIFIED until task 18
// sees spend above zero (P03-P05, P12).
type modelInfo struct {
	InputCostPerToken      float64 `json:"input_cost_per_token"`
	OutputCostPerToken     float64 `json:"output_cost_per_token"`
	CacheReadCostPerToken  float64 `json:"cache_read_input_token_cost"`
	CacheWriteCostPerToken float64 `json:"cache_creation_input_token_cost"`
}

func perToken(c catalogue.Cost) modelInfo {
	return modelInfo{c.InputPerMTok / 1e6, c.OutputPerMTok / 1e6, c.CacheReadPerMTok / 1e6, c.CacheWritePerMTok / 1e6}
}

// costLine is one rendered price. yaml.Marshal writes 5e-06, which PyYAML, the
// YAML 1.1 loader LiteLLM reads its config with, resolves to a STRING: a 1.1
// float needs a dot. A string price would break LiteLLM's cost arithmetic.
var costLine = regexp.MustCompile(`(?m)^( +(?:input_cost_per_token|output_cost_per_token|cache_read_input_token_cost|cache_creation_input_token_cost): )(\S+)$`)

// fixedCosts rewrites every price in fixed notation (0.000005), a float to
// every YAML 1.1 and 1.2 parser.
func fixedCosts(body []byte) ([]byte, error) {
	var bad error
	out := costLine.ReplaceAllFunc(body, func(line []byte) []byte {
		m := costLine.FindSubmatch(line)
		v, err := strconv.ParseFloat(string(m[2]), 64)
		if err != nil {
			bad = fmt.Errorf("rendered price %q is not a number", m[2])
			return line
		}
		s := strconv.FormatFloat(v, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			s += ".0"
		}
		return append(append([]byte(nil), m[1]...), s...)
	})
	return out, bad
}

type params struct {
	Model         string `json:"model"`
	APIBase       string `json:"api_base,omitempty"`
	APIKey        string `json:"api_key,omitempty"`
	AWSRegionName string `json:"aws_region_name,omitempty"`
	DropParams    bool   `json:"drop_params,omitempty"` // AU only (P6-D3)
}

type litellmSettings struct {
	TurnOffMessageLogging bool `json:"turn_off_message_logging"` // no omitempty: always explicit
}

type generalSettings struct {
	MasterKey               string `json:"master_key"`
	DatabaseURL             string `json:"database_url"`
	StorePromptsInSpendLogs bool   `json:"store_prompts_in_spend_logs"` // no omitempty: always explicit
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gatewaygen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o options
	var form, out string
	fs.StringVar(&o.Target, "target", "", "local, local-anthropic or cloud")
	fs.StringVar(&out, "out", "", "output file, or - for stdout")
	fs.StringVar(&form, "api-base-form", "", "full or host (default: the catalogue's)")
	fs.StringVar(&o.GlobalAPIBase, "global-api-base", "", "Global api_base override (cloud, local-anthropic)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	o.APIBaseForm = catalogue.APIBaseForm(form)
	if out == "" {
		fmt.Fprintln(stderr, "gatewaygen: -out is required")
		return 2
	}
	c, err := catalogue.Load()
	if err != nil {
		fmt.Fprintln(stderr, "gatewaygen:", err)
		return 1
	}
	b, err := render(c, o)
	if err != nil {
		fmt.Fprintln(stderr, "gatewaygen:", err)
		return 2
	}
	if out == "-" {
		_, err = stdout.Write(b)
	} else {
		err = writeAtomic(out, b)
	}
	if err != nil {
		fmt.Fprintln(stderr, "gatewaygen:", err)
		return 1
	}
	return 0
}
```

```go
// The fakes get their own throwaway key variable, so that in local-anthropic a
// developer's real Anthropic key is never sent to the AU fake.
const (
	anthropicKey = "os.environ/ANTHROPIC_API_KEY"
	fakeKey      = "os.environ/PANACEA_APP_GATEWAY_FAKE_UPSTREAM_KEY"
)

func render(c *catalogue.Catalogue, o options) ([]byte, error) {
	form := o.APIBaseForm
	if form == "" {
		form = c.LiteLLM().APIBaseForm
	}
	base := func(host string) string {
		if form == catalogue.FormFull {
			return host + "/v1/messages"
		}
		return host
	}
	switch o.Target {
	case "local", "local-anthropic", "cloud":
	default:
		return nil, fmt.Errorf("-target %q is not local, local-anthropic or cloud", o.Target)
	}
	if o.GlobalAPIBase != "" && o.Target == "local" {
		return nil, errors.New("-global-api-base applies to cloud and local-anthropic; local points at the fakes")
	}
	cfg := config{
		LiteLLMSettings: litellmSettings{TurnOffMessageLogging: true},
		GeneralSettings: generalSettings{MasterKey: "os.environ/LITELLM_MASTER_KEY", DatabaseURL: "os.environ/DATABASE_URL"},
	}
	for _, m := range c.Public() {
		g := params{Model: m.Global.Model, APIKey: anthropicKey, APIBase: o.GlobalAPIBase}
		au := params{Model: m.Global.Model, APIKey: fakeKey, APIBase: base(localAUHost), DropParams: true}
		switch o.Target {
		case "local":
			g = params{Model: m.Global.Model, APIKey: fakeKey, APIBase: base(localGlobalHost)}
		case "cloud":
			// No credential: the pod's AWS web identity (task 02 decision (d)).
			au = params{Model: m.AU.Model, AWSRegionName: m.AU.Region, DropParams: true}
		}
		for _, p := range []string{g.APIBase, au.APIBase} {
			if p == "" {
				continue
			}
			if err := catalogue.CheckAPIBase(form, p); err != nil {
				return nil, err
			}
		}
		cfg.ModelList = append(cfg.ModelList,
			modelEntry{ModelName: catalogue.GroupPrefixGlobal + m.ID, Params: g, Info: perToken(m.Price.Global)},
			modelEntry{ModelName: catalogue.GroupPrefixAU + m.ID, Params: au, Info: perToken(m.Price.AU)})
	}
	body, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if body, err = fixedCosts(body); err != nil {
		return nil, err
	}
	return append(header(c, o.Target, form), body...), nil
}

func header(c *catalogue.Catalogue, target string, form catalogue.APIBaseForm) []byte {
	var b strings.Builder
	b.WriteString("# GENERATED by control-plane/cmd/gatewaygen. Do not edit this file: change\n")
	b.WriteString("# control-plane/internal/inference/catalogue/models.yaml, then regenerate with\n")
	fmt.Fprintf(&b, "#   go run ./control-plane/cmd/gatewaygen -target %s -out <this file>\n", target)
	fmt.Fprintf(&b, "# api_base form: %s\n", form)
	b.WriteString("# model_info prices are UNVERIFIED (models.yaml, review I-5); spend is only as right as they are\n")
	if u := c.Unverified(); len(u) > 0 {
		b.WriteString("# UNVERIFIED upstream ids, not yet seen in a real account:\n")
		for _, s := range u {
			fmt.Fprintf(&b, "#   %s\n", s)
		}
	}
	return []byte(b.String())
}

func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".gatewaygen-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // a no-op after the rename
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
```

- [ ] **Step 9: Generate the three outputs**

```bash
go run ./control-plane/cmd/gatewaygen -target local           -out deploy/local/gateway/litellm.config.yaml
go run ./control-plane/cmd/gatewaygen -target local-anthropic -out deploy/local/gateway/litellm.anthropic.config.yaml
go run ./control-plane/cmd/gatewaygen -target cloud           -out deploy/charts/gateway/files/litellm.config.yaml
```

Each exits 0 silently. The cloud file begins like this (the header's unverified list runs to all eight upstreams; key order is `encoding/json`'s, sorted; whether the emitter quotes the `…-v1:0` value is the library's choice and either form is the same YAML):

```yaml
# GENERATED by control-plane/cmd/gatewaygen. Do not edit this file: change
# control-plane/internal/inference/catalogue/models.yaml, then regenerate with
#   go run ./control-plane/cmd/gatewaygen -target cloud -out <this file>
# api_base form: full
# UNVERIFIED upstream ids, not yet seen in a real account:
#   au/claude-haiku-4-5 = bedrock/au.anthropic.claude-haiku-4-5-20251001-v1:0
#   ...
general_settings:
  database_url: os.environ/DATABASE_URL
  master_key: os.environ/LITELLM_MASTER_KEY
  store_prompts_in_spend_logs: false
litellm_settings:
  turn_off_message_logging: true
model_list:
- litellm_params:
    api_key: os.environ/ANTHROPIC_API_KEY
    model: anthropic/claude-opus-5
  model_name: global/claude-opus-5
- litellm_params:
    aws_region_name: ap-southeast-2
    drop_params: true
    model: bedrock/au.anthropic.claude-opus-5
  model_name: au/claude-opus-5
```

Task 17's compose must set `PANACEA_APP_GATEWAY_FAKE_UPSTREAM_KEY` to any non-empty value on the `litellm` service, and `ANTHROPIC_API_KEY` only in `anthropic` mode; task 19's chart must set `ANTHROPIC_API_KEY`, `AWS_ROLE_ARN` and `AWS_WEB_IDENTITY_TOKEN_FILE` (decision (d)). Both are in the gap list of the return.

- [ ] **Step 10: Run both packages to verify they pass**

```bash
cd control-plane && go test ./internal/inference/catalogue/... ./cmd/gatewaygen/... -count=1
```

Expected:

```
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue
ok  	github.com/kpmg-au-emu/panacea-app/control-plane/cmd/gatewaygen
```

- [ ] **Step 11: Mutation spot-check, by hand, then revert**

| Edit | Test that must fail |
|---|---|
| `Resolve`: the `default` `return "", false` → `return GroupPrefixGlobal + id, true` | `TestResolve` (the `""` and `"eu"` rows) |
| `validate`: delete the `auProfilePrefix` check | `TestParseRefusesARouteThatLeavesItsLane` (`apac profile`, `global profile`, `us profile`) |
| `CheckAPIBase`: `FormFull` returns `nil` without the suffix check | `TestCheckAPIBase`, `TestAPIBaseFollowsTheForm` |
| `generalSettings`: add `,omitempty` to `store_prompts_in_spend_logs` | `TestZDRSwitchesAreRenderedExplicitly` |
| `render`, cloud: give the AU group `APIKey: anthropicKey` | `TestCloudRoutes`, and `TestCheckedInConfigsAreTheGeneratorsOutput` (the render no longer matches the checked-in file) |
| Append a comment line by hand to `deploy/charts/gateway/files/litellm.config.yaml` | `TestCheckedInConfigsAreTheGeneratorsOutput` |
| `validate`: delete the `m.Price.AU.validate()` check (review I-5) | `TestParseRefusesAnUnpricedModel` (`no AU price`, `no AU cache read`, `negative AU input`) |
| `Cost.validate`: `f.v >= minPerMTok` → `f.v > 0` | `TestParseRefusesAnUnpricedModel` (`per token`) |
| `render`: the AU entry's `Info: perToken(m.Price.AU)` → `perToken(m.Price.Global)` | `TestEveryGroupIsPricedFromItsOwnRoute` (the `pricedYAML` AU figures) |
| `render`: delete the `fixedCosts` call | `TestEveryGroupIsPricedFromItsOwnRoute` (`5e-06` is not a YAML 1.1 float) |
| `perToken`: drop the `/ 1e6` on `OutputPerMTok` | `TestEveryGroupIsPricedFromItsOwnRoute` |

- [ ] **Step 12: Format, vet, and confirm the import set**

```bash
cd control-plane && gofmt -l internal/inference/catalogue cmd/gatewaygen && go vet ./internal/inference/catalogue/... ./cmd/gatewaygen/...
go list -deps ./internal/inference/catalogue | grep -E 'kpmg-au-emu|sigs.k8s.io'
```

Expected: nothing from `gofmt` and `go vet`, then exactly:

```
sigs.k8s.io/yaml
github.com/kpmg-au-emu/panacea-app/control-plane/internal/dataclass
github.com/kpmg-au-emu/panacea-app/control-plane/internal/inference/catalogue
```

(`sigs.k8s.io/yaml/goyaml.v2`, its vendored parser, may also be listed; any other `kpmg-au-emu` line is a dependency this task did not plan.)

- [ ] **Step 13: Commit**

Write the message to a file (the shell rule: no heredoc), then commit only this task's paths:

```bash
git add control-plane/internal/inference/catalogue control-plane/cmd/gatewaygen \
  deploy/local/gateway/litellm.config.yaml deploy/local/gateway/litellm.anthropic.config.yaml \
  deploy/charts/gateway/files/litellm.config.yaml
git commit -F /tmp/task-02-msg.txt -- control-plane/internal/inference/catalogue control-plane/cmd/gatewaygen \
  control-plane/go.mod deploy/local/gateway/litellm.config.yaml deploy/local/gateway/litellm.anthropic.config.yaml \
  deploy/charts/gateway/files/litellm.config.yaml
```

with `/tmp/task-02-msg.txt`:

```
feat(inference): the model catalogue and the generated LiteLLM config

One embedded models.yaml names every model and where it runs on each
route. The loader refuses an AU upstream that is not a Bedrock au.
profile in ap-southeast-2, a Global upstream that is not the Anthropic
provider, a duplicate or slash-bearing id, a misspelt key, a family
not served under zero data retention, and a route with no usable price.

gatewaygen renders LiteLLM's config for local, local-anthropic and cloud
from the catalogue: no callbacks, prompt storage off, every credential an
os.environ/ reference, no api_base on the cloud Global route, any
api_base in the recorded form (full until task 18 measures otherwise),
and every group's UNVERIFIED price in model_info, in fixed notation so
PyYAML reads a number (review I-5: an unpriced group spends US$0).
The three outputs are checked in and a test fails on any hand edit.

Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>
```

**Acceptance:** step 10 prints two `ok` lines; each row of step 11 turns its named test red and reverting turns it green; step 12 prints nothing and then the expected import lines; `git show --stat HEAD` lists the three generated files, the two packages and `control-plane/go.mod`, and no `go.sum`.

**What this task does not claim:**

- **That the model ids are current or exist in either account.** They are the model notes' set as cached on 2026-06-24. Every Bedrock profile id, and every Global id, is `verified: false` until a real call sees it (task 23's optional Global call, task 24's profile listing).
- **That the prices are right, or that LiteLLM reads them** (decision (f), review I-5). Global is the cached list price, cache read and write are ratios, and AU is a copy of Global until an operator checks the AWS price list (task 24); task 24's P06 checks only that each group has a price above zero. That the pinned LiteLLM prices from `model_info` under these four key names is training knowledge: task 18 shows it when spend comes back above zero (P03–P05, P12). If it prices from `litellm_params` instead, moving `Info` there is a one-line change to `modelEntry`. Task 09's sweep flags a key with requests and no spend either way.
- **That LiteLLM honours every rendered key.** `turn_off_message_logging`, `store_prompts_in_spend_logs`, per-model `drop_params` and `aws_region_name` are LiteLLM's documented names as the skeleton gives them, not re-read here; task 18's P06 and P10 measure the first and third against the pinned image.
- **That LiteLLM falls back to the pod's AWS web identity** when an AU group names no credential (decision (d)). Training knowledge of LiteLLM's use of boto3's default chain, **UNVERIFIED** until task 24's P04.
- **That `full` is the right `api_base` form for the pinned LiteLLM.** It is the Panacea lesson, from an incident record outside the repository tree; task 18's P07 measures it, and a change to `host` must cite that measurement.
- **That the `/v1/models` body is exactly Anthropic's shape** (training knowledge; task 23 watches the CLI's picker).
- **That the ZDR exclusion list is complete.** It holds the two families the cached model notes name; the org's actual retention agreement is owner question Q3.

**Gaps against the skeleton, for the orchestrator:**

1. **A promoted dependency.** `sigs.k8s.io/yaml v1.4.0` moves from indirect to direct (decision (a)); global constraint 1 names only `pgx/v5`. No module is added and `go.sum` does not change.
2. **A third generated file**, `deploy/local/gateway/litellm.anthropic.config.yaml` (target `local-anthropic`), for task 17's `PANACEA_APP_GATEWAY_LOCAL_UPSTREAM=anthropic`. The file map lists two outputs.
3. **A new variable**, `PANACEA_APP_GATEWAY_FAKE_UPSTREAM_KEY`, which task 17's compose must set on `litellm` (any non-empty value), so a developer's real key never reaches a fake.
4. **Task 19 must set `AWS_ROLE_ARN`, `AWS_WEB_IDENTITY_TOKEN_FILE` and a projected service-account token with audience `sts.amazonaws.com`** on the LiteLLM pod (decision (d)); its scope names only the Key Vault files. Task 20's "web-identity parameters, whose names … belong to task 02's cloud config" are therefore not LiteLLM parameters at all.
5. **`Groups()`, `Unverified()`, `LiteLLM()` and `CheckAPIBase`** are added to the skeleton's API; task 07 uses `Groups()` for its key scope.
6. **Two families are excluded** (`claude-fable-`, `claude-mythos-`): the cached model notes say they need 30-day retention, which OD-19's ZDR rules out.
