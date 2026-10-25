package proxy

import (
	"strconv"
	"strings"
)

// ProcessExit is a process observation, not task success or resource cleanup.
// Its fields are private: relay payloads can only create a value through the
// bounded closed decoder below; callers cannot supply arbitrary diagnostic text.
type ProcessExit struct {
	code    int
	hasCode bool
	signal  string
}

func (e ProcessExit) Known() bool { return e.hasCode || e.signal != "" }

func (e ProcessExit) Reason() string {
	if e.hasCode {
		return "process exited with code " + strconv.Itoa(e.code)
	}
	if e.signal != "" {
		return "process ended after " + e.signal + "; cause unconfirmed"
	}
	return "process exited"
}

// DecodeProcessExit accepts sandboxd's process Wait result in its typed Exit frame. Never
// parse stdout, general errors or provider messages as a lifecycle diagnostic.
func DecodeProcessExit(payload []byte) ProcessExit {
	if len(payload) == 0 || len(payload) > 64 {
		return ProcessExit{}
	}
	text := string(payload)
	if raw, ok := strings.CutPrefix(text, "exit status "); ok {
		if len(raw) < 1 || len(raw) > 3 || len(raw) > 1 && raw[0] == '0' {
			return ProcessExit{}
		}
		for _, digit := range raw {
			if digit < '0' || digit > '9' {
				return ProcessExit{}
			}
		}
		if code, err := strconv.Atoi(raw); err == nil && code <= 255 {
			return ProcessExit{code: code, hasCode: true}
		}
	}
	// Fixed Linux signal spellings produced by os/exec, including Go's optional
	// core-dump suffix. SIGKILL alone never establishes an out-of-memory cause.
	text = strings.TrimSuffix(text, " (core dumped)")
	var signal string
	switch text {
	case "signal: hangup":
		signal = "SIGHUP"
	case "signal: interrupt":
		signal = "SIGINT"
	case "signal: quit":
		signal = "SIGQUIT"
	case "signal: illegal instruction":
		signal = "SIGILL"
	case "signal: aborted":
		signal = "SIGABRT"
	case "signal: bus error":
		signal = "SIGBUS"
	case "signal: floating point exception":
		signal = "SIGFPE"
	case "signal: killed":
		signal = "SIGKILL"
	case "signal: segmentation fault":
		signal = "SIGSEGV"
	case "signal: broken pipe":
		signal = "SIGPIPE"
	case "signal: alarm clock":
		signal = "SIGALRM"
	case "signal: terminated":
		signal = "SIGTERM"
	default:
		return ProcessExit{}
	}
	return ProcessExit{signal: signal}
}
