package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

const assemblyCatalogMetadataLimit = 128 << 10
const assemblyCatalogRecordLimit = 64 << 10
const assemblyCatalogRefLimit = 512
const assemblyCatalogRetainedLimit = 1024

var errAssemblyCatalogConflict = errors.New("installer input metadata changed")
var errAssemblyCatalogInvalid = errors.New("installer input metadata is unavailable or invalid")

// This is the existing runtime installer's private metadata owner, not another
// trust authority. Nothing registers it or calls it at boot. The catalog selects
// immutable metadata; neither publication nor membership proves domain validity.
// Accepted producer records, adoption and ResolveCurrent are deliberately absent
// until their actual typed domain constructors exist.
type assemblyInputOwner struct{ api *API }

// Private explicit construction only; retains the existing installer authority
// and installation state owner. No new Config/Deps field or startup hook.
func (a *API) documentImageAssemblyInputOwner() (*assemblyInputOwner, error) {
	if a == nil || a.execution == nil || a.execution.api != a || !filepath.IsAbs(a.execution.path) {
		return nil, errAssemblyCatalogInvalid
	}
	return &assemblyInputOwner{api: a}, nil
}

type assemblyCatalogCAS struct {
	Revision uint64 `json:"revision"`
	Digest   string `json:"digest"`
}

type assemblyCatalogRef struct {
	Schema int    `json:"schema"`
	Kind   string `json:"kind"`
	Digest string `json:"digest"`
	Bytes  int64  `json:"bytes"`
}

type assemblyCatalogPolicy struct {
	Revision string `json:"revision"`
	Digest   string `json:"digest"`
}

// An immutable source configuration snapshot, never an allocation or a grant.
type assemblyCatalogSource struct {
	Schema                int    `json:"schema"`
	SourceCommit          string `json:"sourceCommit"`
	SourceDigest          string `json:"sourceDigest"`
	InputManifestDigest   string `json:"inputManifestDigest"`
	HelperDigest          string `json:"helperDigest"`
	RecipeDigest          string `json:"recipeDigest"`
	DependencyLockDigest  string `json:"dependencyLockDigest"`
	SourceRetentionDigest string `json:"sourceRetentionDigest"`
}

// References are inventory claims. A future domain resolver must independently
// validate typed producer evidence and its current policy before using any ref.
type assemblyCatalogIndex struct {
	Schema  int                  `json:"schema"`
	Records []assemblyCatalogRef `json:"records"`
}

type assemblyInputCatalogState struct {
	Schema           int                   `json:"schema"`
	Revision         uint64                `json:"revision"`
	SourceDescriptor assemblyCatalogRef    `json:"sourceDescriptor"`
	Policy           assemblyCatalogPolicy `json:"policy"`
	EvidenceIndex    assemblyCatalogRef    `json:"evidenceIndex"`
}

type assemblyInputCatalogSnapshot struct {
	State  assemblyInputCatalogState
	CAS    assemblyCatalogCAS
	Source assemblyCatalogSource
	Index  assemblyCatalogIndex
}

func assemblyCatalogDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assemblyCatalogHex(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}

func assemblyCatalogKind(kind string) bool {
	switch kind {
	case "source-descriptor", "evidence-index", "producer-raw-manifest", "official-root-policy", "key-observations", "release-signatures", "signed-index-bindings", "debian-binaries", "debian-sources", "base-rootfs", "wheels", "dependency-closure", "expected-installed-files", "notices-and-sources", "luci-source-helper", "input-closure":
		return true
	}
	return false
}

func validAssemblyCatalogRef(ref assemblyCatalogRef) bool {
	limit := int64(assemblyCatalogRecordLimit)
	if ref.Kind == "evidence-index" {
		limit = assemblyCatalogMetadataLimit
	}
	return ref.Schema == 1 && assemblyCatalogKind(ref.Kind) && assemblyCatalogHex(ref.Digest) && ref.Bytes > 0 && ref.Bytes <= limit
}

func validAssemblyCatalogSource(source assemblyCatalogSource) bool {
	return source.Schema == 1 && len(source.SourceCommit) == 40 && strings.Trim(source.SourceCommit, "0123456789abcdef") == "" && assemblyCatalogHex(source.SourceDigest) && assemblyCatalogHex(source.InputManifestDigest) && assemblyCatalogHex(source.HelperDigest) && source.RecipeDigest == backend.DocumentImageAssemblyRecipeDigest() && assemblyCatalogHex(source.DependencyLockDigest) && assemblyCatalogHex(source.SourceRetentionDigest)
}

func validAssemblyCatalogPolicy(policy assemblyCatalogPolicy) bool {
	return len(policy.Revision) > 0 && len(policy.Revision) <= 128 && strings.Trim(policy.Revision, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-") == "" && assemblyCatalogHex(policy.Digest)
}

func validAssemblyCatalogIndex(index assemblyCatalogIndex) bool {
	if index.Schema != 1 || index.Records == nil || len(index.Records) > assemblyCatalogRefLimit {
		return false
	}
	previous := ""
	for _, ref := range index.Records {
		key := ref.Kind + "/" + ref.Digest
		if !validAssemblyCatalogRef(ref) || ref.Kind == "evidence-index" || ref.Kind == "source-descriptor" || key <= previous {
			return false
		}
		previous = key
	}
	return true
}

// Exact canonical encoding also rejects duplicate keys, unknown fields, trailing
// values and alternate representations. No generic JSON payload is interpreted.
func decodeAssemblyCatalog(data []byte, limit int, target any) error {
	if len(data) == 0 || len(data) > limit {
		return errAssemblyCatalogInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return errAssemblyCatalogInvalid
	}
	canonical, err := json.Marshal(target)
	if err != nil || !bytes.Equal(canonical, data) {
		return errAssemblyCatalogInvalid
	}
	return nil
}

func (o *assemblyInputOwner) root() (string, error) {
	if o == nil || o.api == nil || o.api.execution == nil || !filepath.IsAbs(o.api.execution.path) {
		return "", errAssemblyCatalogInvalid
	}
	parent := filepath.Dir(o.api.execution.path)
	if _, err := hostpath.ResolveOutside(o.api.projectsDir(), parent); err != nil {
		return "", err
	}
	if err := codexRegularPath(filepath.VolumeName(parent)+string(filepath.Separator), parent); err != nil {
		return "", err
	}
	return filepath.Join(parent, "runtime-inputs"), nil
}

func (o *assemblyInputOwner) authorize(ctx context.Context, actor management.ExecutionActorRef) error {
	if o == nil || o.api == nil {
		return management.ErrExecutionAuthority
	}
	_, err := o.api.authorizeRuntimeInstaller(ctx, actor)
	return err
}

// Read-only: missing state is unavailable; it never creates dirs, locks or files.
func (o *assemblyInputOwner) Current(ctx context.Context, actor management.ExecutionActorRef) (assemblyInputCatalogSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := o.authorize(ctx, actor); err != nil {
		return assemblyInputCatalogSnapshot{}, err
	}
	snapshot, err := o.readCurrent(ctx)
	if err == nil {
		err = o.authorize(ctx, actor)
	}
	if err != nil {
		return assemblyInputCatalogSnapshot{}, err
	}
	return snapshot, err
}

func (o *assemblyInputOwner) readCurrent(ctx context.Context) (assemblyInputCatalogSnapshot, error) {
	var out assemblyInputCatalogSnapshot
	root, err := o.root()
	if err != nil || ctx.Err() != nil {
		return out, errAssemblyCatalogInvalid
	}
	data, err := codexRead(filepath.Dir(root), filepath.Join(root, "current.json"), assemblyCatalogRecordLimit)
	if err != nil {
		return out, err
	}
	if decodeAssemblyCatalog(data, assemblyCatalogRecordLimit, &out.State) != nil || out.State.Schema != 1 || out.State.Revision == 0 || !validAssemblyCatalogPolicy(out.State.Policy) || !validAssemblyCatalogRef(out.State.SourceDescriptor) || out.State.SourceDescriptor.Kind != "source-descriptor" || !validAssemblyCatalogRef(out.State.EvidenceIndex) || out.State.EvidenceIndex.Kind != "evidence-index" {
		return out, errAssemblyCatalogInvalid
	}
	out.CAS = assemblyCatalogCAS{Revision: out.State.Revision, Digest: assemblyCatalogDigest(data)}
	source, err := o.readMetadata(ctx, out.State.SourceDescriptor)
	if err != nil || decodeAssemblyCatalog(source, assemblyCatalogRecordLimit, &out.Source) != nil || !validAssemblyCatalogSource(out.Source) {
		return assemblyInputCatalogSnapshot{}, errAssemblyCatalogInvalid
	}
	index, err := o.readMetadata(ctx, out.State.EvidenceIndex)
	if err != nil || decodeAssemblyCatalog(index, assemblyCatalogMetadataLimit, &out.Index) != nil || !validAssemblyCatalogIndex(out.Index) || ctx.Err() != nil {
		return assemblyInputCatalogSnapshot{}, errAssemblyCatalogInvalid
	}
	return out, nil
}

func (o *assemblyInputOwner) readMetadata(ctx context.Context, ref assemblyCatalogRef) ([]byte, error) {
	if !validAssemblyCatalogRef(ref) || ctx.Err() != nil {
		return nil, errAssemblyCatalogInvalid
	}
	root, err := o.root()
	if err != nil {
		return nil, err
	}
	data, err := codexRead(filepath.Dir(root), filepath.Join(root, "records", ref.Kind, ref.Digest+".json"), ref.Bytes)
	if err != nil || int64(len(data)) != ref.Bytes || assemblyCatalogDigest(data) != ref.Digest || ctx.Err() != nil {
		return nil, errAssemblyCatalogInvalid
	}
	return data, nil
}

// One fixed installation-wide lock, deliberately independent of account scope.
// The existing per-child scoped lifecycle lock is NOT a catalog CAS lock.
func (o *assemblyInputOwner) lock(ctx context.Context) (func(), error) {
	parent, err := o.api.execution.prepareExecutionDirectory()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(parent, "runtime-inputs")
	if err = os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if err = codexRegularPath(parent, root); err == nil {
		err = codexPrivateDir(root)
	}
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, "catalog.lock")
	if info, e := os.Lstat(path); e == nil && !info.Mode().IsRegular() {
		return nil, errAssemblyCatalogInvalid
	} else if e != nil && !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return nil, err
	}
	fail := func(err error) (func(), error) { _ = file.Close(); return nil, err }
	opened, err := file.Stat()
	named, namedErr := os.Lstat(path)
	if err != nil || namedErr != nil || !opened.Mode().IsRegular() || !os.SameFile(opened, named) {
		return fail(errAssemblyCatalogInvalid)
	}
	if err = codexRegularPath(parent, path); err == nil {
		err = codexPrivateFile(path)
	}
	if err != nil {
		return fail(err)
	}
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	unlock, err := lockExecutionLedger(bounded, file)
	if err != nil {
		return fail(err)
	}
	var once sync.Once
	return func() { once.Do(func() { unlock(); _ = file.Close() }) }, nil
}

// A metadata-only transaction. It cannot adopt an input-closure record, return a
// resolved closure, grant execution or accept caller Verified/Authenticated flags.
// The eventual domain constructor must validate actual producer evidence before
// selecting a closure; this unregistered storage primitive has no production caller.
func (o *assemblyInputOwner) publishMetadata(ctx context.Context, actor management.ExecutionActorRef, expected assemblyCatalogCAS, source assemblyCatalogSource, policy assemblyCatalogPolicy, index assemblyCatalogIndex) (assemblyCatalogCAS, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if !validAssemblyCatalogSource(source) || !validAssemblyCatalogPolicy(policy) || !validAssemblyCatalogIndex(index) || expected != (assemblyCatalogCAS{}) && (expected.Revision == 0 || !assemblyCatalogHex(expected.Digest)) {
		return assemblyCatalogCAS{}, errAssemblyCatalogInvalid
	}
	if err := o.authorize(ctx, actor); err != nil {
		return assemblyCatalogCAS{}, err
	}
	unlock, err := o.lock(ctx)
	if err != nil {
		return assemblyCatalogCAS{}, err
	}
	defer unlock()
	if err = o.authorize(ctx, actor); err != nil {
		return assemblyCatalogCAS{}, err
	}
	old, err := o.readCurrent(ctx)
	if errors.Is(err, os.ErrNotExist) && expected == (assemblyCatalogCAS{}) {
		// Absence only means the exact current head is absent, not a missing
		// selected source/index. A broken published head must never be replaced.
		root, _ := o.root()
		if _, e := os.Lstat(filepath.Join(root, "current.json")); !errors.Is(e, os.ErrNotExist) {
			return assemblyCatalogCAS{}, errAssemblyCatalogInvalid
		}
		old = assemblyInputCatalogSnapshot{}
	} else if err != nil {
		return assemblyCatalogCAS{}, err
	}
	if old.CAS != expected || old.State.Revision == math.MaxUint64 {
		return assemblyCatalogCAS{}, errAssemblyCatalogConflict
	}
	return o.publishCatalogHeadLocked(ctx, actor, old, source, policy, index)
}

// Both callers already own the same installation-wide catalog lock. This body
// performs storage only; callers retain their distinct metadata/domain gates.
// No arbitrary validator callback and no recursive publishMetadata lock.
func (o *assemblyInputOwner) publishCatalogHeadLocked(ctx context.Context, actor management.ExecutionActorRef, old assemblyInputCatalogSnapshot, source assemblyCatalogSource, policy assemblyCatalogPolicy, index assemblyCatalogIndex) (assemblyCatalogCAS, error) {
	if old.State.Revision == math.MaxUint64 || !validAssemblyCatalogSource(source) || !validAssemblyCatalogPolicy(policy) || !validAssemblyCatalogIndex(index) { return assemblyCatalogCAS{}, errAssemblyCatalogInvalid }
	sourceData, _ := json.Marshal(source)
	indexData, _ := json.Marshal(index)
	sourceRef := assemblyCatalogRef{1, "source-descriptor", assemblyCatalogDigest(sourceData), int64(len(sourceData))}
	indexRef := assemblyCatalogRef{1, "evidence-index", assemblyCatalogDigest(indexData), int64(len(indexData))}
	for _, item := range []struct {
		ref  assemblyCatalogRef
		data []byte
	}{{sourceRef, sourceData}, {indexRef, indexData}} {
		if err := o.writeImmutableMetadata(ctx, item.ref, item.data); err != nil {
			return assemblyCatalogCAS{}, err
		}
	}
	state := assemblyInputCatalogState{1, old.State.Revision + 1, sourceRef, policy, indexRef}
	data, _ := json.Marshal(state)
	next := assemblyCatalogCAS{state.Revision, assemblyCatalogDigest(data)}
	root, _ := o.root()
	if err := o.authorize(ctx, actor); err != nil {
		return assemblyCatalogCAS{}, err
	}
	err := writeExecutionLedger(filepath.Join(root, "current.json"), data)
	return o.reconcilePublication(ctx, next, err)
}

// Exact lost-ACK reconciliation; a different/newer/missing head is uncertainty,
// never permission to retry or overwrite. Objects not selected stay unaccepted.
func (o *assemblyInputOwner) reconcilePublication(ctx context.Context, expected assemblyCatalogCAS, publicationErr error) (assemblyCatalogCAS, error) {
	actual, err := o.readCurrent(ctx)
	if err == nil && actual.CAS == expected {
		return expected, nil
	}
	if publicationErr != nil {
		return assemblyCatalogCAS{}, publicationErr
	}
	return assemblyCatalogCAS{}, errAssemblyCatalogConflict
}

func (o *assemblyInputOwner) writeImmutableMetadata(ctx context.Context, ref assemblyCatalogRef, data []byte) error {
	if !validAssemblyCatalogRef(ref) || ref.Kind != "source-descriptor" && ref.Kind != "evidence-index" || ref.Bytes != int64(len(data)) || ref.Digest != assemblyCatalogDigest(data) || ctx.Err() != nil {
		return errAssemblyCatalogInvalid
	}
	return o.writeImmutableAssemblyObject(ctx, ref, data, false)
}

// Private shared no-replace persistence, not an adoption entry point. Raw files
// have their own closed namespace; they cannot be selected as catalog domains.
func (o *assemblyInputOwner) writeImmutableAssemblyObject(ctx context.Context, ref assemblyCatalogRef, data []byte, raw bool) error {
	if ref.Bytes != int64(len(data)) || ref.Digest != assemblyCatalogDigest(data) || ctx.Err() != nil || !assemblyCatalogHex(ref.Digest) { return errAssemblyCatalogInvalid }
	suffix := ".json"
	if raw {
		if ref.Kind != "producer-raw" || len(data) > 1<<20 { return errAssemblyCatalogInvalid }
		suffix = ".raw"
	} else if !validAssemblyCatalogRef(ref) { return errAssemblyCatalogInvalid }
	root, err := o.root()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "records", ref.Kind)
	for _, path := range []string{filepath.Join(root, "records"), dir} {
		if err = os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		if err = codexRegularPath(root, path); err == nil {
			err = codexPrivateDir(path)
		}
		if err != nil {
			return err
		}
	}
	path := filepath.Join(dir, ref.Digest+suffix)
	readExact := func() ([]byte,error) {
		if !raw { return o.readMetadata(ctx,ref) }
		old,e := codexRead(filepath.Dir(root),path,ref.Bytes)
		if e != nil || int64(len(old)) != ref.Bytes || assemblyCatalogDigest(old) != ref.Digest { return nil,errAssemblyCatalogInvalid }
		return old,nil
	}
	if _, err = os.Lstat(path); err == nil {
		old, readErr := readExact()
		if readErr != nil || !bytes.Equal(old, data) {
			return errAssemblyCatalogInvalid
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err = boundedAssemblyCatalogHistory(ctx, filepath.Join(root, "records"), int64(len(data))); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".input-metadata-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name) // Only this positively unallocated metadata temp file.
	if err = codexPrivateFile(name); err == nil {
		var count int
		count, err = f.Write(data)
		if err == nil && count != len(data) {
			err = io.ErrShortWrite
		}
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// No-replace publication; fsync/write-through includes directory lifetime.
	// On lost ACK only exact content readback may settle, never overwrite/retry.
	publishErr := publishAssemblyCatalogObject(name, path)
	old, readErr := readExact()
	if readErr == nil && bytes.Equal(old, data) {
		return nil
	}
	if publishErr != nil {
		return publishErr
	}
	return errAssemblyCatalogInvalid
}

// Retained metadata is never silently pruned. Exhaustion refuses a new object;
// exact old heads remain readable and can still support resource-only cleanup.
func boundedAssemblyCatalogHistory(ctx context.Context, root string, additional int64) error {
	count, entries, total := 0, 0, additional
	var scan func(string, int) error
	scan = func(dir string, depth int) error {
		if err := codexRegularPath(root, dir); err != nil {
			return err
		}
		file, err := os.Open(dir)
		if err != nil {
			return err
		}
		defer file.Close()
		for {
			batch, readErr := file.ReadDir(32) // Never read an unbounded directory.
			for _, entry := range batch {
				entries++
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if entries > assemblyCatalogRetainedLimit+32 || entry.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
					return errAssemblyCatalogInvalid
				}
				path := filepath.Join(dir, entry.Name())
				if entry.IsDir() {
					if depth != 0 || !assemblyCatalogKind(entry.Name()) {
						return errAssemblyCatalogInvalid
					}
					if err = scan(path, depth+1); err != nil {
						return err
					}
					continue
				}
				suffix, limit := ".json", int64(assemblyCatalogMetadataLimit)
				if filepath.Base(dir) == "producer-raw" { suffix, limit = ".raw", 1<<20 }
				if depth != 1 || !strings.HasSuffix(entry.Name(), suffix) || !assemblyCatalogHex(strings.TrimSuffix(entry.Name(), suffix)) {
					return errAssemblyCatalogInvalid
				}
				info, err := entry.Info()
				if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || suffix == ".json" && info.Size() == 0 || info.Size() > limit {
					return errAssemblyCatalogInvalid
				}
				count++
				total += info.Size()
				if count >= assemblyCatalogRetainedLimit || total > 64<<20 {
					return errAssemblyCatalogInvalid
				}
				if err = hostpath.OneName(path); err != nil {
					return err
				}
			}
			if readErr == io.EOF {
				return nil
			}
			if readErr != nil {
				return readErr
			}
			if len(batch) == 0 {
				return errAssemblyCatalogInvalid
			}
		}
	}
	return scan(root, 0)
}
