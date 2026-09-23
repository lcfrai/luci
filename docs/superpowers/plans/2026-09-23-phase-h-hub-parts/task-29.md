### Task 29: `credstore` — GitHub, Azure DevOps and Panacea tokens over `Secrets`; phase 3's `Store` satisfied (H-1)

**Lane C. Depends on: 28. About 2.5 hours. Contract task C1 (phase 3's H-1; phase 7's token).**

The typed layer of seam **S-CRED**. Phase 3 asks for one interface — `credstore.Store`,
`Credential(ctx, Host) (Credential, error)` (phase 3 task 13) — and this task satisfies it for
GitHub with `Local`, over task 28's `Secrets`. Azure DevOps is satisfied first by `LocalDevOps`,
the user's pasted PAT per organisation (review I10: OD-4 and OD-6 need an ADO credential in this
store from v1, and the Entra path waits on ADR-H1 Q5), then by task 31's `entraauth.DevOps` (the
Entra sign-in, if Q5 is yes). Phase 7's `pnc_` token gets its own accessor, `PanaceaTokens`.
Task 39 is the write path for all three pasted tokens; nothing else writes them.

**The package is shared with phase 3.** Phase 3 task 13 creates `credstore/credstore.go` (`Host`,
`Credential`, `Store`, `None`, `Chain`). If it has landed, this task only adds its own files. If
not, this task writes **phase 3's file verbatim** from the contract below — the same text phase 3's
part holds — so whichever lands first, the other is a no-op merge. One comment differs, as contract
§9.1 names: `Credential.Source` is `"credential-manager" | "entra" | "git-credential-helper"`, not
`"hub"`, because the Hub server holds no git credential.

**Files:**
- `control-plane/internal/credstore/credstore.go` + `credstore_test.go` — **phase 3's**; only if absent (new)
- `control-plane/internal/credstore/tokens.go` — the blob format, `GitHubTokens`, `DevOpsTokens`, `PanaceaTokens`, `Inventory` (new)
- `control-plane/internal/credstore/local.go` — `Local` and `LocalDevOps`, phase 3's `Store` for GitHub and for an Azure DevOps PAT (new)
- `control-plane/internal/credstore/{tokens_test.go,local_test.go}` (new)

#### Contract — phase 3's file, verbatim (phase 3 task 13 `:30-92`)

```go
// credstore.go — phase 3's file. Its package comment is dropped in favour of task 28's
// (secrets.go): one package comment per package.
package credstore

import (
	"context"
	"errors"
	"time"
)

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
	Source    string    // "credential-manager" | "entra" | "git-credential-helper"   (Hub contract §9.1)
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

(The method bodies are phase 3's to write; this plan fixes their behaviour through phase 3's tests,
listed below.)

#### Contract — this task's files, complete

```go
// control-plane/internal/credstore/tokens.go
package credstore

// StoredToken is a pasted token as the store keeps it. Its String and GoString
// redact the token; it has no JSON form outside this file.
type StoredToken struct {
	Token   string
	Subject string    // for display: a GitHub login, a Panacea user, as the user typed or the host said
	SavedAt time.Time
}

func (t StoredToken) String() string   // "credstore.StoredToken{<Subject>, saved <time>, token redacted}"
func (t StoredToken) GoString() string // the same

// The blob is {"v":1,"token":"…","subject":"…","savedAt":"<RFC 3339>"} — one
// blob, never chunked: a GitHub, Azure DevOps or pnc_ token is well under 2,560 bytes, and
// Put refuses one that is not (ErrTooLarge).

// GitHubTokens are GitHub tokens by owner; owner "" is the token for every owner.
type GitHubTokens struct{ Secrets Secrets }

func (g GitHubTokens) Get(owner string) (StoredToken, error) // ErrNotFound
func (g GitHubTokens) Put(owner string, t StoredToken) error
func (g GitHubTokens) Delete(owner string) error

// PanaceaTokens are phase 7's pnc_ tokens by API host (OD-13: phase 7 calls
// Panacea's API as a client; the token is the user's own).
type PanaceaTokens struct{ Secrets Secrets }

func (p PanaceaTokens) Get(apiHost string) (StoredToken, error) // ErrNotFound
func (p PanaceaTokens) Put(apiHost string, t StoredToken) error
func (p PanaceaTokens) Delete(apiHost string) error

// DevOpsTokens are Azure DevOps personal access tokens by organisation (review
// I10), under AzureDevOpsTarget(org). The local path OD-4 and OD-6 need while
// ADR-H1 Q5 is unanswered: the store holds the user's ADO credential from v1,
// with no Entra consent for Azure DevOps.
type DevOpsTokens struct{ Secrets Secrets }

func (d DevOpsTokens) Get(org string) (StoredToken, error) // ErrNotFound
func (d DevOpsTokens) Put(org string, t StoredToken) error
func (d DevOpsTokens) Delete(org string) error

// Entry describes one stored credential WITHOUT its secret, for GET /v1/credentials (task 39).
type Entry struct {
	Kind    string    // "github" | "azure-devops" | "panacea" | "entra"
	Scope   string    // the owner ("*" for all), the ADO organisation, the API host, or the tenant id
	Subject string
	SavedAt time.Time
}

// Inventory lists every entry under TargetPrefix. Entra appears once per
// account record — never once per refresh chunk or per refresh generation
// (task 28's …/refresh/<gen>/<i>) — and its Subject is the
// account's username. It reads blobs to find subjects; it returns no secret.
// Kind comes from the target's namespace: PanaceaApp/git/github/ → "github",
// PanaceaApp/git/azure-devops/ → "azure-devops" (review I10), PanaceaApp/panacea/
// → "panacea", an Entra account record → "entra"; Scope is the last part.
func Inventory(s Secrets) ([]Entry, error)
```

```go
// control-plane/internal/credstore/local.go
package credstore

// Local implements Store for GitHub over this machine's store (H-1). For a host
// that is not GitHub on github.com it answers ErrNoCredential, so a Chain moves
// on — for Azure DevOps to LocalDevOps (a pasted PAT, below), then to
// entraauth.DevOps (task 31), then to GitHelper (phase 3).
// It looks up the owner's token first, then the every-owner token.
type Local struct{ Secrets Secrets }

func (l Local) Credential(ctx context.Context, h Host) (Credential, error)
// → Credential{Token, Scheme: Bearer, Subject: the stored subject,
//               Source: "credential-manager"} — ExpiresAt zero: a pasted token's
//               expiry is the host's to enforce

// LocalDevOps implements Store for Azure DevOps over this machine's store (review
// I10): the user's pasted PAT for the host's organisation. For a host that is not
// Azure DevOps, or whose Org is "" (a PAT is per organisation; there is no
// wildcard), or whose organisation has no PAT, it answers ErrNoCredential, so a
// Chain moves on to entraauth.DevOps. It looks up strings.ToLower(h.Org): ADO
// organisation names are case-insensitive, and task 39 stores them lowercased.
type LocalDevOps struct{ Secrets Secrets }

func (l LocalDevOps) Credential(ctx context.Context, h Host) (Credential, error)
// → Credential{Token, Scheme: BasicPAT, Subject: the stored subject,
//               Source: "credential-manager"} — ExpiresAt zero, as for Local
```

**The chain phase 3 wires** (phase 3 task 13 `:177-185` and `:222`, with the Hub's pieces named):
`Credentials = credstore.Chain{credstore.Local{Secrets: s}, credstore.LocalDevOps{Secrets: s},
entraauth.DevOps{Tokens: auth}, credstore.GitHelper{…}}` for PR creation, and `HubCredentials =
credstore.Chain{credstore.Local{…}, credstore.LocalDevOps{…}, entraauth.DevOps{…}}` for push
(review I10: `Local → LocalDevOps → entraauth.DevOps → GitHelper`). `Local` answers
`ErrNoCredential` for Azure DevOps, `LocalDevOps` for GitHub and for an organisation with no PAT,
and `entraauth.DevOps` for GitHub and when Q5 is no, so the order is safe either way. A pasted PAT
wins over the Entra token for its organisation: the user put it there on purpose. When Q5 flips
to yes, a user with no PAT gets the Entra path with no change to the write path; one who pasted a
PAT keeps it until they delete it (`DELETE /v1/credentials/azure-devops?scope=<org>`, task 39).
**Checked 2026-09-24** in the phase-3 worktree's task 13: its deps block (`:177-185`) has
`Credentials` and `HubCredentials` as single `credstore.Store` values, `credstore.None{}` until the
Hub, and Step 3 (`:222`) builds `Credentials = credstore.Chain{hub, credstore.GitHelper{…}}`. So
phase 3 takes whatever `hub` store this plan hands it; **this plan names the chain, phase 3 wires
it** (a sibling ask, recorded in the review's "Fix batch b04" section). Also asked there: phase 3's
`PushAuth` (`:187-190`) names what `ErrNoCredential` does and nothing else, so an
`ErrScopeNeedsConsent` or `ErrEntraUnavailable` must fail the push with that error, not fall back
to the zero `Auth` and the machine's helper.

#### Tests to write first

If this task writes phase 3's file, it writes phase 3's four tests for it too, verbatim from phase 3
task 13: `TestCredentialNeverPrintsItsToken`, `TestNoneHasNothing`,
`TestChainSkipsNoCredentialAndStopsOnFailure` (the fourth, `TestGitHelper…`, belongs to
`githelper.go`, which stays phase 3's). Then, over `Memory()`:

| Test | Asserts | Mutant it kills |
|---|---|---|
| `TestStoredTokenRedacts` | `%v`, `%+v`, `%#v`, `%s` and `slog` of a `StoredToken` never contain `s3cr3t` | a token in a log |
| `TestGitHubTokensRoundTrip` | put for `lcfrai` and for `""`; each reads back its own; delete removes one only | owner tokens overwriting each other |
| `TestPanaceaTokensArePerHost` | tokens for two hosts are independent; an absent host → `ErrNotFound` | a pnc_ token sent to the wrong Panacea |
| `TestLocalTakesTheOwnersTokenFirst` | owner token and wildcard token stored; `Host{GitHub, "github.com", "lcfrai"}` → the owner's; another owner → the wildcard's | a wildcard shadowing an owner-scoped token |
| `TestLocalIsGitHubOnly` | Azure DevOps, and GitHub Enterprise (`github.example.com`) → `ErrNoCredential` | a GitHub token sent to another host |
| `TestLocalSaysWhereItCameFrom` | `Source == "credential-manager"`, `Scheme == Bearer` | contract §9.1's named difference lost |
| `TestDevOpsTokensArePerOrganisation` | PATs for `kpmg-au` and `lcfr` are independent; an absent organisation → `ErrNotFound`; `DevOpsTokens.Put("")` panics through the builder (review I10) | an ADO PAT sent to another organisation |
| `TestLocalDevOpsServesAPAT` | a PAT stored for `kpmgcomau`: `Host{AzureDevOps, "dev.azure.com", "KPMGcomau"}` and `Host{AzureDevOps, "kpmgcomau.visualstudio.com", "kpmgcomau"}` → it, `Scheme == BasicPAT`, `Source == "credential-manager"`; another organisation, `Org ""`, and `Host{GitHub, "github.com", "kpmgcomau"}` → `ErrNoCredential`; a `Chain{LocalDevOps, <a store that fails>}` with the PAT stored → the PAT, and the failing store is never asked (review I10) | a PAT presented as a bearer token (ADO answers 401); an ADO PAT sent to GitHub; a case-sensitive lookup that misses the user's own PAT |
| `TestInventoryHoldsNoSecret` | one GitHub, one Azure DevOps PAT (kind `azure-devops`, scope the organisation), one Panacea, one Entra account with 3 chunks in its named generation plus a stray second generation (a rotation mid-way, task 28) → 4 entries; the formatted list contains no token or chunk bytes | a listing route that leaks the secret it lists |
| `TestAnOversizeTokenIsRefused` | a 3,000-byte token → `ErrTooLarge` | a truncated token that fails at the host with a confusing 401 |

#### Steps

- [ ] **Step 1: Preflight.** `ls control-plane/internal/credstore/credstore.go` on this lane's base,
  and read phase 3's task 13 part (sibling `feat/phase-3-git`, read only). Write phase 3's file
  only if absent; record which case applied in the commit message.
- [ ] **Step 2:** Tests; they fail. Then `tokens.go`, `local.go`.
- [ ] **Step 3:** Commit by pathspec.

#### Acceptance

```bash
go test ./control-plane/internal/credstore/... -race -count=1 -v 2>&1 | grep -c "^--- PASS"
# on Linux: 18 (task 28's 8 — TestMemoryConforms, six more, TestWinCredUnsupportedOffWindows — + these 10) when phase 3's file was already there; 21 when this task wrote it (+ its 3)
go vet ./control-plane/internal/credstore/...                                                   # no output
git grep -n "Source" -- control-plane/internal/credstore/credstore.go | grep -c '"hub"'          # 0
```

#### What this task does not claim

- That a GitHub OAuth app exists: v1 takes a pasted fine-grained token (open question Q9).
- That GitHub accepts a user's token in the `x-access-token` basic form: phase 3 task 13's claim,
  checked in its manual run.
- Any HTTP route: task 39.
