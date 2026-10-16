package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Private temporary files/OS locks only. No daemon, socket, command or producer.
func assemblyCatalogFixture(t *testing.T, root string) (*assemblyInputOwner, management.ExecutionActorRef, assemblyCatalogSource, assemblyCatalogPolicy, assemblyCatalogIndex) {
	t.Helper()
	g := executionTestGate(t, root)
	return &assemblyInputOwner{g.api}, management.ExecutionActorRef{Local: true}, assemblyCatalogSource{1, strings.Repeat("a", 40), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), backend.DocumentImageAssemblyRecipeDigest(), strings.Repeat("e", 64), strings.Repeat("f", 64)}, assemblyCatalogPolicy{"fixture-policy-1", strings.Repeat("1", 64)}, assemblyCatalogIndex{1, []assemblyCatalogRef{}}
}

func TestAssemblyCatalogReadonlyAbsenceAndUnauthorizedPublicationHaveNoEffects(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-created")
	o, actor, source, policy, index := assemblyCatalogFixture(t, root)
	if _, err := o.Current(context.Background(), actor); err == nil {
		t.Fatal("absent catalog became current")
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read created state", err)
	}
	o.api.deps.AuthorizeRuntimeInstaller = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if _, err := o.publishMetadata(context.Background(), actor, assemblyCatalogCAS{}, source, policy, index); !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal(err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("forbidden publication created state", err)
	}
}

func TestAssemblyCatalogGlobalCASAcrossIndependentActorScopes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shared")
	left, actor, source, policy, index := assemblyCatalogFixture(t, root)
	right, _, _, _, _ := assemblyCatalogFixture(t, root)
	right.api.execution.scope = strings.Repeat("9", 64)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, owner := range []*assemblyInputOwner{left, right} {
		wg.Add(1)
		go func(o *assemblyInputOwner) {
			defer wg.Done()
			_, err := o.publishMetadata(context.Background(), actor, assemblyCatalogCAS{}, source, policy, index)
			results <- err
		}(owner)
	}
	wg.Wait()
	close(results)
	passed, conflicts := 0, 0
	for err := range results {
		if err == nil {
			passed++
		} else if errors.Is(err, errAssemblyCatalogConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if passed != 1 || conflicts != 1 {
		t.Fatal("separate scopes bypassed global CAS", passed, conflicts)
	}
	current, err := left.Current(context.Background(), actor)
	if err != nil || current.CAS.Revision != 1 {
		t.Fatal(current, err)
	}
	if _, err := right.publishMetadata(context.Background(), actor, current.CAS, source, policy, index); err != nil {
		t.Fatal(err)
	}
	if _, err := left.publishMetadata(context.Background(), actor, current.CAS, source, policy, index); !errors.Is(err, errAssemblyCatalogConflict) {
		t.Fatal("stale writer overwrote current", err)
	}
}

func TestAssemblyCatalogRevocationRecheckedAfterGlobalLock(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shared")
	left, actor, source, policy, index := assemblyCatalogFixture(t, root)
	right, _, _, _, _ := assemblyCatalogFixture(t, root)
	unlock, err := left.lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	first := make(chan struct{}, 1)
	var revoked atomic.Bool
	right.api.deps.AuthorizeRuntimeInstaller = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		if revoked.Load() {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		select {
		case first <- struct{}{}:
		default:
		}
		return management.ExecutionAuthority{}, nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := right.publishMetadata(context.Background(), actor, assemblyCatalogCAS{}, source, policy, index)
		done <- err
	}()
	select {
	case <-first:
	case <-time.After(time.Second):
		t.Fatal("authority was not checked")
	}
	revoked.Store(true)
	unlock()
	select {
	case err = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("bounded publisher did not settle")
	}
	if !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("revoked writer published metadata", err)
	}
	if _, err = os.Lstat(filepath.Join(root, "runtime-inputs", "current.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("revocation changed head", err)
	}
}

func TestAssemblyCatalogImmutableReadbackAndCorruptSelectedObject(t *testing.T) {
	o, actor, source, policy, index := assemblyCatalogFixture(t, filepath.Join(t.TempDir(), "state"))
	cas, err := o.publishMetadata(context.Background(), actor, assemblyCatalogCAS{}, source, policy, index)
	if err != nil {
		t.Fatal(err)
	}
	current, err := o.Current(context.Background(), actor)
	if err != nil || current.Source != source || current.CAS != cas {
		t.Fatal(current, err)
	}
	if got, err := o.reconcilePublication(context.Background(), cas, errors.New("ACK lost after exact publication")); err != nil || got != cas {
		t.Fatal("exact lost ACK not reconciled", got, err)
	}
	wrong := cas
	wrong.Digest = strings.Repeat("8", 64)
	if _, err := o.reconcilePublication(context.Background(), wrong, errors.New("uncertain publication")); err == nil {
		t.Fatal("unrelated readback acknowledged")
	}
	root, _ := o.root()
	ref := current.State.SourceDescriptor
	path := filepath.Join(root, "records", ref.Kind, ref.Digest+".json")
	original, _ := os.ReadFile(path)
	if err = o.writeImmutableMetadata(context.Background(), ref, original); err != nil {
		t.Fatal("exact immutable reuse failed", err)
	}
	changed := append([]byte{}, original...)
	changed[len(changed)-2] ^= 1
	if err = os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = o.Current(context.Background(), actor); err == nil {
		t.Fatal("mutated immutable object accepted")
	}
	if _, err = o.publishMetadata(context.Background(), actor, cas, source, policy, index); err == nil {
		t.Fatal("broken published head overwritten")
	}
	if err = o.writeImmutableMetadata(context.Background(), ref, original); err == nil {
		t.Fatal("immutable address overwritten")
	}
}

func TestAssemblyCatalogCanonicalBoundsAndInventoryNeverConferEligibility(t *testing.T) {
	index := assemblyCatalogIndex{1, make([]assemblyCatalogRef, 0)}
	for i := 0; i < assemblyCatalogRefLimit; i++ {
		index.Records = append(index.Records, assemblyCatalogRef{1, "debian-sources", fmt.Sprintf("%064x", i), 1})
	}
	if !validAssemblyCatalogIndex(index) {
		t.Fatal("bounded sorted inventory refused")
	}
	index.Records = append(index.Records, index.Records[0])
	if validAssemblyCatalogIndex(index) {
		t.Fatal("unbounded/duplicate inventory accepted")
	}
	index.Records = index.Records[:assemblyCatalogRefLimit]
	index.Records[0].Kind = "caller-verified"
	if validAssemblyCatalogIndex(index) {
		t.Fatal("arbitrary proof kind accepted")
	}
	for _, raw := range []string{`{"schema":1,"schema":1,"records":[]}`, `{"schema":1,"records":[],"verified":true}`, `{"schema":1,"records":[]} {}`, ` {"schema":1,"records":[]}`, `{"schema":1,"records":null}`} {
		var parsed assemblyCatalogIndex
		if decodeAssemblyCatalog([]byte(raw), assemblyCatalogMetadataLimit, &parsed) == nil && validAssemblyCatalogIndex(parsed) {
			t.Fatal("noncanonical/invalid inventory accepted", raw)
		}
	}
	o, actor, source, policy, _ := assemblyCatalogFixture(t, filepath.Join(t.TempDir(), "state"))
	// Missing referenced producer objects are just inventory claims, explicitly
	// not accepted proof. This owner has no Adopt/ResolveCurrent or execution API.
	claims := assemblyCatalogIndex{1, []assemblyCatalogRef{{1, "input-closure", strings.Repeat("7", 64), 1}}}
	if _, err := o.publishMetadata(context.Background(), actor, assemblyCatalogCAS{}, source, policy, claims); err != nil {
		t.Fatal(err)
	}
	if _, err := o.readMetadata(context.Background(), claims.Records[0]); err == nil {
		t.Fatal("missing claimed producer evidence manufactured")
	}
}

func TestAssemblyCatalogBoundedRetainedHistoryRefusesNewObjectWithoutPruning(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "evidence-index")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < assemblyCatalogRetainedLimit; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("%064x.json", i)), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := boundedAssemblyCatalogHistory(context.Background(), root, 1); err == nil {
		t.Fatal("retained object limit bypassed")
	}
	if data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("%064x.json", 0))); err != nil || string(data) != "x" {
		t.Fatal("old history pruned", err)
	}
	var parsed assemblyCatalogIndex
	data, _ := json.Marshal(assemblyCatalogIndex{1, []assemblyCatalogRef{}})
	if decodeAssemblyCatalog(data, assemblyCatalogMetadataLimit, &parsed) != nil {
		t.Fatal("valid canonical metadata refused")
	}
}
