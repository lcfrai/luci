### Task 13: The credential-store seam (Hub ask H-1), the git-helper fallback, remote recognition, and push with a stored token

OD-6 puts the user's tokens in Windows Credential Manager, in a store **the Hub (Phase H) builds**,
and says the same store holds the Azure DevOps and GitHub tokens OD-4 needs. **No Hub contract
existed on 2026-09-23** (`.claude/worktrees/hub/docs/superpowers/contracts/` held only the two phase
2 contracts), so this task declares the minimal interface phase 3 needs, `credstore.Store`, marks
it as the Hub's to satisfy, and builds two implementations of its own that need no Hub:
`credstore.None` (nothing stored) and, per open question Q1's recommendation, `credstore.GitHelper`,
which asks the machine's own git credential helper for the user's own token, non-interactively.

It also builds the one thing both push and PR creation need first: recognising a project's
`origin` as a GitHub or Azure DevOps repository. The forms are taken from this programme's own
remotes, read on 2026-09-23: `https://github.com/<owner>/<repo>` and
`https://kpmgcomau.visualstudio.com/<project>/_git/<repo>`.

**Two rules no implementation may break:** a token is never written to disk by phase 3, never on
a command line, never logged, never in an error, never returned over HTTP; and every token is the
signed-in user's own (never pooled, OD-6).

**Files:**
- `control-plane/internal/credstore/credstore.go` + test — `Host`, `Credential`, `Store`, `None`, `Chain` (new)
- `control-plane/internal/credstore/githelper.go` + test — `GitHelper` (new)
- `control-plane/internal/githost/remote.go` + test — `Kind`, `Remote`, `ParseRemote`, `RepoKey` (new)
- `control-plane/internal/githost/header.go` + test — `GitAuthHeader` (new)
- `control-plane/internal/remote/push.go` — no change; `cmd/api/main.go` builds `PushAuth` (p2)
- `control-plane/internal/api/gitdeps.go` — `// ── task 13 ──`

#### Interfaces — complete

```go
// Package credstore is the seam between phase 3 and the Hub's credential store (OD-6; Hub ask
// H-1). The Hub satisfies Store over Windows Credential Manager, per signed-in user. Until it
// does, None and GitHelper are the implementations.
package credstore

type HostKind string

const (
	GitHub      HostKind = "github"
	AzureDevOps HostKind = "azure-devops"
)

// Host names the account a credential is for.
type Host struct {
	Kind HostKind
	// Authority is the git remote's host, lowercased: "github.com", "dev.azure.com",
	// "kpmgcomau.visualstudio.com".
	Authority string
	// Org is the GitHub owner or the Azure DevOps organisation ("kpmgcomau" for both
	// dev.azure.com/kpmgcomau and kpmgcomau.visualstudio.com). "" when a token covers the host.
	Org string
}

type Scheme string

const (
	Bearer   Scheme = "bearer"    // an OAuth access token: Entra for Azure DevOps; a GitHub OAuth, app or fine-grained token
	BasicPAT Scheme = "basic-pat" // an Azure DevOps personal access token, presented as HTTP basic
)

// Credential is one user's token for one host. It cannot be printed or serialised with its
// token: String and GoString redact, MarshalJSON refuses.
type Credential struct {
	Token     string
	Scheme    Scheme
	Subject   string    // who it authenticates as, for display: "dmackie1@kpmg.com.au", "octocat"
	ExpiresAt time.Time // zero when the source does not say
	Source    string    // "hub" | "git-credential-helper"
}

func (c Credential) String() string   // "credstore.Credential{<Subject>, <Scheme>, <Source>, token redacted}"
func (c Credential) GoString() string // the same
func (c Credential) MarshalJSON() ([]byte, error) // always an error: "a credential is never serialised"

var ErrNoCredential = errors.New("credstore: no credential for that host")

// Store is Hub ask H-1: the signed-in user's own credential for a host.
type Store interface {
	Credential(ctx context.Context, h Host) (Credential, error)
}

// None is a store with nothing in it.
type None struct{}

func (None) Credential(context.Context, Host) (Credential, error) { return Credential{}, ErrNoCredential }

// Chain asks each store in order. ErrNoCredential moves to the next; any other error stops the
// chain and is returned, because "the Hub is down" must not silently become "use the other one".
type Chain []Store

func (c Chain) Credential(ctx context.Context, h Host) (Credential, error)
```

```go
package credstore

// GitHelper asks the machine's configured git credential helper — Git Credential Manager on
// the owner's laptop — for the user's own credential (open question Q1). It runs exactly
//   git credential fill
// with stdin "protocol=https\nhost=<Authority>\npath=<the remote's path>\n\n", as gitx.Local
// (so GCM_INTERACTIVE=never and GIT_TERMINAL_PROMPT=0 apply: it never opens a window or asks).
// It never runs `credential approve` or `credential reject`: the user's store is read, not changed.
// A non-zero exit, or output without "password=", is ErrNoCredential.
// Scheme: Bearer for GitHub; for Azure DevOps, Bearer when the secret has JWT shape (three
// base64url segments, first starting "eyJ"), else BasicPAT. Subject is "username=".
type GitHelper struct {
	Runner gitx.Runner
	Dir    string // any directory; the project's, so a repo-local helper config applies
	Path   string // the remote's path, e.g. "lcfrai/panaceaapp"; set by the caller per call
}

func (g GitHelper) Credential(ctx context.Context, h Host) (Credential, error)
```

```go
// Package githost recognises a project's remote and talks to its host's API (task 14).
package githost

type Kind = credstore.HostKind

// Remote is a recognised git remote.
type Remote struct {
	Kind      Kind
	URL       string // as `git remote get-url` printed it
	HTTPS     bool   // false for an SSH remote: push then uses the machine's SSH keys, never a token
	Authority string // lowercased: "github.com", "dev.azure.com", "kpmgcomau.visualstudio.com"
	Owner     string // GitHub owner, or Azure DevOps organisation
	Project   string // Azure DevOps project; "" for GitHub
	Repo      string // without ".git"
	Path      string // the credential path: "owner/repo", or "org/project/_git/repo"
}

// Host is the credstore key for this remote.
func (r Remote) Host() credstore.Host

// RepoKey is a machine-independent identity for the repository (Hub ask H-3):
// "github.com/<owner>/<repo>" or "azure-devops/<org>/<project>/<repo>", lowercased. Two
// teammates' clones of one repository have the same RepoKey and different project ids.
func (r Remote) RepoKey() string

var ErrUnknownHost = errors.New("githost: the remote is neither GitHub nor Azure DevOps")

// ParseRemote recognises, and nothing else:
//   https://github.com/<o>/<r>[.git]            https://<user>@github.com/<o>/<r>[.git]
//   git@github.com:<o>/<r>[.git]                ssh://git@github.com/<o>/<r>[.git]
//   https://dev.azure.com/<org>/<p>/_git/<r>    https://<org>@dev.azure.com/<org>/<p>/_git/<r>
//   https://<org>.visualstudio.com/[DefaultCollection/]<p>/_git/<r>
//   git@ssh.dev.azure.com:v3/<org>/<p>/<r>      <org>@vs-ssh.visualstudio.com:v3/<org>/<p>/<r>
// Percent-encoded project names ("My%20Project") are decoded. GitHub Enterprise Server and Azure
// DevOps Server are ErrUnknownHost in v1.
func ParseRemote(url string) (Remote, error)
```

```go
package githost

// GitAuthHeader is the http.extraHeader value that authenticates git over HTTPS as cred on
// this remote's host:
//   GitHub, any scheme:        "Authorization: Basic " + base64("x-access-token:" + token)
//   Azure DevOps, Bearer:      "Authorization: Bearer " + token
//   Azure DevOps, BasicPAT:    "Authorization: Basic " + base64(":" + token)
func GitAuthHeader(r Remote, cred credstore.Credential) string

// PushEnv is the remote.Auth for a push with cred: the header, scoped to this remote's
// origin URL so no other host ever receives it —
//   GIT_CONFIG_COUNT=1, GIT_CONFIG_KEY_0=http.https://<Authority>/.extraHeader, GIT_CONFIG_VALUE_0=<header>
// The token travels only in the child process's environment, never on its command line.
func PushEnv(r Remote, cred credstore.Credential) map[string]string
```

```go
// gitdeps.go
	// ── task 13 ──
	// Credentials is the chain PR creation asks: the Hub's store once it exists, then the
	// machine's git credential helper (Q1). Push asks only the Hub's (below), because git
	// already asks the machine's helper itself.
	Credentials credstore.Store
	HubCredentials credstore.Store // credstore.None{} until the Hub (gate G4)
```

`main.go` builds `PushAuth` (task 12's hook): read `origin`'s URL; `ParseRemote`; an SSH or
unrecognised remote → the zero `Auth`; else `HubCredentials.Credential(remote.Host())`;
`ErrNoCredential` → the zero `Auth` (the machine's helper, task 12's behaviour); a credential →
`remote.Auth{Env: githost.PushEnv(r, cred)}`.

#### The tests to write first

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestCredentialNeverPrintsItsToken` | `fmt.Sprintf` with `%v`, `%+v`, `%#v`, `%s` and `slog` of a Credential → no `s3cr3t`; `json.Marshal` → error | a token in a log line |
| `TestNoneHasNothing` | → `ErrNoCredential` | a default that invents a token |
| `TestChainSkipsNoCredentialAndStopsOnFailure` | [None, fixed] → fixed; [failing, fixed] → failing's error | "Hub down" silently falling through |
| `TestGitHelperReadsFill` | isolated config with `credential.helper=!f() { echo username=ada; echo password=gho_x; }; f` → `Token gho_x`, `Subject ada`, `Source git-credential-helper` | a parser that misses the secret |
| `TestGitHelperNothingStoredIsNoCredential` | no helper configured → `ErrNoCredential` within `LocalTimeout`, never a prompt | a hang on a terminal prompt |
| `TestGitHelperOnlyEverRunsFill` | Script runner: the argv is exactly `credential fill` | approving or rejecting the user's credentials |
| `TestGitHelperSchemes` | GitHub → Bearer; ADO JWT-shaped → Bearer; ADO other → BasicPAT | an ADO PAT sent as a bearer token |
| `TestParseRemoteTable` | every form in `ParseRemote`'s comment → the right fields; this programme's own two remotes exactly; `https://gitlab.com/a/b` and GHES → `ErrUnknownHost` | a remote misfiled as the other host |
| `TestRepoKeyIsMachineIndependent` | the HTTPS and SSH URLs of one repo → one RepoKey; case differences → one | two teammates' records never meeting in the Hub |
| `TestGitAuthHeaderPerHost` | the three header forms byte for byte | a header the host rejects |
| `TestPushEnvScopesTheHeaderToTheHost` | the key is `http.https://github.com/.extraHeader`, not `http.extraHeader` | a token sent to every host git talks to |
| `TestPushWithAStoredTokenSendsTheHeader` | an `httptest` smart-HTTP server records `Authorization` and answers 401; push with `PushEnv` → the recorded header equals `GitAuthHeader`; the returned error contains no token | the header never reaching git |
| `TestTokenIsNeverOnArgv` | Script runner over a push with `PushEnv` → no argv element contains the token | a token visible in the process list |
| `TestPushAuthAsksOnlyTheHubStore` | a GitHelper that fails the test if called; PushAuth with Hub `None` → zero Auth | the machine's token injected into a push git would have made anyway |

#### Steps

- [ ] **Step 1: Preflight.** Check again for a Hub contract:
  `ls /c/Users/au-dev-dmackie1/Projects/PanaceaApp/.claude/worktrees/hub/docs/superpowers/contracts/`.
  **If a Hub contract now exists and declares a credential store**, satisfy it instead: keep
  `credstore.Store` as phase 3's view and write a one-file adapter over the Hub's type, and record
  every difference in the commit message. Do not edit the Hub's worktree.
- [ ] **Step 2:** tests; then `credstore`, `githost/remote.go`, `githost/header.go`.
- [ ] **Step 3:** the deps block and `PushAuth` in `main.go`; `Credentials = credstore.Chain{hub, credstore.GitHelper{…}}`.
- [ ] **Step 4:** run; commit by pathspec.

#### Acceptance

```
go test ./control-plane/internal/credstore/ ./control-plane/internal/githost/ ./control-plane/internal/remote/ -race -count=1 → PASS
git grep -n "Token" -- control-plane/internal | grep -iv "_test.go" | grep -i "log\.\|slog\|Printf"   → no output
```

#### What this task does not claim

- **The header forms are from the hosts' documented behaviour, not run against either host**:
  GitHub's `x-access-token` basic form (documented for app installation tokens; that it accepts a
  user OAuth token the same way is inferred) and Azure DevOps' bearer `extraHeader` (documented for
  pipeline tokens; an Entra user token is inferred to work the same way). Task 23's manual checks
  run both once, on the owner's laptop.
- Whether GCM on the owner's laptop stores an Entra token or a PAT for
  `kpmgcomau.visualstudio.com` was **not checked**; `GitHelper`'s scheme detection handles both.
- The Hub's real store, per-user keying, expiry and refresh are H-1, not built here.
- The environment variable `GIT_CONFIG_COUNT` set for a push replaces any `GIT_CONFIG_COUNT` the
  user's own environment set for that one process. Stated, not handled.
