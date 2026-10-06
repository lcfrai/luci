package management

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type managedTeamBinding struct {
	Version   int    `json:"version"`
	TeamID    string `json:"teamId"`
	CreatedAt string `json:"createdAt"`
}

// bindManagedTeam commissions one installation for one existing team. A saved
// binding survives an omitted environment setting and cannot be changed through
// an API or by pointing the setting at a different team. This prevents personal
// workspaces from bypassing team entitlements; it does not attest host isolation.
func (g *Gateway) bindManagedTeam(requested string) error {
	if requested != "" && !teamIDPattern.MatchString(requested) {
		return errors.New("LUCI_MANAGED_TEAM_ID must identify an existing team")
	}
	path := filepath.Join(g.dir, "managed-team.json")
	info, err := os.Lstat(path)
	var binding managedTeamBinding
	if err == nil {
		if !info.Mode().IsRegular() || info.Size() > 4096 {
			return errors.New("invalid private managed team binding")
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil || json.Unmarshal(data, &binding) != nil || binding.Version != 1 || !teamIDPattern.MatchString(binding.TeamID) {
			return errors.New("invalid private managed team binding")
		}
		if _, err := time.Parse(time.RFC3339Nano, binding.CreatedAt); err != nil {
			return errors.New("invalid private managed team binding timestamp")
		}
		if requested != "" && requested != binding.TeamID {
			return errors.New("this installation is already bound to a different managed team; use a separate installation for another team")
		}
		requested = binding.TeamID
		if err := privatePath(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read managed team binding: %w", err)
	}
	if requested == "" {
		return nil
	}
	if _, exists := g.state.Teams[requested]; !exists || len(g.state.Teams) != 1 {
		return errors.New("a managed installation must contain exactly its bound team; bootstrap an owner and create that one team before binding a dedicated installation")
	}
	if binding.TeamID == "" {
		binding = managedTeamBinding{Version: 1, TeamID: requested, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		data, _ := json.Marshal(binding)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("reserve private managed team binding: %w", err)
		}
		if err = privatePath(path); err == nil {
			_, err = file.Write(data)
		}
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			// Preserve a partial marker on failure: a subsequent startup must
			// fail closed until the operator resolves its private storage.
			return fmt.Errorf("save private managed team binding: %w", err)
		}
	}
	g.managedTeamID = requested
	return nil
}

// ManagedTeamID returns the immutable installation binding established at startup.
func (g *Gateway) ManagedTeamID() string { return g.managedTeamID }
