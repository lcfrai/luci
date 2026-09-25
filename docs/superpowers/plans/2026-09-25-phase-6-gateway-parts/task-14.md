### Task 14: `panacea token` and the managed-settings fragment

This task is the sandbox's whole side of the gateway: one small command and one settings file. Claude Code, unmodified, is told through a documented setting to fetch its credential by running `panacea token`; the command prints the session token the launcher put in a mounted directory, and nothing else in the sandbox ever holds the token in an environment variable.

**What the docs say, read by this part on 2026-09-25** (https://code.claude.com/docs/en/authentication, "Credential management" and "Authentication precedence"; https://code.claude.com/docs/en/managed-settings, "Deploy a managed settings file" and "Split a file-based policy across teams"):

- *"configure the `apiKeyHelper` setting to run a shell script that returns an API key"*; *"Claude Code re-runs `apiKeyHelper` after five minutes by default. Set the `CLAUDE_CODE_API_KEY_HELPER_TTL_MS` environment variable for custom refresh intervals."*
- *"if `apiKeyHelper` takes longer than 10 seconds to return a key, Claude Code displays a warning notice in the prompt bar"*; *"when the script exits with an error, times out, or prints nothing, requests fail with `Your apiKeyHelper script is failing` within three attempts."* The helper's own timeout is not stated.
- Precedence: `ANTHROPIC_AUTH_TOKEN` (2) and `ANTHROPIC_API_KEY` (3) both rank **above** `apiKeyHelper` (4). A stray variable would silently replace the session token.
- `forceLoginOrgUUID` or `forceLoginMethod: "claudeai"|"console"` in any settings file blocks `apiKeyHelper` **at startup**, *"since organization membership can't be verified for an environment credential."*
- Linux: `/etc/claude-code/managed-settings.json`, plus *"an optional `managed-settings.d/` directory"* in `/etc/claude-code/`. *"Claude Code merges `managed-settings.json` first, then every `*.json` file in the directory in alphabetical order"*; single values are replaced by the later file, lists combine, and nested blocks such as `env` *"merge key by key"*.
- A managed file or drop-in that *"isn't valid JSON, or its top level isn't an object"* makes Claude Code **refuse to start**.

Four decisions follow from those lines:

- **(a) The fragment is a drop-in, `/etc/claude-code/managed-settings.d/50-panacea-gateway.json`.** That is OD-23's merge, done by Claude Code itself: whatever policy mechanism later owns `managed-settings.json` (seam S6.6) leaves this file alone, and the key-by-key `env` merge means the fragment's two variables survive a base file that sets its own. The skeleton's "installs the fragment at the managed-settings path" becomes this; the path is no longer **UNVERIFIED**. Whether a drop-in is read when no `managed-settings.json` exists beside it is not stated, and task 23 checks it with `/status`.
- **(b) The fragment can never block its own helper.** A test refuses `forceLoginOrgUUID` and `forceLoginMethod` in it, and the part names the same constraint on the base file for S6.6 (a KPMG-derived `managed-settings.json` carrying `forceLoginOrgUUID` would stop every gateway session at startup).
- **(c) `panacea token` never prints what it cannot vouch for.** It reads only the file named `token`, trims surrounding whitespace, and prints it only if what remains is 1 to 4096 printable ASCII characters with no space. Anything else is an exit 1 with a message that quotes no byte of the file. `token.tmp`, the launcher's half-written file (task 15), is ignored by construction, not by a filter.
- **(d) The wait is 20 seconds, polled every 250 ms, and the 10-second warning is accepted.** The first call of a session can wait for a slow mint (the Panacea lesson, skeleton pitfall 3); past 10 s the CLI shows a notice, which is the honest signal. Whether the CLI's helper timeout is shorter than 20 s is **UNVERIFIED**; if it is, the first request fails, the CLI retries within three attempts, and the mint has usually landed by then. Task 23 observes it.

**Files:**
- `sandboxd/cmd/panacea/main.go` — `run`, `waitForToken`, `readToken`, the clock seam (new; standard library only, so nothing to vendor)
- `sandboxd/cmd/panacea/main_test.go` — the command's tests (new)
- `sandboxd/cmd/panacea/fragment_test.go` — the fragment's and the Dockerfile's tests; it lives in the Go module because `images/` has none, and it reads both files by path from the repository root (new)
- `images/sandbox/managed-settings.gateway.json` — the fragment (new)
- `images/sandbox/Dockerfile` — build and install `/usr/local/bin/panacea`; install the fragment as the drop-in (modified)

**Interfaces:**
- Produces: the command line `panacea token` (exit 0 and the token on stdout with no newline; exit 1 and one line on stderr; exit 2 on any other arguments); `PANACEA_APP_GATEWAY_TOKEN_DIR` (default `/run/panacea/gateway`); the settings keys `apiKeyHelper`, `env.CLAUDE_CODE_API_KEY_HELPER_TTL_MS`, `env.CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC`.
- Consumes: the file `<dir>/token`, written atomically by the launcher (task 15 locally, the broker in the cloud, ask P8-1); the skeleton's shared name for the directory (`/run/panacea/gateway/`, a mounted directory, never a mounted file).
- Required by: task 15 (writes the file this reads; sets `ANTHROPIC_BASE_URL` and, on AU, `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1` in the launch environment), task 23 (runs the image's `claude` through it), seam S6.6.

- [ ] **Step 1: Write the failing command tests**

Time is a seam: a fake clock advances only when the command sleeps, and can run a hook at the n-th sleep, so "the token lands one second late" is exact and the 20-second timeout takes no wall time.

Create `sandboxd/cmd/panacea/main_test.go`:

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeClock struct {
	t      time.Time
	sleeps []time.Duration
	at     map[int]func() // run after the n-th sleep, 1-based
}

func (c *fakeClock) Now() time.Time { return c.t }

func (c *fakeClock) Sleep(d time.Duration) {
	c.sleeps = append(c.sleeps, d)
	c.t = c.t.Add(d)
	if f := c.at[len(c.sleeps)]; f != nil {
		f()
	}
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)} }

func runIn(dir string, c *fakeClock, args ...string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	env := func(k string) string {
		if k == "PANACEA_APP_GATEWAY_TOKEN_DIR" {
			return dir
		}
		return ""
	}
	code = run(args, env, &out, &errb, c)
	return code, out.String(), errb.String()
}

func put(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Kills a trailing newline on stdout and kills waiting when the token is there.
func TestTheTokenIsPrintedAloneAndAtOnce(t *testing.T) {
	dir, c := t.TempDir(), newClock()
	put(t, filepath.Join(dir, "token"), "sk-pa-abc_DEF-123\n")
	code, out, errOut := runIn(dir, c, "token")
	if code != 0 || out != "sk-pa-abc_DEF-123" || errOut != "" || len(c.sleeps) != 0 {
		t.Fatalf("code %d, stdout %q, stderr %q, %d sleeps", code, out, errOut, len(c.sleeps))
	}
}

// Kills no wait: the mint lands one second after the CLI first asks.
func TestATokenWrittenLateIsPrinted(t *testing.T) {
	dir, c := t.TempDir(), newClock()
	c.at = map[int]func(){4: func() { put(t, filepath.Join(dir, "token"), "sk-pa-late") }}
	if code, out, _ := runIn(dir, c, "token"); code != 0 || out != "sk-pa-late" || len(c.sleeps) != 4 {
		t.Fatalf("code %d, stdout %q, %d sleeps; want the token after 4 polls", code, out, len(c.sleeps))
	}
}

// Kills a busy loop, a longer poll, and a bound other than 20 s. Whitespace
// alone is "not yet", like an absent file.
func TestTheWaitIsTwentySecondsInQuarterSeconds(t *testing.T) {
	dir, c := t.TempDir(), newClock()
	put(t, filepath.Join(dir, "token"), " \n")
	code, out, errOut := runIn(dir, c, "token")
	var total time.Duration
	for _, d := range c.sleeps {
		if d != 250*time.Millisecond {
			t.Fatalf("slept %s, want every poll 250ms", d)
		}
		total += d
	}
	if code != 1 || out != "" || total != 20*time.Second || !strings.Contains(errOut, "no gateway token") {
		t.Fatalf("code %d, stdout %q, waited %s, stderr %q", code, out, total, errOut)
	}
}
```

The refusals, same file:

```go
// Kills reading the launcher's half-written token.tmp (a glob, or "the newest
// file"), and kills echoing file content in an error.
func TestOnlyTheFileNamedTokenIsRead(t *testing.T) {
	dir, c := t.TempDir(), newClock()
	put(t, filepath.Join(dir, "token.tmp"), "sk-pa-half-written-SECRET")
	code, out, errOut := runIn(dir, c, "token")
	if code != 1 || out != "" || strings.Contains(errOut, "SECRET") {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out, errOut)
	}
}

// Kills printing a value the CLI would put in a header as-is, and kills
// quoting it back. A malformed file fails at once: waiting cannot fix it.
func TestAMalformedTokenIsRefusedWithoutEcho(t *testing.T) {
	for name, body := range map[string]string{
		"a space":     "sk-pa-SECRET 1",
		"a newline":   "sk-pa-SECRET\nsecond-line",
		"a NUL":       "sk-pa-SECRET\x00",
		"non-ASCII":   "sk-pa-SECRET-é",
		"over 4 KiB":  "sk-pa-SECRET" + strings.Repeat("a", 4096),
		"a control":   "sk-pa-SECRET\x1b[2J",
	} {
		dir, c := t.TempDir(), newClock()
		put(t, filepath.Join(dir, "token"), body)
		code, out, errOut := runIn(dir, c, "token")
		if code != 1 || out != "" || strings.Contains(errOut, "SECRET") || len(c.sleeps) != 0 {
			t.Errorf("%s: code %d, stdout %q, stderr %q, %d sleeps", name, code, out, errOut, len(c.sleeps))
		}
	}
}

// Kills a command that treats anything as "token" (team mode's panacea family
// extends this binary; an unknown verb must not print a credential).
func TestAnyOtherInvocationExitsTwo(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "token"), "sk-pa-abc")
	for _, args := range [][]string{nil, {"tok"}, {"token", "--json"}, {"--help"}, {"Token"}} {
		if code, out, _ := runIn(dir, newClock(), args...); code != 2 || out != "" {
			t.Errorf("panacea %q: code %d, stdout %q; want 2 and nothing", args, code, out)
		}
	}
}

// Kills a default directory other than the skeleton's shared name.
func TestTheDefaultDirectoryIsTheSharedName(t *testing.T) {
	if got := tokenPath(func(string) string { return "" }); got != filepath.Join("/run/panacea/gateway", "token") {
		t.Errorf("tokenPath = %q", got)
	}
}
```

- [ ] **Step 2: Write the failing fragment and image tests**

**One value differs from the skeleton, deliberately: the helper TTL is `120000` (2 minutes), not `300000`.** Task 07 refreshes at `REFRESH_AT` (45 minutes) and revokes the previous generation `Grace` (5 minutes) later. The CLI keeps presenting the old token until it next runs the helper, which is up to one helper TTL after the new token lands. With a 5-minute TTL that is exactly the grace, so a request at the boundary is a `401`; the TTL must be strictly below the grace, with room for the CLI's own scheduling. Two minutes costs one file read per two minutes per session. The test states the inequality, so a later change to either side fails here.

Create `sandboxd/cmd/panacea/fragment_test.go`:

```go
package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	fragmentPath   = "../../../images/sandbox/managed-settings.gateway.json" // from sandboxd/cmd/panacea
	dockerfilePath = "../../../images/sandbox/Dockerfile"
	dropIn         = "/etc/claude-code/managed-settings.d/50-panacea-gateway.json"
	refreshAt      = 45 * time.Minute // PANACEA_APP_GATEWAY_REFRESH_AT, skeleton shared names
	grace          = 5 * time.Minute  // task 07's Grace
)

func repoFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("%s: %v (run from sandboxd/cmd/panacea)", p, err)
	}
	return string(b)
}

// Kills a malformed fragment (Claude Code refuses to start on one) and any
// key beyond the gateway's three.
func TestTheFragmentIsExactlyTheGatewayKeys(t *testing.T) {
	var got map[string]any
	if err := json.Unmarshal([]byte(repoFile(t, fragmentPath)), &got); err != nil {
		t.Fatalf("the fragment is not one JSON object: %v", err)
	}
	want := map[string]any{"apiKeyHelper": "/usr/local/bin/panacea token", "env": map[string]any{
		"CLAUDE_CODE_API_KEY_HELPER_TTL_MS": "120000", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fragment %v, want %v", got, want)
	}
}

// Kills a credential or a provider switch in the environment (both outrank
// the helper, or bypass the gateway) and kills a login restriction, which
// blocks apiKeyHelper at startup.
func TestTheFragmentCarriesNoCredentialAndNoLoginRestriction(t *testing.T) {
	f := repoFile(t, fragmentPath)
	for _, banned := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL",
		"CLAUDE_CODE_USE_BEDROCK", "AWS_", "forceLoginMethod", "forceLoginOrgUUID", "sk-"} {
		if strings.Contains(f, banned) {
			t.Errorf("the fragment contains %q", banned)
		}
	}
}

// Kills a helper TTL at or past the grace: the CLI would present a revoked token.
func TestTheHelperTTLIsInsideTheRefreshGrace(t *testing.T) {
	var f struct{ Env map[string]string }
	_ = json.Unmarshal([]byte(repoFile(t, fragmentPath)), &f)
	ms, err := strconv.Atoi(f.Env["CLAUDE_CODE_API_KEY_HELPER_TTL_MS"])
	ttl := time.Duration(ms) * time.Millisecond
	if err != nil || ttl < 30*time.Second || ttl >= grace || refreshAt+ttl >= refreshAt+grace {
		t.Errorf("helper TTL %s: want 30s <= TTL < the %s grace", ttl, grace)
	}
}

// Kills a helper path the image does not have, a fragment the CLI never
// reads, and a credential baked into the image's environment.
func TestTheImageInstallsTheHelperAndTheDropIn(t *testing.T) {
	d := repoFile(t, dockerfilePath)
	for _, want := range []string{"go build -o /out/panacea ./cmd/panacea",
		"COPY --from=build /out/panacea /usr/local/bin/panacea",
		"COPY images/sandbox/managed-settings.gateway.json " + dropIn} {
		if !strings.Contains(d, want) {
			t.Errorf("the Dockerfile lacks %q", want)
		}
	}
	for _, line := range strings.Split(d, "\n") {
		if l := strings.TrimSpace(line); strings.HasPrefix(l, "ENV") && strings.Contains(l, "ANTHROPIC") {
			t.Errorf("the Dockerfile sets %q", l)
		}
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `cd sandboxd && go test ./cmd/panacea/`
Expected: a build failure, `undefined: run` and `undefined: tokenPath`. (The fragment tests would fail on the missing files, but the build fails first.)

- [ ] **Step 4: Write the command**

Create `sandboxd/cmd/panacea/main.go`:

```go
// Command panacea is the product's command inside a sandbox. Today it has one
// verb, `panacea token`, which Claude Code runs as its apiKeyHelper. Team
// mode's panacea family (05-design C4) extends this binary.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	envTokenDir = "PANACEA_APP_GATEWAY_TOKEN_DIR"
	defaultDir  = "/run/panacea/gateway" // a mounted directory, never a mounted file
	pollEvery   = 250 * time.Millisecond
	waitFor     = 20 * time.Second
	maxToken    = 4096
	readLimit   = maxToken + 64 // room for trailing whitespace, and a way to see "too long"
)

type clock interface {
	Now() time.Time
	Sleep(time.Duration)
}

type realClock struct{}

func (realClock) Now() time.Time        { return time.Now() }
func (realClock) Sleep(d time.Duration) { time.Sleep(d) }

var (
	errNotYet    = errors.New("not yet")
	errMalformed = errors.New("the token file does not hold a token") // never quotes the file
)

func main() { os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr, realClock{})) }

func run(args []string, getenv func(string) string, stdout, stderr io.Writer, c clock) int {
	if len(args) != 1 || args[0] != "token" {
		fmt.Fprintln(stderr, "usage: panacea token")
		return 2
	}
	tok, err := waitForToken(tokenPath(getenv), c)
	if err != nil {
		fmt.Fprintln(stderr, "panacea token:", err)
		return 1
	}
	if _, err := io.WriteString(stdout, tok); err != nil {
		return 1
	}
	return 0
}

func tokenPath(getenv func(string) string) string {
	dir := getenv(envTokenDir)
	if dir == "" {
		dir = defaultDir
	}
	return filepath.Join(dir, "token")
}

// waitForToken polls while the file is absent or blank; anything else ends it.
func waitForToken(path string, c clock) (string, error) {
	deadline := c.Now().Add(waitFor)
	for {
		tok, err := readToken(path)
		if !errors.Is(err, errNotYet) {
			return tok, err
		}
		if !c.Now().Before(deadline) {
			return "", fmt.Errorf("no gateway token after %s; this session's mint has not landed, and the cockpit shows why", waitFor)
		}
		c.Sleep(pollEvery)
	}
}

func readToken(path string) (string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", errNotYet
	}
	if err != nil {
		return "", fmt.Errorf("the token file could not be opened: %w", err) // a *PathError: the path and errno, no content
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, readLimit))
	if err != nil {
		return "", errors.New("the token file could not be read")
	}
	tok := strings.TrimSpace(string(b))
	switch {
	case tok == "":
		return "", errNotYet
	case len(tok) > maxToken || len(b) == readLimit:
		return "", errMalformed
	}
	for i := 0; i < len(tok); i++ {
		if tok[i] < 0x21 || tok[i] > 0x7e { // printable ASCII, no space
			return "", errMalformed
		}
	}
	return tok, nil
}
```

- [ ] **Step 5: Write the fragment and install both in the image**

Create `images/sandbox/managed-settings.gateway.json`:

```json
{
  "apiKeyHelper": "/usr/local/bin/panacea token",
  "env": {
    "CLAUDE_CODE_API_KEY_HELPER_TTL_MS": "120000",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
  }
}
```

In `images/sandbox/Dockerfile`, two edits and nothing else. Line 8 (`RUN cd sandboxd && … go build -o /out/sandboxd ./cmd/sandboxd`) becomes two builds from the same vendored module:

```dockerfile
RUN cd sandboxd && CGO_ENABLED=0 GOFLAGS=-mod=vendor go build -o /out/sandboxd ./cmd/sandboxd \
 && CGO_ENABLED=0 GOFLAGS=-mod=vendor go build -o /out/panacea ./cmd/panacea
```

And after line 20 (`COPY --from=build /out/sandboxd /usr/local/bin/sandboxd`), while the build is still root:

```dockerfile
COPY --from=build /out/panacea /usr/local/bin/panacea
# The gateway's settings fragment, as a managed-settings drop-in (phase 6 task 14,
# seam S6.6). Claude Code merges managed-settings.json first, then every *.json in
# managed-settings.d/ alphabetically; env blocks merge key by key. A drop-in that
# is not one JSON object stops every session from starting, so it is tested.
COPY images/sandbox/managed-settings.gateway.json /etc/claude-code/managed-settings.d/50-panacea-gateway.json
```

`COPY` leaves both root-owned and world-readable, which is what a managed file needs: `dev` (uid 1001, line 19) can read the policy and run the helper, and cannot change either. The image creates nothing at `/run/panacea/gateway`: the launcher mounts the directory, and a directory baked into the image would make a missing mount look like a mint that has not landed yet.

- [ ] **Step 6: Run the tests to verify they pass**

```bash
cd sandboxd
go test -count=1 ./cmd/panacea/                                          # expect: ok
go test -count=1 -v ./cmd/panacea/ | grep -c '^--- PASS: Test'           # expect 11
```

- [ ] **Step 7: Kill four mutants by hand**

| Edit | Test that must fail |
|---|---|
| `run`: `io.WriteString(stdout, tok)` → `fmt.Fprintln(stdout, tok)` | `TestTheTokenIsPrintedAloneAndAtOnce` |
| `waitForToken`: `c.Sleep(pollEvery)` loop replaced by a single `readToken(path)` | `TestATokenWrittenLateIsPrinted` |
| `readToken`: the printable-ASCII loop deleted | `TestAMalformedTokenIsRefusedWithoutEcho` (the space, newline, NUL, control and non-ASCII rows) |
| fragment: `"120000"` → `"300000"` | `TestTheFragmentIsExactlyTheGatewayKeys`, `TestTheHelperTTLIsInsideTheRefreshGrace` |

- [ ] **Step 8: Vet, and prove the image carries both files**

```bash
cd sandboxd && go vet ./cmd/panacea/ && gofmt -l cmd/panacea/                 # expect no output
go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}' ./cmd/panacea/  # expect one line, the package itself: nothing to vendor
cd .. && docker build -q -t panacea-sandbox:task14 -f images/sandbox/Dockerfile .
docker run --rm --entrypoint cat panacea-sandbox:task14 /etc/claude-code/managed-settings.d/50-panacea-gateway.json
docker run --rm --entrypoint /usr/local/bin/panacea panacea-sandbox:task14 token; echo "exit $?"
```

Expected: the fragment's seven lines exactly; then, after 20 seconds, `panacea token: no gateway token after 20s; this session's mint has not landed, and the cockpit shows why` and `exit 1`.

- [ ] **Step 9: Commit**

```bash
git add sandboxd/cmd/panacea/main.go sandboxd/cmd/panacea/main_test.go sandboxd/cmd/panacea/fragment_test.go images/sandbox/managed-settings.gateway.json
git commit -F <message file> -- sandboxd/cmd/panacea/main.go sandboxd/cmd/panacea/main_test.go sandboxd/cmd/panacea/fragment_test.go images/sandbox/managed-settings.gateway.json images/sandbox/Dockerfile
```

Message: `feat(sandbox): panacea token and the gateway managed-settings drop-in (phase 6 task 14)`, a body naming decisions (a) to (d) and the TTL change, and the attribution line.

**Acceptance:** step 6 prints `ok` and `11`; step 7's four edits each fail the named tests; step 8 prints nothing, then one package, then the fragment, the timeout line and `exit 1`.

**What this task does not claim:**

- **That Claude Code reads a drop-in with no `managed-settings.json` beside it.** The docs describe the directory as optional and merged with the file; the case of the directory alone is not stated. Task 23 runs `/status` in the image and records what it shows. If the drop-in alone is ignored, the Dockerfile also installs the fragment as `/etc/claude-code/managed-settings.json` until S6.6's mechanism owns that file.
- **That the helper is re-run at the TTL, or on a `401`.** The docs say five minutes by default and that the variable changes it; task 23 observes the interval. Re-running on a `401` is referred to (*"the other cases in which Claude Code re-runs the helper"*) and not read here.
- **That 20 seconds is inside the CLI's helper timeout.** Decision (d).
- **That no process in the sandbox can read the token.** Any process running as `dev` can run `panacea token`, which is why the token is pod-bound (task 10) and one-hour. The helper keeps it out of the **environment**, which every test suite and `postinstall` inherits (`05-design.md:207-208`); it does not keep it from the user's own code.
- **That the launch environment is clean.** `ANTHROPIC_API_KEY` or `ANTHROPIC_AUTH_TOKEN` in the environment outranks the helper (the docs' precedence list). Their absence is task 15's test locally and ask P8-1's in the cloud; the fragment cannot unset a variable.
- **The rest of the product's managed settings.** The `Bash(panacea *)` allow rule, the per-class permission modes and `allowManagedHooksOnly` (`05-design.md:243-248`) belong to the base file OD-23's mechanism will own.

**Gaps against the skeleton, for the orchestrator:**

1. **The helper TTL is `120000`, not the skeleton's `300000`** (step 2 explains): a 5-minute TTL equals task 07's 5-minute `Grace`, so a request at the refresh boundary could present a revoked token. Either this value stays below `Grace`, or task 07 raises `Grace`; the test here pins the inequality.
2. **S6.6 inherits a constraint.** The base `managed-settings.json` must not set `forceLoginOrgUUID`, or `forceLoginMethod` to `claudeai` or `console`: the docs block `apiKeyHelper` at startup under either. `05-design.md:249-250` already leaves `forceLoginMethod` unset; `forceLoginOrgUUID` is new, and a file derived from KPMG's tenant policy may carry it.
3. The fragment is a **drop-in** at `/etc/claude-code/managed-settings.d/50-panacea-gateway.json`, not a file "at the managed-settings path" (decision (a)); the path is now verified.
4. The "JSON test beside the fragment" lives in `sandboxd/cmd/panacea/fragment_test.go`, because `images/` has no Go module and the repository has no JSON test runner.
