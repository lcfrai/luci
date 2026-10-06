package product

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Recent snapshots stay in the small canonical file. Older snapshots are
// preserved verbatim in immutable, content-addressed public context chunks.
// These caps bound the entire reachable chain, not just each individual file.
const journeyRecentRevisions = 8
const journeyActiveTarget = 4 << 20
const journeyStateLimit = 8 << 20
const journeyArchiveChunkLimit = 16 << 20
const journeyArchiveBytesLimit = 256 << 20
const journeyArchiveChunksLimit = 256

var errJourneyArchiveCapacity = errors.New("project context archive is full; export a project backup, then explicitly remove archived revisions in Context storage before saving again; current and recent revisions are retained")

type journeyArchiveChunk struct {
	Version                int                    `json:"version"`
	ContextID              string                 `json:"contextId"`
	ProjectID              string                 `json:"projectId"`
	Previous               string                 `json:"previous,omitempty"`
	BriefRevisions         []Brief                `json:"briefRevisions,omitempty"`
	DesignRevisions        []Design               `json:"designRevisions,omitempty"`
	SpecificationRevisions []ProjectSpecification `json:"specificationRevisions,omitempty"`
	DesignPlanRevisions    []ProjectDesignPlan    `json:"designPlanRevisions,omitempty"`
}
type journeyArchiveRecord struct {
	Digest string
	Data   []byte
	Chunk  journeyArchiveChunk
}

func journeyArchiveFile(digest string) string { return ".luci/history/" + digest + ".json" }
func journeyArchivePath(path string) bool {
	return strings.HasPrefix(path, ".luci/history/") && strings.HasSuffix(path, ".json") && hexDigest(strings.TrimSuffix(strings.TrimPrefix(path, ".luci/history/"), ".json"))
}
func journeyArchiveDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Reading an archive never creates a directory, repairs files, or adopts review
// authority. The caller decides whether to project its old project IDs locally.
func readJourneyArchive(dir, contextID, head string) ([]journeyArchiveRecord, error) {
	return readJourneyArchiveAt(contextID, head, func(digest string) ([]byte, error) { return readJourneyArchiveData(dir, digest) })
}
func readJourneyArchiveData(dir, digest string) ([]byte, error) {
	path, err := guardedProjectPath(dir, journeyArchiveFile(digest), false)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > journeyArchiveChunkLimit {
		return nil, errors.New("context archive must contain bounded ordinary files")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !os.SameFile(info, after) {
		return nil, errors.New("context archive changed while opening")
	}
	return io.ReadAll(io.LimitReader(f, journeyArchiveChunkLimit+1))
}

func readJourneyArchiveAt(contextID, head string, read func(string) ([]byte, error)) ([]journeyArchiveRecord, error) {
	records := []journeyArchiveRecord{}
	err := walkJourneyArchiveAt(contextID, head, read, func(record journeyArchiveRecord) { records = append(records, record) })
	if err != nil {
		return nil, err
	}
	return records, nil
}
func walkJourneyArchive(dir, contextID, head string, visit func(journeyArchiveRecord)) error {
	return walkJourneyArchiveAt(contextID, head, func(digest string) ([]byte, error) { return readJourneyArchiveData(dir, digest) }, visit)
}

// Normal reads and saves verify one bounded chunk at a time; they do not retain
// the entire 256 MiB archive in memory just to report counts or show one page.
func walkJourneyArchiveAt(contextID, head string, read func(string) ([]byte, error), visit func(journeyArchiveRecord)) error {
	seen := map[string]bool{}
	total := 0
	for head != "" {
		if !hexDigest(head) || seen[head] {
			return errors.New("context archive has an invalid or cyclic reference")
		}
		if len(seen) >= journeyArchiveChunksLimit {
			return errJourneyArchiveCapacity
		}
		seen[head] = true
		data, err := read(head)
		if err != nil {
			return errors.New("context archive is missing or unreadable; restore its verified files before editing")
		}
		total += len(data)
		if len(data) > journeyArchiveChunkLimit || total > journeyArchiveBytesLimit {
			return errJourneyArchiveCapacity
		}
		chunk, err := validateJourneyArchiveChunk(data, head, contextID)
		if err != nil {
			return err
		}
		visit(journeyArchiveRecord{Digest: head, Data: data, Chunk: chunk})
		head = chunk.Previous
	}
	return nil
}

func validateJourneyArchiveChunk(data []byte, digest, contextID string) (journeyArchiveChunk, error) {
	var chunk journeyArchiveChunk
	if len(data) > journeyArchiveChunkLimit || journeyArchiveDigest(data) != digest {
		return chunk, errors.New("context archive content does not match its digest")
	}
	if json.Unmarshal(data, &chunk) != nil || chunk.Version != 1 || chunk.ContextID != contextID {
		return chunk, errors.New("context archive does not belong to this context")
	}
	probe := journeyState{Version: 1, ContextID: chunk.ContextID, HistoryHead: chunk.Previous, Brief: Brief{ProjectID: chunk.ProjectID}, Design: Design{ProjectID: chunk.ProjectID}, BriefRevisions: chunk.BriefRevisions, DesignRevisions: chunk.DesignRevisions, SpecificationRevisions: chunk.SpecificationRevisions, DesignPlanRevisions: chunk.DesignPlanRevisions}
	if err := normalizeJourneyIdentity(&probe, chunk.ProjectID); err != nil {
		return chunk, err
	}
	return chunk, nil
}

// Include unreferenced chunks in the physical storage cap: an interrupted
// canonical write must not permit unlimited orphan accumulation on retries.
func journeyArchiveInventory(dir string) (map[string]int64, int64, error) {
	files := map[string]int64{}
	path, err := guardedProjectPath(dir, ".luci/history", false)
	if errors.Is(err, os.ErrNotExist) {
		return files, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	folder, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer folder.Close()
	entries, err := folder.ReadDir(journeyArchiveChunksLimit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, 0, err
	}
	if len(entries) > journeyArchiveChunksLimit {
		return nil, 0, errJourneyArchiveCapacity
	}
	var total int64
	for _, entry := range entries {
		if !journeyArchivePath(".luci/history/" + entry.Name()) {
			return nil, 0, errors.New("context archive contains an unsupported file; inspect it before continuing")
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > journeyArchiveChunkLimit {
			return nil, 0, errors.New("context archive contains an unsafe or oversized file")
		}
		digest := strings.TrimSuffix(entry.Name(), ".json")
		files[digest] = info.Size()
		total += info.Size()
	}
	if total > journeyArchiveBytesLimit {
		return nil, 0, errJourneyArchiveCapacity
	}
	return files, total, nil
}

func archiveOldest[T any](recent *[]T, archived *[]T, count int) {
	if count <= 0 {
		return
	}
	*archived = append(*archived, (*recent)[:count]...)
	*recent = append([]T(nil), (*recent)[count:]...)
}
func journeyArchiveCount(chunk journeyArchiveChunk) int {
	return len(chunk.BriefRevisions) + len(chunk.DesignRevisions) + len(chunk.SpecificationRevisions) + len(chunk.DesignPlanRevisions)
}

func compactJourneyHistory(dir string, state *journeyState) error {
	chainChunks, chainBytes := 0, 0
	err := walkJourneyArchive(dir, state.ContextID, state.HistoryHead, func(record journeyArchiveRecord) { chainChunks++; chainBytes += len(record.Data) })
	if err != nil {
		return err
	}
	chunk := journeyArchiveChunk{Version: 1, ContextID: state.ContextID, ProjectID: state.Brief.ProjectID, Previous: state.HistoryHead}
	archiveOldest(&state.BriefRevisions, &chunk.BriefRevisions, len(state.BriefRevisions)-journeyRecentRevisions)
	archiveOldest(&state.DesignRevisions, &chunk.DesignRevisions, len(state.DesignRevisions)-journeyRecentRevisions)
	archiveOldest(&state.SpecificationRevisions, &chunk.SpecificationRevisions, len(state.SpecificationRevisions)-journeyRecentRevisions)
	archiveOldest(&state.DesignPlanRevisions, &chunk.DesignPlanRevisions, len(state.DesignPlanRevisions)-journeyRecentRevisions)
	for {
		data, err := json.Marshal(state)
		if err != nil {
			return err
		}
		if len(data) <= journeyActiveTarget {
			break
		}
		// Remove the largest removable prefix first, retaining at least the newest
		// snapshot of each kind where possible. Under the hard size limit, the
		// current snapshot remains in its canonical field even if its duplicate
		// history entry must also move into the archive.
		largest, kind := 0, ""
		candidates := []struct {
			name  string
			value any
			count int
		}{{"brief", state.BriefRevisions, len(state.BriefRevisions)}, {"design", state.DesignRevisions, len(state.DesignRevisions)}, {"spec", state.SpecificationRevisions, len(state.SpecificationRevisions)}, {"handoff", state.DesignPlanRevisions, len(state.DesignPlanRevisions)}}
		for _, candidate := range candidates {
			if candidate.count > 1 || (len(data) > journeyStateLimit && candidate.count > 0) {
				raw, _ := json.Marshal(candidate.value)
				if len(raw) > largest {
					largest = len(raw)
					kind = candidate.name
				}
			}
		}
		switch kind {
		case "brief":
			archiveOldest(&state.BriefRevisions, &chunk.BriefRevisions, 1)
		case "design":
			archiveOldest(&state.DesignRevisions, &chunk.DesignRevisions, 1)
		case "spec":
			archiveOldest(&state.SpecificationRevisions, &chunk.SpecificationRevisions, 1)
		case "handoff":
			archiveOldest(&state.DesignPlanRevisions, &chunk.DesignPlanRevisions, 1)
		default:
			if len(data) > journeyStateLimit {
				return errors.New("current context exceeds the 8 MiB local-state limit; reduce the current content before saving")
			}
			goto compacted
		}
	}
compacted:
	if journeyArchiveCount(chunk) == 0 {
		return nil
	}
	data, err := json.Marshal(chunk)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	total := len(data) + chainBytes
	if len(data) > journeyArchiveChunkLimit || total > journeyArchiveBytesLimit || chainChunks >= journeyArchiveChunksLimit {
		return errJourneyArchiveCapacity
	}
	path, err := guardedProjectPath(dir, ".luci/history", true)
	if err != nil {
		return err
	}
	if err = os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	digest := journeyArchiveDigest(data)
	inventory, diskBytes, err := journeyArchiveInventory(dir)
	if err != nil {
		return err
	}
	if _, exists := inventory[digest]; !exists && (len(inventory) >= journeyArchiveChunksLimit || diskBytes+int64(len(data)) > journeyArchiveBytesLimit) {
		return errJourneyArchiveCapacity
	}
	// An interrupted save may have left this exact chunk. Verify before reuse.
	target, err := guardedProjectPath(dir, journeyArchiveFile(digest), true)
	if err != nil {
		return err
	}
	if info, statErr := os.Lstat(target); statErr == nil {
		if !info.Mode().IsRegular() || info.Size() != int64(len(data)) {
			return errors.New("context archive path is occupied by different content")
		}
		existing, readErr := os.ReadFile(target)
		if readErr != nil || journeyArchiveDigest(existing) != digest {
			return errors.New("context archive path is occupied by different content")
		}
	} else if errors.Is(statErr, os.ErrNotExist) {
		if err = writeProjectAtomic(dir, journeyArchiveFile(digest), data); err != nil {
			return fmt.Errorf("could not preserve older context revisions: %w", err)
		}
	} else {
		return statErr
	}
	if err = syncProjectRunDirectory(path); err != nil {
		return err
	}
	state.HistoryHead = digest
	return nil
}

// Interrupted saves can leave valid immutable chunks outside the attached
// chain. Backups preserve them separately and explicit pruning removes them;
// a cursor never makes them part of the project's accepted history.
func readUnusedJourneyArchives(dir, contextID string, chain []journeyArchiveRecord) ([]journeyArchiveRecord, error) {
	inventory, _, err := journeyArchiveInventory(dir)
	if err != nil {
		return nil, err
	}
	attached := map[string]bool{}
	for _, record := range chain {
		attached[record.Digest] = true
	}
	unused := []journeyArchiveRecord{}
	for digest := range inventory {
		if attached[digest] {
			continue
		}
		data, err := readJourneyArchiveData(dir, digest)
		if err != nil {
			return nil, errors.New("an unused context archive cannot be read")
		}
		chunk, err := validateJourneyArchiveChunk(data, digest, contextID)
		if err != nil {
			return nil, err
		}
		unused = append(unused, journeyArchiveRecord{Digest: digest, Data: data, Chunk: chunk})
	}
	return unused, nil
}

var journeyStagingName = regexp.MustCompile(`^\.luci-write-[0-9]+$`)

// Only explicit writes recover pre-rename staging left by a crashed process.
// Such a file has never become an immutable archive or canonical history head:
// the previous canonical state/chain still retains all accepted revisions.
// An incomplete new save remains unconfirmed and must be retried by its caller.
func discardJourneyArchiveStaging(dir string) error {
	path, err := guardedProjectPath(dir, ".luci/history", false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	folder, err := os.Open(path)
	if err != nil {
		return err
	}
	defer folder.Close()
	entries, err := folder.ReadDir(journeyArchiveChunksLimit + 17)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if len(entries) > journeyArchiveChunksLimit+16 {
		return errJourneyArchiveCapacity
	}
	staged := []string{}
	for _, entry := range entries {
		if !journeyStagingName.MatchString(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > journeyArchiveChunkLimit {
			return errors.New("interrupted context staging is unsafe; inspect the private project storage before saving")
		}
		if len(staged) >= 16 {
			return errJourneyArchiveCapacity
		}
		candidate, err := guardedProjectPath(dir, ".luci/history/"+entry.Name(), false)
		if err != nil {
			return err
		}
		staged = append(staged, candidate)
	}
	for _, candidate := range staged {
		if err := os.Remove(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.New("interrupted context staging could not be cleared; check project storage permissions")
		}
	}
	if len(staged) > 0 {
		return syncProjectRunDirectory(path)
	}
	return nil
}
