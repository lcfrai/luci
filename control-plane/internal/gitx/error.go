package gitx

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaxSentence caps what the cockpit is shown of git's stderr.
const MaxSentence = 2048

// Error is a git call that did not answer.
type Error struct {
	Args     []string // the argv after "git"; Env is never here
	Dir      string
	ExitCode int    // -1 when git did not start or the timeout killed it
	Stderr   string // trimmed, capped at MaxSentence (the tail is kept: git's fatal line is last)
	TimedOut bool
	// NotStarted is true when no git code ran: the executable was missing, or Windows' loader
	// failed before git's main (0xC0000142, STATUS_DLL_INIT_FAILED; carried t01-D14).
	NotStarted bool
	Err        error
}

// Error is "git <args> in <dir>: <exit or timeout>: <Sentence()>".
func (e *Error) Error() string {
	return "git " + strings.Join(e.Args, " ") + " in " + e.Dir + ": " + e.how() + ": " + e.Sentence()
}

func (e *Error) how() string {
	switch {
	case e.TimedOut:
		return "timed out"
	case e.NotStarted:
		return "could not start"
	default:
		return "exit " + strconv.Itoa(e.ExitCode)
	}
}

// Unwrap is the underlying cause: the context's error on a timeout, the start error, or the
// *exec.ExitError.
func (e *Error) Unwrap() error { return e.Err }

// Sentence is git's own words: the last stderr line starting "fatal:", "error:" or
// "remote:", else the last non-empty stderr line, else "git exited <code>" ("git timed out"
// on a timeout, "git could not start" when no git code ran). Capped at MaxSentence bytes on a
// UTF-8 boundary. The cockpit renders it verbatim (details.gitSays).
func (e *Error) Sentence() string {
	var last, lastCause string
	for _, line := range strings.Split(e.Stderr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		last = line
		if strings.HasPrefix(line, "fatal:") || strings.HasPrefix(line, "error:") ||
			strings.HasPrefix(line, "remote:") {
			lastCause = line
		}
	}
	s := lastCause
	if s == "" {
		s = last
	}
	if s == "" {
		switch {
		case e.TimedOut:
			s = "git timed out"
		case e.NotStarted:
			s = "git could not start"
		default:
			s = "git exited " + strconv.Itoa(e.ExitCode)
		}
	}
	return capHead(s, MaxSentence)
}

// AsError is errors.As for *Error.
func AsError(err error) (*Error, bool) {
	var ge *Error
	ok := errors.As(err, &ge)
	return ge, ok
}

// capHead keeps at most n bytes of s from the front, cut on a rune boundary.
func capHead(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// capTail keeps at most n bytes of s from the end, cut on a rune boundary.
func capTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := len(s) - n
	for cut < len(s) && !utf8.RuneStart(s[cut]) {
		cut++
	}
	return s[cut:]
}
