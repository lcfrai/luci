package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

// This is the pinned OpenCode 1.18.34 read-only session/message API, reached
// only from inside the owned sandbox. No transcript, prompt, argument, result,
// provider key, or arbitrary upstream response crosses the sandbox boundary.
const nativeTurnMaxBytes = 1 << 20

type nativeTurn struct {
	State     string `json:"state"`
	ToolClass string `json:"toolClass"`
	TurnID    string `json:"turnId"`
}

type upstreamSession struct {
	ID string `json:"id"`
}

type upstreamMessage struct {
	Info struct {
		ID     string `json:"id"`
		Role   string `json:"role"`
		Finish string `json:"finish"`
		Time   struct {
			Completed int64 `json:"completed"`
		} `json:"time"`
		Error json.RawMessage `json:"error"`
	} `json:"info"`
	Parts []struct {
		Type  string `json:"type"`
		Tool  string `json:"tool"`
		State struct {
			Status string `json:"status"`
		} `json:"state"`
	} `json:"parts"`
}

func nativeOpenCodeCommand(command string, args []string) bool {
	if command != "opencode" {
		return false
	}
	host, port := false, false
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--hostname" && args[i+1] == "127.0.0.1" {
			host = true
		}
		if args[i] == "--port" && args[i+1] == "4096" {
			port = true
		}
	}
	return host && port
}

func (s *Server) nativeTurn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.authorised(r) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.URL.RawQuery != "" || r.URL.Fragment != "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	client := &http.Client{Timeout: 300 * time.Millisecond, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var sessions []upstreamSession
	if err := s.nativeRead(r, client, "/session?limit=2", &sessions); err != nil || len(sessions) != 1 || !nativeID(sessions[0].ID) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	var messages []upstreamMessage
	// The pinned MessageV2.page endpoint returns newest first. A bounded page
	// suffices only when it includes this turn's user message; otherwise the
	// classifier refuses to infer a complete turn.
	if err := s.nativeRead(r, client, "/session/"+sessions[0].ID+"/message?limit=64", &messages); err != nil || len(messages) == 0 || len(messages) > 64 {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	slices.Reverse(messages)
	turn, ok := classifyNativeTurn(messages, s.startedAt)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(turn)
}

func nativeID(id string) bool {
	if len(id) < 4 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if c < 'a' || c > 'z' {
			if c < 'A' || c > 'Z' {
				if c < '0' || c > '9' {
					if c != '_' && c != '-' {
						return false
					}
				}
			}
		}
	}
	return true
}

func (s *Server) nativeRead(r *http.Request, client *http.Client, path string, target any) error {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, s.nativeURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("opencode:"+s.cfg.AttachToken)))
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength > nativeTurnMaxBytes {
		return errors.New("native read unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, nativeTurnMaxBytes+1))
	if err != nil || len(data) > nativeTurnMaxBytes {
		return errors.New("native read exceeds bound")
	}
	if err := json.Unmarshal(data, target); err != nil || strings.TrimSpace(string(data)) == "null" {
		return errors.New("invalid native read")
	}
	return nil
}

// A completed assistant message is authoritative only for its own turn. It is
// never evidence of a source candidate, passing checks or accepted changes.
// Unknown tools and even a completed bash tool keep effects unverified.
func classifyNativeTurn(messages []upstreamMessage, startedAt int64) (nativeTurn, bool) {
	var lastUser int = -1
	for i := range messages {
		if messages[i].Info.Role == "user" {
			lastUser = i
		}
	}
	if lastUser < 0 || lastUser == len(messages)-1 || len(messages)-lastUser > 64 {
		return nativeTurn{}, false
	}
	last := messages[len(messages)-1]
	if last.Info.Role != "assistant" || last.Info.Finish != "stop" || last.Info.Time.Completed < startedAt || !nativeID(last.Info.ID) || len(last.Info.Error) > 0 && string(last.Info.Error) != "null" {
		return nativeTurn{}, false
	}
	toolClass := "none"
	for _, msg := range messages[lastUser+1:] {
		if msg.Info.Role != "assistant" || len(msg.Info.Error) > 0 && string(msg.Info.Error) != "null" {
			return nativeTurn{}, false
		}
		for _, part := range msg.Parts {
			switch part.Type {
			case "text", "reasoning", "step-start", "step-finish", "compaction", "retry":
				// Content is deliberately ignored. These parts are not native
				// tool effects, and cannot create a candidate or passing check.
			case "tool":
				if part.State.Status != "completed" && part.State.Status != "error" {
					return nativeTurn{}, false
				}
				if part.Tool != "read" && part.Tool != "glob" && part.Tool != "grep" && part.Tool != "list" {
					toolClass = "effects-unverified"
				} else if toolClass == "none" {
					toolClass = "reads-only"
				}
			default:
				// Patch/snapshot/subtask and future provider parts cannot be
				// treated as a read-only observation.
				toolClass = "effects-unverified"
			}
		}
	}
	return nativeTurn{State: "finished", ToolClass: toolClass, TurnID: last.Info.ID}, true
}
