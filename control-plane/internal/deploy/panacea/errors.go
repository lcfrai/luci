package panacea

import (
	"bytes"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
)

// maxMessage is the most of Panacea's sentence that reaches Error.Message (deploy.Error).
const maxMessage = 500

// readProblem returns Panacea's sentence from an error body. Panacea's error bodies are
// {"error":"<sentence>"} from three writers: writeError (handlers/helpers.go:41-43),
// writeAPIKeyError (middleware/api_key.go:396-400), and http.Error with a JSON literal
// (middleware/auth.go:250, middleware/rbac.go:398-400). The last arrives as text/plain, so the
// body is parsed as JSON whatever Content-Type says. It takes "error", falls back to "message"
// (models.APIResponse carries both), trims to 500 bytes on a rune boundary, and returns "" for
// a body that is not that JSON (an ingress HTML page, the router's "404 page not found").
func readProblem(body []byte) string {
	var fields map[string]json.RawMessage
	if json.Unmarshal(bytes.TrimSpace(body), &fields) != nil {
		return ""
	}
	for _, k := range []string{"error", "message"} {
		var s string
		if raw, ok := fields[k]; ok && json.Unmarshal(raw, &s) == nil {
			if s = strings.TrimSpace(s); s != "" {
				return trimSentence(s)
			}
		}
	}
	return ""
}

// trimSentence cuts s to maxMessage bytes without splitting a rune.
func trimSentence(s string) string {
	s = strings.ToValidUTF8(s, "�")
	if len(s) <= maxMessage {
		return s
	}
	i := maxMessage
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return strings.TrimSpace(s[:i])
}

// mapStatus is the status table, exactly:
//
//	3xx                → KindUnavailable, ""     (Version answers ErrSignInRedirect instead)
//	401                → KindUnauthorized, the sentence
//	403                → KindForbidden, the sentence
//	404                → KindNotFound, the sentence
//	409, 402           → KindConflict, the sentence (402: the deploy route's budget and risk
//	                     gates, handlers/projects.go:2856 and :2916-2918 — a refusal, never an outage)
//	400, 413, 422      → KindRejected, the sentence
//	429                → KindUnavailable, "", RetryAfter
//	500–599            → KindUnavailable, "" — never the sentence: Panacea appends err.Error()
//	                     to them (e.g. projects.go:1529). RetryAfter on 503 only.
//	any other          → KindUnavailable, ""
//
// Status is always the upstream status. The caller sets Cause.
func mapStatus(op string, status int, sentence string, retryAfter time.Duration) *deploy.Error {
	e := &deploy.Error{Kind: deploy.KindUnavailable, Op: op, Status: status}
	switch status {
	case http.StatusUnauthorized:
		e.Kind, e.Message = deploy.KindUnauthorized, sentence
	case http.StatusForbidden:
		e.Kind, e.Message = deploy.KindForbidden, sentence
	case http.StatusNotFound:
		e.Kind, e.Message = deploy.KindNotFound, sentence
	case http.StatusConflict, http.StatusPaymentRequired:
		e.Kind, e.Message = deploy.KindConflict, sentence
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		e.Kind, e.Message = deploy.KindRejected, sentence
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		e.RetryAfter = retryAfter
	}
	return e
}

// parseRetryAfter reads a Retry-After header: delay-seconds (at most int32) or an HTTP-date.
// Anything else, and a delay in the past, is 0.
func parseRetryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if n, err := strconv.ParseInt(v, 10, 32); err == nil {
		if n <= 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// pncToken matches anything shaped like a Panacea API key (apiKeyPrefix, middleware/api_key.go:19),
// so a body quoting a truncated or other key is redacted too. This comment names the prefix
// without quoting it: task 05's key gate (task-05.md:380-381) must match only its own prefix check.
var pncToken = regexp.MustCompile(`pnc_[A-Za-z0-9_\-]+`)

// redact removes the caller's token, and anything shaped like a pnc_ key, from an error body
// before it reaches Error.Message or the log.
func redact(body []byte, token string) []byte {
	if token != "" {
		body = bytes.ReplaceAll(body, []byte(token), []byte("[redacted]"))
	}
	return pncToken.ReplaceAll(body, []byte("pnc_[redacted]"))
}
