package hubapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Id prefixes the Hub mints (contract §3.1). Disjoint from the control plane's
// s-, p-, b-, w-, m-.
const (
	PrefixOrg      = "org"
	PrefixUser     = "u"
	PrefixTeam     = "tm"
	PrefixInvite   = "iv"
	PrefixProject  = "pj"
	PrefixBrief    = "bf"
	PrefixApproval = "ap"
	PrefixLink     = "ln"
	PrefixRelease  = "rl"
	PrefixEvidence = "ev"
)

var wideHex = regexp.MustCompile(`^[0-9a-f]{16}$`)

// ValidID reports whether id is prefix + "-" + 16 lowercase hex.
func ValidID(prefix, id string) bool {
	rest, ok := strings.CutPrefix(id, prefix+"-")
	return ok && wideHex.MatchString(rest)
}

var (
	sessionIDRe = regexp.MustCompile(`^[a-z]{1,4}-[0-9a-f]{8,32}$`) // contract §7.8
	canvasIDRe  = regexp.MustCompile(`^[a-z]{2,4}-[0-9a-f]{4,32}$`) // contract §3.2
)

func ValidSessionID(id string) bool { return sessionIDRe.MatchString(id) }
func ValidCanvasID(id string) bool  { return canvasIDRe.MatchString(id) }

// ── keys (contract §3.2) ────────────────────────────────────────────────────

type KeyKind string

const (
	KeyBrief  KeyKind = "brief"
	KeyDesign KeyKind = "design"
)

var ErrInvalidKey = errors.New("hubapi: not a key in the Hub's grammar")

var (
	briefKeyRe  = regexp.MustCompile(`^brief:(bf-[0-9a-f]{16})@([1-9][0-9]{0,8})$`)
	designKeyRe = regexp.MustCompile(`^design:([a-z]{2,4}-[0-9a-f]{4,32})@([1-9][0-9]{0,8})$`)
)

// BriefKey names one immutable brief revision: brief:<briefId>@<revision>.
type BriefKey struct {
	BriefID  string
	Revision int // 1 … 999,999,999
}

func (k BriefKey) String() string { return "brief:" + k.BriefID + "@" + strconv.Itoa(k.Revision) }

func ParseBriefKey(s string) (BriefKey, error) {
	m := briefKeyRe.FindStringSubmatch(s)
	if m == nil {
		return BriefKey{}, ErrInvalidKey
	}
	n, _ := strconv.Atoi(m[2]) // the grammar bounds it to nine digits
	return BriefKey{BriefID: m[1], Revision: n}, nil
}

// DesignKey names one approved canvas revision: design:<canvasId>@<designRevision>.
type DesignKey struct {
	CanvasID string
	Revision int
}

func (k DesignKey) String() string { return "design:" + k.CanvasID + "@" + strconv.Itoa(k.Revision) }

func ParseDesignKey(s string) (DesignKey, error) {
	m := designKeyRe.FindStringSubmatch(s)
	if m == nil {
		return DesignKey{}, ErrInvalidKey
	}
	n, _ := strconv.Atoi(m[2])
	return DesignKey{CanvasID: m[1], Revision: n}, nil
}

// ParseKey classifies a key without keeping it. ErrInvalidKey for anything else.
func ParseKey(s string) (KeyKind, error) {
	switch {
	case briefKeyRe.MatchString(s):
		return KeyBrief, nil
	case designKeyRe.MatchString(s):
		return KeyDesign, nil
	}
	return "", ErrInvalidKey
}

// ── digests ─────────────────────────────────────────────────────────────────

// Digest is a SHA-256. Its wire form is "sha256:" + 64 lowercase hex; Postgres
// stores the 32 raw bytes (contract §10.2).
type Digest [32]byte

var ErrInvalidDigest = errors.New(`hubapi: a digest is "sha256:" and 64 lowercase hex characters`)

func DigestOf(b []byte) Digest { return sha256.Sum256(b) }

func (d Digest) String() string { return "sha256:" + hex.EncodeToString(d[:]) }

func (d Digest) Hex() string { return hex.EncodeToString(d[:]) }

// IsZero reports Digest{}: what encoding/json leaves in a Digest field that was
// absent or null. No input hashes to it (SHA-256 of nothing is e3b0c442…), so
// in a request it means "not sent"; RequireDigests refuses it.
func (d Digest) IsZero() bool { return d == Digest{} }

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func ParseDigest(s string) (Digest, error) {
	if !digestRe.MatchString(s) {
		return Digest{}, ErrInvalidDigest
	}
	var d Digest
	_, _ = hex.Decode(d[:], []byte(s[len("sha256:"):]))
	return d, nil
}

// ParseDigestHex accepts the bare 64-hex form a blob route's {sha} carries.
func ParseDigestHex(s string) (Digest, error) { return ParseDigest("sha256:" + s) }

func (d Digest) MarshalText() ([]byte, error) { return []byte(d.String()), nil }

// UnmarshalText takes the wire form only. Its refusal is a
// *json.UnmarshalTypeError because that is the one error encoding/json adds the
// field's path to; httpx.DecodeJSON then answers 400 bad_request naming the
// field. Any other error type comes back bare, and DecodeJSON answers
// invalid_json, which says the body is not one JSON object when it is
// (contract §4.2; §7.7 check 1). Task 03 departure; ParseDigest keeps
// ErrInvalidDigest.
func (d *Digest) UnmarshalText(b []byte) error {
	p, err := ParseDigest(string(b))
	if err != nil {
		return &json.UnmarshalTypeError{Value: `a string that is not "sha256:" and 64 lowercase hex`, Type: digestType}
	}
	*d = p
	return nil
}

// ── branch refs (contract §3.3) ─────────────────────────────────────────────

const BranchRefMaxBytes = 255

// ValidBranchRef reports whether ref is "" (project-scoped) or a full
// refs/heads/<name> that git check-ref-format would accept, by the rules that
// need no git: at most 255 bytes; a non-empty name after "refs/heads/"; no "..",
// no "@{", no "//"; no ASCII control character or DEL; none of space ~ ^ : ? * [ \;
// not ending in "/" or "."; no component beginning with "." or ending in
// ".lock". Those are check-ref-format's rules 1 to 8 and 10; rule 9, a lone
// "@", cannot follow "refs/heads/". One rule is the Hub's, not git's: the ref
// is UTF-8. It sits in the canonical envelope, which is UTF-8 only (contract
// §3.4 rule 2), and encoding/json rewrites a bad byte as U+FFFD, so a ref that
// is not UTF-8 would be stored as a ref git does not have. Contract §3.3 lists
// every one of these rules.
func ValidBranchRef(ref string) bool {
	if ref == "" {
		return true
	}
	name, ok := strings.CutPrefix(ref, "refs/heads/")
	if !ok || name == "" || len(ref) > BranchRefMaxBytes || !utf8.ValidString(ref) {
		return false
	}
	if strings.Contains(ref, "..") || strings.Contains(ref, "@{") || strings.Contains(ref, "//") {
		return false
	}
	if strings.HasSuffix(ref, "/") || strings.HasSuffix(ref, ".") {
		return false
	}
	for i := 0; i < len(ref); i++ {
		c := ref[i]
		if c < 0x20 || c == 0x7f || strings.IndexByte(" ~^:?*[\\", c) >= 0 {
			return false
		}
	}
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return false
		}
	}
	return true
}

// ── repository keys (contract §7.4, plan correction A1) ─────────────────────

type GitHost string

const (
	HostGitHub      GitHost = "github"
	HostAzureDevOps GitHost = "azure-devops"
)

// An Azure DevOps project or repository segment may hold spaces, so it is "not
// a slash" rather than a letter class; it also excludes the ASCII control
// characters and DEL, which no Azure DevOps name holds, so "…/repo\n" is not a
// second spelling of "…/repo" (RE2's [^/] alone matches a newline).
var (
	githubKeyRe = regexp.MustCompile(`^github\.com/[a-z0-9-]{1,39}/[a-z0-9._-]{1,100}$`)
	adoKeyRe    = regexp.MustCompile(`^azure-devops/[a-z0-9-]{1,50}/[^/\x00-\x1f\x7f]{1,64}/[^/\x00-\x1f\x7f]{1,64}$`)
)

// ValidRepoKey reports whether key is phase 3's githost.RepoKey() for that host,
// in its canonical, lowercased form: "github.com/<owner>/<repo>" or
// "azure-devops/<org>/<project>/<repo>" (phase 3's plan, task-13.md:140-143 on
// feat/phase-3-git; its review I10). A key that is not already lowercase is
// refused, never folded: the Hub stores one spelling of a repository or it
// links two projects to it. A key that is not UTF-8 fails the same check,
// because strings.ToLower rewrites an invalid byte as U+FFFD.
func ValidRepoKey(host GitHost, key string) bool {
	if key != strings.ToLower(key) {
		return false
	}
	switch host {
	case HostGitHub:
		return githubKeyRe.MatchString(key)
	case HostAzureDevOps:
		return adoKeyRe.MatchString(key)
	}
	return false
}

// HostOfRepoKey says which host a valid key belongs to.
func HostOfRepoKey(key string) (GitHost, bool) {
	for _, h := range []GitHost{HostGitHub, HostAzureDevOps} {
		if ValidRepoKey(h, key) {
			return h, true
		}
	}
	return "", false
}
