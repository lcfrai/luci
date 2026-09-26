package gitx

import (
	"errors"
	"strings"
)

// The trailer keys phase 3 writes. Readers match them exactly as written here.
const (
	TrailerSession     = "Panacea-Session"      // the hook, task 05
	TrailerLand        = "Panacea-Land"         // a land's worktree id, task 09
	TrailerBrief       = "Panacea-Brief"        // the brief key the human saw, task 09
	TrailerBriefDigest = "Panacea-Brief-Digest" // that revision's digest, task 09
	TrailerReverts     = "Panacea-Reverts"      // the land a revert takes back, task 10
	// TrailerBringIn marks task 09's merge-mode bring-in; its value is the upstream tip it
	// merged, a 40-hex sha, so Message's charset always accepts it (a branch name may not).
	TrailerBringIn = "Panacea-Bring-In"
)

// Trailer is one "Key: value" line.
type Trailer struct{ Key, Value string }

// ErrBadMessage is a subject or trailer value Message refuses.
var ErrBadMessage = errors.New("gitx: subject or trailer value is not writable")

const (
	maxSubject      = 200
	maxTrailerValue = 256
)

// Message builds "subject\n\nbody\n\nKey: value\n…\n". It refuses a subject with a newline
// or over 200 bytes, and a trailer value that is empty, over 256 bytes, or outside
// [A-Za-z0-9._:@/+=-]. That check is a charset, not a grammar: a brief key is copied byte for
// byte and never split (contract P5). Its purpose is that no value can start a second
// trailer line.
//
// Two refusals beyond that, for the same purpose: a key outside [A-Za-z0-9-] (a key with a
// colon or a newline would write a second trailer), and, when trailers is empty, a body whose
// last paragraph holds a "Panacea-…:" line, because git reads a message's last paragraph as its
// trailer block and would find a forged trailer there. With trailers given, Message's own
// paragraph is the last one, so the body may hold anything.
func Message(subject, body string, trailers []Trailer) (string, error) {
	if strings.ContainsAny(subject, "\r\n") || len(subject) > maxSubject {
		return "", ErrBadMessage
	}
	for _, t := range trailers {
		if !validKey(t.Key) || !validValue(t.Value) {
			return "", ErrBadMessage
		}
	}
	body = strings.Trim(body, "\n")
	if len(trailers) == 0 && lastParagraphForges(body) {
		return "", ErrBadMessage
	}
	var b strings.Builder
	b.WriteString(subject)
	b.WriteString("\n")
	if body != "" {
		b.WriteString("\n")
		b.WriteString(body)
		b.WriteString("\n")
	}
	if len(trailers) > 0 {
		b.WriteString("\n")
		for _, t := range trailers {
			b.WriteString(t.Key)
			b.WriteString(": ")
			b.WriteString(t.Value)
			b.WriteString("\n")
		}
	}
	return b.String(), nil
}

func validKey(k string) bool {
	if k == "" {
		return false
	}
	for i := 0; i < len(k); i++ {
		c := k[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func validValue(v string) bool {
	if v == "" || len(v) > maxTrailerValue {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
			strings.IndexByte("._:@/+=-", c) >= 0) {
			return false
		}
	}
	return true
}

// lastParagraphForges reports whether body's last paragraph holds a line git would read as a
// Panacea trailer: "panacea-" in any case at column 0, a token, optional blanks, then ':'.
func lastParagraphForges(body string) bool {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			return false // the paragraph above a blank line is not the trailer block
		}
		if len(line) < len("panacea-") || !strings.EqualFold(line[:len("panacea-")], "panacea-") {
			continue
		}
		rest := line[len("panacea-"):]
		j := 0
		for j < len(rest) && (validKey(rest[j:j+1])) {
			j++
		}
		if strings.HasPrefix(strings.TrimLeft(rest[j:], " \t"), ":") {
			return true
		}
	}
	return false
}

// Log formats use NUL between fields and RS between records, so a subject can hold anything.
const (
	FieldSep  = "\x00"
	RecordSep = "\x1e"
	ValueSep  = "\x1f"
)

// TrailerFormat is a --format fragment printing one trailer key's values joined by ValueSep.
func TrailerFormat(key string) string {
	return "%(trailers:key=" + key + ",valueonly,separator=%x1f)"
}
