package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

// The management launcher supplies this host-owned directory. It never comes
// from project configuration, model output, or a browser request. Multiple
// account workers share policy without sharing their private queue contents.
type sharedSwarmPolicy struct {
	Version    int                  `json:"version"`
	HostScope  string               `json:"hostScope"`
	Revision   int                  `json:"revision"`
	ObservedAt string               `json:"observedAt"`
	Paused     bool                 `json:"paused"`
	Policy     SwarmSchedulerPolicy `json:"policy"`
}

func (s *Server) initSwarmPolicyDirectory() error {
	dir := s.swarmSchedulerPolicyDir
	if dir == "" {
		return nil
	}
	if !filepath.IsAbs(dir) {
		return errors.New("shared execution policy directory must be absolute")
	}
	ancestor := filepath.Clean(dir)
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return ErrProjectRunInvalid
		}
		ancestor = parent
	}
	if _, err := hostpath.ResolveOutside(s.projectRoot(), ancestor); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	real, err := hostpath.ResolveOutside(s.projectRoot(), dir)
	if err != nil {
		return err
	}
	if err = stateOutsideProjects(s.projectRoot(), real); err != nil {
		return err
	}
	s.swarmSchedulerPolicyDir = real
	_, err = s.readSharedSwarmPolicy()
	return err
}
func (s *Server) readSharedSwarmPolicy() (sharedSwarmPolicy, error) {
	dir := s.swarmSchedulerPolicyDir
	row := sharedSwarmPolicy{Version: 1, HostScope: reviewDigest(dir), Revision: 1, Policy: defaultSwarmSchedulerPolicy()}
	real, err := hostpath.ResolveOutside(s.projectRoot(), dir)
	if err != nil || real != dir {
		return sharedSwarmPolicy{}, ErrProjectRunInvalid
	}
	path, err := guardedProjectPath(dir, "policy.json", false)
	if errors.Is(err, os.ErrNotExist) {
		return row, nil
	}
	if err != nil {
		return sharedSwarmPolicy{}, err
	}
	if hostpath.OneName(path) != nil {
		return sharedSwarmPolicy{}, ErrProjectRunInvalid
	}
	file, err := os.Open(path)
	if err != nil {
		return sharedSwarmPolicy{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16384 {
		return sharedSwarmPolicy{}, ErrProjectRunInvalid
	}
	data, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil {
		return sharedSwarmPolicy{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	// Defaults apply only to a missing file. Missing optional fields in an
	// existing policy must retain their legacy zero meaning, not inherit a new
	// default (which could silently change its reserve or invalidate its floor).
	row = sharedSwarmPolicy{}
	if len(data) > 16384 || decoder.Decode(&row) != nil || decoder.Decode(new(any)) != io.EOF || row.Version != 1 || row.HostScope != reviewDigest(dir) || row.Revision < 1 || !validateSwarmSchedulerPolicy(row.Policy) {
		return sharedSwarmPolicy{}, ErrProjectRunInvalid
	}
	if _, err = time.Parse(time.RFC3339Nano, row.ObservedAt); err != nil {
		return sharedSwarmPolicy{}, ErrProjectRunInvalid
	}
	return row, nil
}
func (s *Server) applySharedSwarmPolicy(row swarmSchedulerLedger) (swarmSchedulerLedger, error) {
	if s.swarmSchedulerPolicyDir == "" {
		return row, nil
	}
	shared, err := s.readSharedSwarmPolicy()
	if err != nil {
		return swarmSchedulerLedger{}, err
	}
	row.Policy = shared.Policy
	row.PolicyRevision = shared.Revision
	row.Paused = shared.Paused
	if shared.ObservedAt > row.ObservedAt {
		row.ObservedAt = shared.ObservedAt
	}
	return row, nil
}
func (s *Server) updateSharedSwarmPolicy(ctx context.Context, expected int, policy SwarmSchedulerPolicy, paused bool) error {
	if !validateSwarmSchedulerPolicy(policy) {
		return ErrProjectRunInvalid
	}
	dir := s.swarmSchedulerPolicyDir
	if _, err := s.readSharedSwarmPolicy(); err != nil {
		return err
	}
	path, err := guardedProjectPath(dir, "policy.lock", true)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(path); err == nil {
		if hostpath.OneName(path) != nil {
			return ErrProjectRunInvalid
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lock, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	info, err := lock.Stat()
	named, statErr := os.Lstat(path)
	if err != nil || statErr != nil || !info.Mode().IsRegular() || !os.SameFile(info, named) || hostpath.OneName(path) != nil {
		return ErrProjectRunInvalid
	}
	release, err := lockSwarmPolicy(ctx, lock)
	if err != nil {
		return err
	}
	defer release()
	row, err := s.readSharedSwarmPolicy()
	if err != nil {
		return err
	}
	if row.Revision != expected {
		return ErrProjectRunConflict
	}
	row.Revision++
	row.Policy = policy
	row.Paused = paused
	row.ObservedAt = s.schedulerNow().Format(time.RFC3339Nano)
	path, err = guardedProjectPath(dir, "policy.json", true)
	if err != nil {
		return err
	}
	if err = durableRunJSON(path, row); err != nil {
		return err
	}
	return syncProjectRunDirectory(dir)
}
