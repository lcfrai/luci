package gitx

import (
	"context"
	"path/filepath"
)

// Machine is where git runs for one machine id (brief A6). Phase 3 has one, Local. Phase 8
// adds a Kata machine behind the same methods: phase 8's machine serves the runner
// (P8-D7), and lane work crosses to the control plane as a bundle, which is data (ruling
// R4.4), the same model phase 3 builds with task 24's runner.
type Machine interface {
	// ID is the machine id worktrees and sessions carry: api.LocalMachineID ("local") here.
	ID() string
	// Git runs git on this machine, in host-private repositories only (ruling R4.2).
	Git() Runner
	// CloneRoot is the absolute directory this machine's lane clones are created under.
	// Task 04 copies it into Hardening.LaneRoot, so Git() refuses every call in it.
	CloneRoot() string
	// Join joins path elements in this machine's own path syntax (P3A-1, env seams review 1 M4).
	// Every path phase 3 derives from CloneRoot() is built through it, so a Windows control
	// plane never hands a cloud machine `\machine\lanes\…`, which its MachineFS refuses.
	// Local.Join is filepath.Join, so nothing local changes; phase 8's machines.Remote is path.Join.
	Join(elem ...string) string
	// FS is this machine's disk for lane create and remove (seam S8-P3.6, env R4 ruling 4).
	// nil means this process's own disk, lanes.LocalFS (task 03), and is admitted only when
	// ID() == "local": lanes answers ErrNoMachineFS for a nil FS on any other machine, so a
	// cloud machine never falls back to the laptop's disk.
	FS() MachineFS
}

// Local is this machine.
type Local struct {
	Runner Runner
	Root   string    // absolute; cmd/api validates it (task 04)
	Files  MachineFS // nil: lanes.LocalFS, the laptop's disk. Only tests set it (task 03's Seal order)
}

func (l Local) ID() string                 { return "local" }
func (l Local) Git() Runner                { return l.Runner }
func (l Local) CloneRoot() string          { return l.Root }
func (l Local) Join(elem ...string) string { return filepath.Join(elem...) }
func (l Local) FS() MachineFS              { return l.Files }

var _ Machine = Local{}

// MachineFS is the filesystem half of lane create and remove (phase-3
// task-03.md:153-154, :189-190, :227-268). Paths are absolute on that machine.
// The local implementation is os, on the laptop's disk.
type MachineFS interface {
	// Exists is os.Lstat at path: exists is false only for not-exist; dir is true only for a
	// real directory, never a symlink. Step 3 reads exists; Catalog.Path's worktree.Verify reads dir.
	Exists(ctx context.Context, path string) (exists, dir bool, err error)
	// RemoveTree is task 03's RemoveTree(root, path): it refuses a path not under root.
	RemoveTree(ctx context.Context, root, path string) error
	// CopyGeneratedHooks is the copy half of task 03's CopyGeneratedHooks: its Lstat
	// guards, its SameFile check and its 64-file / 1 MiB budget. Its ls-files check and
	// the take-back stay in lanes.CopyGeneratedHooks, over m.Git() and RemoveTree.
	CopyGeneratedHooks(ctx context.Context, projectTop, cloneTop, rel string) (int, error)
	// Seal ends R4.5's window for path: Create calls it after step 6, before Store.Add.
	// Locally it does nothing; on a cloud machine it closes machined's creating-set entry.
	Seal(ctx context.Context, path string) error
}
