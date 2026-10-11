package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// These are storage bounds, not a claim that history may be discarded. Reaching
// either bound refuses a new write; existing exact receipts remain readable.
const coordinationArchivePageLimit = 64
const coordinationArchiveBytes = 256 << 20
const coordinationArchiveFileLimit = coordinationFileLimit + orchestratorFileLimit + (1 << 20)
const coordinationRetainThreshold = 48
const coordinationArchiveReadBytes = 32 << 20

type CoordinationEventReference struct {
	ID             string `json:"id"`
	Sequence       int    `json:"sequence"`
	OriginIntentID string `json:"originIntentId"`
}

type CoordinationArchiveReference struct {
	Digest string                       `json:"digest"`
	Bytes  int                          `json:"bytes"`
	Events []CoordinationEventReference `json:"events"`
}

type coordinationArchivedIdentity struct {
	IntentID    string `json:"intentId"`
	TurnID      string `json:"turnId"`
	ActorID     string `json:"actorId"`
	Key         string `json:"keyDigest"`
	Request     string `json:"requestDigest"`
	TurnKey     string `json:"turnKeyDigest"`
	TurnRequest string `json:"turnRequestDigest"`
}

type coordinationArchiveIndex struct {
	CoordinationArchiveReference
	Retired []coordinationArchivedIdentity `json:"retired"`
}

type coordinationArchive struct {
	Version int                `json:"version"`
	Ledger  coordinationLedger `json:"ledger"`
	Origins orchestratorLedger `json:"origins"`
	Retired []string           `json:"retired"`
}

// A receipt identifies an observation, not permission to act on it. An empty
// archive digest identifies an event that was active when the receipt was made;
// exact lookup follows the bounded retirement index if it has since moved.
type CoordinationEventReceipt struct {
	EventID        string `json:"eventId"`
	Sequence       int    `json:"sequence"`
	OriginIntentID string `json:"originIntentId"`
	ArchiveDigest  string `json:"archiveDigest,omitempty"`
}

type CoordinationEventObservation struct {
	Receipt CoordinationEventReceipt `json:"receipt"`
	Event   CoordinationEvent        `json:"event"`
	Intent  CoordinationIntent       `json:"intent"`
	Origin  OrchestratorTurn         `json:"origin"`
}

func (s *Server) coordinationArchiveDir(id string, create bool) (string, error) {
	if !prototypeReleaseID.MatchString(id) {
		return "", ErrOrchestratorInvalid
	}
	for _, relative := range []string{"coordination-archives", "coordination-archives/" + id} {
		path, err := guardedProjectPath(s.stateDir, relative, create)
		if err != nil {
			return "", err
		}
		if create {
			if err = os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return "", err
			}
			if runtime.GOOS == "windows" {
				err = restrictPrivateFile(path)
			} else {
				err = os.Chmod(path, 0700)
			}
			if err != nil {
				return "", err
			}
		}
	}
	return guardedProjectPath(s.stateDir, "coordination-archives/"+id, false)
}

func coordinationArchiveDescriptor(page coordinationArchive) (coordinationArchiveIndex, error) {
	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil || len(data)+1 > coordinationArchiveFileLimit {
		return coordinationArchiveIndex{}, ErrOrchestratorCapacity
	}
	out := coordinationArchiveIndex{CoordinationArchiveReference: CoordinationArchiveReference{Digest: reviewDigest(page), Bytes: len(data) + 1, Events: []CoordinationEventReference{}}, Retired: []coordinationArchivedIdentity{}}
	wanted := map[string]bool{}
	for _, id := range page.Retired {
		if wanted[id] {
			return out, ErrOrchestratorInvalid
		}
		wanted[id] = true
	}
	for _, row := range page.Ledger.Intents {
		if !wanted[row.ID] {
			continue
		}
		turn := coordinationTurn(page.Origins, row.TurnID)
		if row.Status == "pending" || turn == nil || turn.ApplyIntent != "" {
			return out, ErrOrchestratorInvalid
		}
		out.Retired = append(out.Retired, coordinationArchivedIdentity{row.ID, row.TurnID, row.Actor.ID, row.Key, row.Request, turn.KeyDigest, turn.RequestDigest})
	}
	if len(out.Retired) == 0 || len(out.Retired) != len(wanted) {
		return out, ErrOrchestratorInvalid
	}
	for _, event := range page.Ledger.Events {
		if wanted[event.IntentID] {
			out.Events = append(out.Events, CoordinationEventReference{event.ID, event.Sequence, event.IntentID})
		}
	}
	return out, nil
}

// Validate the complete sequence using compact references; no archive contents
// are interpreted as dependency satisfaction or current authority here.
func coordinationEventIndex(saved coordinationLedger) (map[int]string, error) {
	if len(saved.Archives) > coordinationArchivePageLimit {
		return nil, ErrOrchestratorCapacity
	}
	bySequence, ids, intents, turns, keys := map[int]string{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	turnKeys := map[string]bool{}
	digests, totalBytes := map[string]bool{}, 0
	add := func(id string, seq int) bool {
		if !strings.HasPrefix(id, "coord-event-") || !prototypeReleaseID.MatchString(id) || seq < 1 || seq > (coordinationArchivePageLimit+1)*coordinationIntentLimit*3 || ids[id] || bySequence[seq] != "" {
			return false
		}
		ids[id], bySequence[seq] = true, id
		return true
	}
	for _, archive := range saved.Archives {
		if !hexDigest(archive.Digest) || digests[archive.Digest] || archive.Bytes < 1 || archive.Bytes > coordinationArchiveFileLimit || len(archive.Retired) == 0 || len(archive.Retired) > coordinationIntentLimit || len(archive.Events) == 0 || len(archive.Events) > coordinationIntentLimit*3 {
			return nil, ErrOrchestratorInvalid
		}
		digests[archive.Digest] = true
		totalBytes += archive.Bytes
		owned := map[string]bool{}
		for _, row := range archive.Retired {
			if !strings.HasPrefix(row.IntentID, "intent-") || !prototypeReleaseID.MatchString(row.IntentID) || !strings.HasPrefix(row.TurnID, "turn-") || !prototypeReleaseID.MatchString(row.TurnID) || !validConnectionToken(row.ActorID, 128, false) || intents[row.IntentID] || turns[row.TurnID] || keys[row.Key] || turnKeys[row.TurnKey] || !hexDigest(row.Key) || !hexDigest(row.Request) || !hexDigest(row.TurnKey) || !hexDigest(row.TurnRequest) {
				return nil, ErrOrchestratorInvalid
			}
			intents[row.IntentID], turns[row.TurnID], keys[row.Key] = true, true, true
			owned[row.IntentID], turnKeys[row.TurnKey] = true, true
		}
		for _, ref := range archive.Events {
			if !owned[ref.OriginIntentID] || !add(ref.ID, ref.Sequence) {
				return nil, ErrOrchestratorInvalid
			}
		}
	}
	if totalBytes > coordinationArchiveBytes {
		return nil, ErrOrchestratorCapacity
	}
	for _, row := range saved.Intents {
		if intents[row.ID] || turns[row.TurnID] || keys[row.Key] {
			return nil, ErrOrchestratorInvalid
		}
	}
	for _, event := range saved.Events {
		if !add(event.ID, event.Sequence) {
			return nil, ErrOrchestratorInvalid
		}
	}
	for i := 1; i <= len(bySequence); i++ {
		if bySequence[i] == "" {
			return nil, ErrOrchestratorInvalid
		}
	}
	if saved.EventSequence != 0 || saved.EventHead != "" || len(saved.Archives) > 0 {
		if saved.EventSequence != len(bySequence) || saved.EventHead != bySequence[len(bySequence)] {
			return nil, ErrOrchestratorInvalid
		}
	}
	return bySequence, nil
}

type coordinationArchivedWitness struct {
	Intent          CoordinationIntent
	AppliedSequence int
}
type coordinationArchiveReader struct {
	server  *Server
	project project.Project
	pages   map[string]coordinationArchive
	loading map[string]bool
	bytes   int
}

func newCoordinationArchiveReader(s *Server, p project.Project) *coordinationArchiveReader {
	return &coordinationArchiveReader{server: s, project: p, pages: map[string]coordinationArchive{}, loading: map[string]bool{}}
}
func (s *Server) readCoordinationArchive(p project.Project, saved coordinationLedger, ref coordinationArchiveIndex) (coordinationArchive, error) {
	return newCoordinationArchiveReader(s, p).read(saved, ref)
}
func (r *coordinationArchiveReader) hydrate(saved *coordinationLedger) error {
	saved.witnesses = map[string]coordinationArchivedWitness{}
	active := map[string]bool{}
	for _, row := range saved.Intents {
		active[row.ID] = true
	}
	for _, row := range saved.Intents {
		for _, dep := range row.DependsOn {
			if active[dep] {
				continue
			}
			if _, ok := saved.witnesses[dep]; ok {
				continue
			}
			found := false
			for _, ref := range saved.Archives {
				for _, id := range ref.Retired {
					if id.IntentID != dep {
						continue
					}
					page, err := r.read(*saved, ref)
					if err != nil {
						return err
					}
					intent := coordinationIntent(&page.Ledger, id.TurnID)
					if intent == nil || intent.ID != dep || intent.Status != "applied" {
						return ErrOrchestratorInvalid
					}
					for _, event := range page.Ledger.Events {
						if event.IntentID == dep && event.Kind == "applied" {
							saved.witnesses[dep] = coordinationArchivedWitness{intent.CoordinationIntent, event.Sequence}
							found = true
						}
					}
				}
			}
			if !found {
				return ErrOrchestratorInvalid
			}
		}
	}
	return nil
}

func (r *coordinationArchiveReader) read(saved coordinationLedger, ref coordinationArchiveIndex) (coordinationArchive, error) {
	s, p := r.server, r.project
	if page, ok := r.pages[ref.Digest]; ok {
		actual, e := coordinationArchiveDescriptor(page)
		if e != nil || !reflect.DeepEqual(actual, ref) {
			return coordinationArchive{}, ErrOrchestratorInvalid
		}
		return page, nil
	}
	if r.loading[ref.Digest] || ref.Bytes < 1 || r.bytes+ref.Bytes > coordinationArchiveReadBytes {
		return coordinationArchive{}, ErrOrchestratorCapacity
	}
	r.loading[ref.Digest] = true
	defer delete(r.loading, ref.Digest)
	r.bytes += ref.Bytes
	var page coordinationArchive
	dir, err := s.coordinationArchiveDir(p.ID, false)
	if err != nil {
		return page, ErrOrchestratorInvalid
	}
	path, err := guardedProjectPath(dir, ref.Digest+".json", false)
	if err != nil || externalReadJSON(path, coordinationArchiveFileLimit, &page) != nil {
		return page, ErrOrchestratorInvalid
	}
	if page.Version != 1 || reviewDigest(page) != ref.Digest || page.Ledger.ProjectID != saved.ProjectID || page.Ledger.AccountScope != saved.AccountScope || page.Ledger.ProjectScope != saved.ProjectScope {
		return page, ErrOrchestratorInvalid
	}
	actual, err := coordinationArchiveDescriptor(page)
	if err != nil || !reflect.DeepEqual(actual, ref) {
		return page, ErrOrchestratorInvalid
	}
	if err = validateCoordinationShape(page.Ledger, saved); err != nil {
		return page, err
	}
	raw, err := json.Marshal(page.Origins)
	if err != nil {
		return page, ErrOrchestratorInvalid
	}
	if _, err = decodeOrchestratorLedger(raw, orchestratorLedger{ProjectID: saved.ProjectID, AccountScope: saved.AccountScope, ProjectScope: saved.ProjectScope}); err != nil {
		return page, err
	}
	if err = r.hydrate(&page.Ledger); err != nil {
		return page, err
	}
	if err = validateCoordinationBindingsWithTurns(page.Ledger, page.Origins); err != nil {
		return page, err
	}
	r.pages[ref.Digest] = page
	return page, nil
}

// The physical quota includes unreferenced pages left by a crash before pointer
// publication. They may be reused by exact digest, never silently pruned.
func (s *Server) writeCoordinationArchive(p project.Project, page coordinationArchive, ref coordinationArchiveIndex) error {
	dir, err := s.coordinationArchiveDir(p.ID, true)
	if err != nil {
		return ErrOrchestratorInvalid
	}
	f, err := os.Open(dir)
	if err != nil {
		return ErrOrchestratorInvalid
	}
	entries, readErr := f.ReadDir(coordinationArchivePageLimit + 1)
	f.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return ErrOrchestratorInvalid
	}
	if len(entries) > coordinationArchivePageLimit {
		return ErrOrchestratorCapacity
	}
	bytes, exists := 0, false
	for _, entry := range entries {
		digest := strings.TrimSuffix(entry.Name(), ".json")
		staging := orchestratorStagingName.MatchString(entry.Name())
		if (!staging && (!hexDigest(digest) || entry.Name() != digest+".json")) || !entry.Type().IsRegular() {
			return ErrOrchestratorInvalid
		}
		path, e := guardedProjectPath(dir, entry.Name(), false)
		if e != nil || hostpath.OneName(path) != nil {
			return ErrOrchestratorInvalid
		}
		info, e := os.Stat(path)
		if e != nil || info.Size() > coordinationArchiveFileLimit {
			return ErrOrchestratorInvalid
		}
		bytes += int(info.Size())
		if !staging && digest == ref.Digest {
			exists = true
		}
	}
	if bytes > coordinationArchiveBytes {
		return ErrOrchestratorCapacity
	}
	if exists {
		_, err = s.readCoordinationArchive(p, page.Ledger, ref)
		return err
	}
	if len(entries) >= coordinationArchivePageLimit || bytes+ref.Bytes > coordinationArchiveBytes {
		return ErrOrchestratorCapacity
	}
	path, err := guardedProjectPath(dir, ref.Digest+".json", true)
	if err != nil || durableRunJSON(path, page) != nil {
		return ErrOrchestratorInvalid
	}
	return nil
}

// Caller owns the existing mutation lock. Publication is archive-first and
// active-pointer-second. Failure leaves the old authoritative rows untouched.
func (s *Server) compactCoordinationLocked(p project.Project, saved *coordinationLedger, incoming int, protected ...string) error {
	if len(saved.Intents)+incoming <= coordinationRetainThreshold {
		return nil
	}
	turns, err := s.readOrchestrator(p)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, id := range protected {
		keep[id] = true
	}
	for _, row := range saved.Intents {
		turn := coordinationTurn(turns, row.TurnID)
		if row.Status == "pending" || turn == nil || turn.ApplyIntent != "" || (turn.Answer != nil && turn.Answer.Blocking && turn.QuestionResolvedBy == "") {
			keep[row.ID] = true
		}
	}
	for range saved.Intents {
		for _, row := range saved.Intents {
			if !keep[row.ID] {
				continue
			}
			for _, dep := range row.DependsOn {
				keep[dep] = true
			}
			if row.SupersededBy != "" {
				keep[row.SupersededBy] = true
			}
		}
	}
	retired := []string{}
	for _, row := range saved.Intents {
		if !keep[row.ID] {
			retired = append(retired, row.ID)
		}
	}
	if len(retired) == 0 {
		return nil
	}
	origins := turns
	origins.Turns = []storedOrchestratorTurn{}
	for _, row := range saved.Intents {
		turn := coordinationTurn(turns, row.TurnID)
		if turn == nil {
			return ErrOrchestratorInvalid
		}
		origins.Turns = append(origins.Turns, *turn)
	}
	sort.Slice(origins.Turns, func(i, j int) bool { return origins.Turns[i].Sequence < origins.Turns[j].Sequence })
	page := coordinationArchive{Version: 1, Ledger: *saved, Origins: origins, Retired: retired}
	ref, err := coordinationArchiveDescriptor(page)
	if err != nil {
		return err
	}
	if len(saved.Archives) >= coordinationArchivePageLimit {
		return ErrOrchestratorCapacity
	}
	if err = s.writeCoordinationArchive(p, page, ref); err != nil {
		return err
	}
	// Allocate new slices: the immutable page's source slices must not change.
	active := []storedCoordinationIntent{}
	for _, row := range saved.Intents {
		if keep[row.ID] {
			active = append(active, row)
		}
	}
	events := []CoordinationEvent{}
	for _, event := range saved.Events {
		if keep[event.IntentID] {
			events = append(events, event)
		}
	}
	sequence, err := coordinationEventIndex(*saved)
	if err != nil {
		return err
	}
	saved.EventSequence = len(sequence)
	saved.EventHead = sequence[len(sequence)]
	saved.Intents, saved.Events = active, events
	saved.Archives = append(append([]coordinationArchiveIndex{}, saved.Archives...), ref)
	saved.Revision++
	return nil
}

func (s *Server) archivedCoordinationIntent(p project.Project, saved coordinationLedger, match func(coordinationArchivedIdentity) bool) (*storedCoordinationIntent, *storedOrchestratorTurn, error) {
	for _, ref := range saved.Archives {
		for _, id := range ref.Retired {
			if match(id) {
				page, err := s.readCoordinationArchive(p, saved, ref)
				if err != nil {
					return nil, nil, err
				}
				intent := coordinationIntent(&page.Ledger, id.TurnID)
				turn := coordinationTurn(page.Origins, id.TurnID)
				if intent == nil || turn == nil || intent.ID != id.IntentID {
					return nil, nil, ErrOrchestratorInvalid
				}
				return intent, turn, nil
			}
		}
	}
	return nil, nil, nil
}

// This also protects the upstream paid-request namespace after normal history
// rotation. An archived same key with changed request remains a conflict.
func (s *Server) archivedCoordinationRequestLocked(p project.Project, key, request string) (*storedOrchestratorTurn, error) {
	saved, err := s.readCoordination(p)
	if err != nil {
		return nil, err
	}
	_, turn, err := s.archivedCoordinationIntent(p, saved, func(id coordinationArchivedIdentity) bool { return id.TurnKey == key })
	if err != nil {
		return nil, err
	}
	if turn != nil && turn.RequestDigest != request {
		return nil, ErrOrchestratorConflict
	}
	return turn, nil
}

// Caller holds s.mu. This is exact original readback, including settled apply
// receipts; it never promotes an archived proposal back into active work.
func (s *Server) archivedCoordinationTurnLocked(p project.Project, turnID string) (*storedOrchestratorTurn, error) {
	saved, err := s.readCoordination(p)
	if err != nil {
		return nil, err
	}
	_, turn, err := s.archivedCoordinationIntent(p, saved, func(id coordinationArchivedIdentity) bool { return id.TurnID == turnID })
	return turn, err
}

func (s *Server) archivedCoordinationForeignRequestLocked(p project.Project, actor, key string) (bool, error) {
	saved, err := s.readCoordination(p)
	if err != nil {
		return false, err
	}
	for _, page := range saved.Archives {
		for _, id := range page.Retired {
			if id.ActorID != actor && id.TurnKey == reviewDigest(id.ActorID+":"+key) {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *Server) getCoordinationEvent(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	for key, values := range q {
		if len(values) != 1 || (key != "sequence" && key != "originIntentId" && key != "archiveDigest") {
			orchestratorProblem(w, ErrOrchestratorInvalid)
			return
		}
	}
	sequence, err := strconv.Atoi(q.Get("sequence"))
	if err != nil {
		orchestratorProblem(w, ErrOrchestratorInvalid)
		return
	}
	value, err := s.CoordinationEventByIdentity(r.Context(), r.PathValue("id"), CoordinationEventReceipt{r.PathValue("eventId"), sequence, q.Get("originIntentId"), q.Get("archiveDigest")})
	if err != nil {
		coordinatorPolicyProblem(w, err)
		return
	}
	journeyJSON(w, 200, value)
}

func (s *Server) getCoordinationIntent(w http.ResponseWriter, r *http.Request) {
	if _, ok := authenticatedContextActor(r.Context()); !ok {
		coordinatorPolicyProblem(w, errCoordinatorAuthority)
		return
	}
	id := r.PathValue("intentId")
	if !strings.HasPrefix(id, "intent-") || !prototypeReleaseID.MatchString(id) {
		orchestratorProblem(w, ErrOrchestratorInvalid)
		return
	}
	release, err := s.AcquireGitSessionAdmission(r.Context(), r.PathValue("id"))
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	defer release()
	p, err := s.resolveProject(r.PathValue("id"))
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	saved, err := s.readCoordination(p)
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	for _, intent := range saved.Intents {
		if intent.ID == id {
			journeyJSON(w, 200, intent.CoordinationIntent)
			return
		}
	}
	intent, _, err := s.archivedCoordinationIntent(p, saved, func(row coordinationArchivedIdentity) bool { return row.IntentID == id })
	if err != nil {
		orchestratorProblem(w, err)
		return
	}
	if intent == nil {
		orchestratorProblem(w, ErrOrchestratorConflict)
		return
	}
	journeyJSON(w, 200, intent.CoordinationIntent)
}

func (s *Server) CoordinationEventByIdentity(ctx context.Context, id string, receipt CoordinationEventReceipt) (CoordinationEventObservation, error) {
	if _, ok := authenticatedContextActor(ctx); !ok {
		return CoordinationEventObservation{}, errCoordinatorAuthority
	}
	if receipt.Sequence < 1 || !strings.HasPrefix(receipt.EventID, "coord-event-") || !prototypeReleaseID.MatchString(receipt.EventID) || !strings.HasPrefix(receipt.OriginIntentID, "intent-") || !prototypeReleaseID.MatchString(receipt.OriginIntentID) || receipt.ArchiveDigest != "" && !hexDigest(receipt.ArchiveDigest) {
		return CoordinationEventObservation{}, ErrOrchestratorInvalid
	}
	release, err := s.AcquireGitSessionAdmission(ctx, id)
	if err != nil {
		return CoordinationEventObservation{}, err
	}
	defer release()
	p, err := s.resolveProject(id)
	if err != nil {
		return CoordinationEventObservation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	saved, err := s.readCoordination(p)
	if err != nil {
		return CoordinationEventObservation{}, err
	}
	selected, origins := saved, orchestratorLedger{}
	actualDigest := ""
	for _, ref := range saved.Archives {
		for _, event := range ref.Events {
			if event.ID == receipt.EventID {
				if receipt.ArchiveDigest != "" && receipt.ArchiveDigest != ref.Digest {
					return CoordinationEventObservation{}, ErrOrchestratorConflict
				}
				page, e := s.readCoordinationArchive(p, saved, ref)
				if e != nil {
					return CoordinationEventObservation{}, e
				}
				selected, origins, actualDigest = page.Ledger, page.Origins, ref.Digest
			}
		}
	}
	if actualDigest == "" {
		if receipt.ArchiveDigest != "" {
			return CoordinationEventObservation{}, ErrOrchestratorConflict
		}
		origins, err = s.readOrchestrator(p)
		if err != nil {
			return CoordinationEventObservation{}, err
		}
	}
	for _, event := range selected.Events {
		if event.ID == receipt.EventID && event.Sequence == receipt.Sequence && event.IntentID == receipt.OriginIntentID {
			for _, intent := range selected.Intents {
				if intent.ID == event.IntentID {
					turn := coordinationTurn(origins, intent.TurnID)
					if turn == nil {
						return CoordinationEventObservation{}, ErrOrchestratorInvalid
					}
					receipt.ArchiveDigest = actualDigest
					return CoordinationEventObservation{receipt, event, intent.CoordinationIntent, turn.OrchestratorTurn}, nil
				}
			}
		}
	}
	return CoordinationEventObservation{}, ErrOrchestratorConflict
}
