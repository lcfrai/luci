// Package lanes owns the cockpit's worktrees: local clones of a project repository, one per
// worktree (OD-2), and the small index that says which project and product branch each
// belongs to. git is the truth for every git fact; this index holds only what git cannot.
package lanes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/statepath"
)

// Lane is one cockpit worktree. Its ID is phase 2's rule, the hash of its Path, and its Path ends
// in a creation nonce, so removing a lane and creating another with the same Name gives a new
// directory and a new ID (review Minor 7). Without the nonce the new lane would inherit the old
// one's id, and with it every record keyed on that id: the Panacea-Land trailers history names
// (task 10's worktreeName), undo's SetWorktreeReady, registry's WorktreeFlags, the ref
// refs/panacea/lanes/<id> and task 11's refs/panacea/archive/lanes/<id>/*. The ID stays
// WorktreeID(Path), not a hash of path plus a counter, so every id in the product is still
// derived by the one rule phase 2 wrote.
type Lane struct {
	ID        string    `json:"id"`        // project.WorktreeID(Path): "w-" + 8 hex; new per creation
	ProjectID string    `json:"projectId"` // the project it was cloned from
	Branch    string    `json:"branch"`    // the product branch's short ref name, e.g. "evidence-upload"
	Name      string    `json:"name"`      // the worktree's name, unique per project among its lanes
	Path      string    `json:"path"`      // absolute: <CloneRoot>/<ProjectID>/<Name>-<Nonce>
	MachineID string    `json:"machineId"` // gitx.Machine.ID(); "local" in phase 3
	CreatedAt time.Time `json:"createdAt"`
}

// Queue is the human's landing order for one product branch (plan decision 1). Order holds
// worktree ids; an id that is not a ready lane of that branch is ignored when read.
type Queue struct {
	ProjectID string   `json:"projectId"`
	Branch    string   `json:"branch"`
	Order     []string `json:"order"`
}

// Rename is one branch rename the cockpit performed (brief A8). Task 11 writes these.
type Rename struct {
	ProjectID    string    `json:"projectId"`
	FromBranchID string    `json:"fromBranchId"`
	ToBranchID   string    `json:"toBranchId"`
	From         string    `json:"from"`
	To           string    `json:"to"`
	At           time.Time `json:"at"`
}

var (
	ErrExists  = errors.New("lanes: a lane with that id or name already exists")
	ErrUnknown = errors.New("lanes: no lane with that id")
)

// Store is the lane index. Every method is safe for concurrent use.
type Store interface {
	List() []Lane
	ByID(id string) (Lane, bool)
	ByProject(projectID string) []Lane       // CreatedAt order, then Name
	Add(l Lane) error                        // ErrExists on a duplicate ID, or ProjectID+Name
	Remove(id string) error                  // ErrUnknown; also drops id from every Queue
	Queue(projectID, branch string) []string // nil when never set
	SetQueue(projectID, branch string, order []string) error
	Renames(projectID string) []Rename // oldest first
	AddRename(r Rename) error
	// Retarget moves every lane and queue of (projectID, from) to branch to. Task 11's rename
	// refuses while lanes exist (open question Q4), so in v1 it only ever moves a queue.
	Retarget(projectID, from, to string) error
}

// schemaVersion is the version this build writes and the highest it reads.
const schemaVersion = 1

// indexFile is the one document under the store's directory.
const indexFile = "lanes.json"

type document struct {
	SchemaVersion int      `json:"schemaVersion"`
	Lanes         []Lane   `json:"lanes"`
	Queues        []Queue  `json:"queues"`
	Renames       []Rename `json:"renames"`
}

// FileStore keeps the index in one JSON document, <dir>/lanes.json, schemaVersion 1,
// written through statepath.WriteAtomic on every mutation and rolled back in memory when the
// write fails (registry.FileStore's discipline).
type FileStore struct {
	path string
	mu   sync.Mutex
	doc  document
}

var _ Store = (*FileStore)(nil)

// OpenFileStore loads dir/lanes.json, creating dir (0o700) and an empty index when absent.
// An unreadable or unparseable file is an error naming the path — never an empty index,
// which would orphan every clone on disk without a word.
func OpenFileStore(dir string) (*FileStore, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("lanes: index directory %s: %w", dir, err)
	}
	s := &FileStore{path: filepath.Join(dir, indexFile)}
	raw, err := os.ReadFile(s.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		s.doc = document{SchemaVersion: schemaVersion}
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		return s, nil
	case err != nil:
		return nil, fmt.Errorf("lanes: read %s: %w", s.path, err)
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("lanes: parse %s: %w", s.path, err)
	}
	if doc.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("lanes: %s is schemaVersion %d, this build reads %d", s.path, doc.SchemaVersion, schemaVersion)
	}
	s.doc = doc
	return s, nil
}

func (s *FileStore) saveLocked() error {
	doc := s.doc
	doc.SchemaVersion = schemaVersion
	if doc.Lanes == nil {
		doc.Lanes = []Lane{}
	}
	if doc.Queues == nil {
		doc.Queues = []Queue{}
	}
	if doc.Renames == nil {
		doc.Renames = []Rename{}
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("lanes: encode %s: %w", s.path, err)
	}
	if err := statepath.WriteAtomic(s.path, append(raw, '\n')); err != nil {
		return fmt.Errorf("lanes: write %s: %w", s.path, err)
	}
	return nil
}

// mutate runs f on a copy of the document and keeps the copy only when it reached the disk.
func (s *FileStore) mutate(f func(d *document) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	before := s.doc
	next := cloneDoc(s.doc)
	if err := f(&next); err != nil {
		return err
	}
	s.doc = next
	if err := s.saveLocked(); err != nil {
		s.doc = before
		return err
	}
	return nil
}

func cloneDoc(d document) document {
	out := document{SchemaVersion: d.SchemaVersion,
		Lanes: append([]Lane(nil), d.Lanes...), Renames: append([]Rename(nil), d.Renames...)}
	for _, q := range d.Queues {
		q.Order = append([]string(nil), q.Order...)
		out.Queues = append(out.Queues, q)
	}
	return out
}

func sortLanes(ls []Lane) {
	sort.SliceStable(ls, func(i, j int) bool {
		a, b := ls[i], ls[j]
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
}

func (s *FileStore) List() []Lane {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Lane{}, s.doc.Lanes...)
	sortLanes(out)
	return out
}

func (s *FileStore) ByID(id string) (Lane, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.doc.Lanes {
		if l.ID == id {
			return l, true
		}
	}
	return Lane{}, false
}

func (s *FileStore) ByProject(projectID string) []Lane {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Lane
	for _, l := range s.doc.Lanes {
		if l.ProjectID == projectID {
			out = append(out, l)
		}
	}
	sortLanes(out)
	return out
}

func (s *FileStore) Add(l Lane) error {
	return s.mutate(func(d *document) error {
		for _, have := range d.Lanes {
			if have.ID == l.ID || (have.ProjectID == l.ProjectID && have.Name == l.Name) {
				return fmt.Errorf("%w: %s (%s in %s)", ErrExists, l.ID, l.Name, l.ProjectID)
			}
		}
		d.Lanes = append(d.Lanes, l)
		return nil
	})
}

func (s *FileStore) Remove(id string) error {
	return s.mutate(func(d *document) error {
		kept := d.Lanes[:0:0]
		for _, l := range d.Lanes {
			if l.ID != id {
				kept = append(kept, l)
			}
		}
		if len(kept) == len(d.Lanes) {
			return fmt.Errorf("%w: %s", ErrUnknown, id)
		}
		d.Lanes = kept
		for i := range d.Queues {
			d.Queues[i].Order = without(d.Queues[i].Order, id)
		}
		return nil
	})
}

func without(order []string, id string) []string {
	out := order[:0:0]
	for _, o := range order {
		if o != id {
			out = append(out, o)
		}
	}
	return out
}

func (s *FileStore) Queue(projectID, branch string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range s.doc.Queues {
		if q.ProjectID == projectID && q.Branch == branch {
			return append([]string(nil), q.Order...)
		}
	}
	return nil
}

func (s *FileStore) SetQueue(projectID, branch string, order []string) error {
	return s.mutate(func(d *document) error {
		for i, q := range d.Queues {
			if q.ProjectID == projectID && q.Branch == branch {
				d.Queues[i].Order = append([]string(nil), order...)
				return nil
			}
		}
		d.Queues = append(d.Queues, Queue{ProjectID: projectID, Branch: branch, Order: append([]string(nil), order...)})
		return nil
	})
}

func (s *FileStore) Renames(projectID string) []Rename {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Rename
	for _, r := range s.doc.Renames {
		if r.ProjectID == projectID {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

func (s *FileStore) AddRename(r Rename) error {
	return s.mutate(func(d *document) error {
		d.Renames = append(d.Renames, r)
		return nil
	})
}

func (s *FileStore) Retarget(projectID, from, to string) error {
	return s.mutate(func(d *document) error {
		for i, l := range d.Lanes {
			if l.ProjectID == projectID && l.Branch == from {
				d.Lanes[i].Branch = to
			}
		}
		src, dst := -1, -1
		for i, q := range d.Queues {
			switch {
			case q.ProjectID == projectID && q.Branch == from:
				src = i
			case q.ProjectID == projectID && q.Branch == to:
				dst = i
			}
		}
		if src < 0 {
			return nil
		}
		if dst < 0 {
			d.Queues[src].Branch = to
			return nil
		}
		for _, id := range d.Queues[src].Order { // to's order first, then from's ids it lacks
			if !contains(d.Queues[dst].Order, id) {
				d.Queues[dst].Order = append(d.Queues[dst].Order, id)
			}
		}
		d.Queues = append(d.Queues[:src], d.Queues[src+1:]...)
		return nil
	})
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
