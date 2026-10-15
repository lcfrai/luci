package main

import "encoding/json"

func validClosedDependencyLock(data []byte) bool {
	var closure struct {
		State      string            `json:"state"`
		Unresolved []json.RawMessage `json:"unresolved"`
	}
	return len(data) > 0 && len(data) <= 2<<20 && uniqueJSON(data) == nil && json.Unmarshal(data, &closure) == nil && closure.State == "closed-reviewed" && closure.Unresolved != nil && len(closure.Unresolved) == 0
}
