package dockerbe_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type heldCreationDocker struct {
	*fakeDocker
	entered, release          chan struct{}
	inspected, releaseInspect chan struct{}
	inspectCalls              atomic.Int32
}

func (f *heldCreationDocker) ContainerCreate(context.Context, *container.Config, *container.HostConfig, *network.NetworkingConfig, *ocispec.Platform, string) (container.CreateResponse, error) {
	close(f.entered)
	<-f.release
	return container.CreateResponse{}, errors.New("create response interrupted")
}
func (f *heldCreationDocker) ContainerInspect(_ context.Context, name string) (container.InspectResponse, error) {
	if f.inspectCalls.Add(1) == 1 {
		return container.InspectResponse{}, notFoundErr{name}
	}
	if f.inspected != nil && f.inspectCalls.Load() == 2 {
		close(f.inspected)
		<-f.releaseInspect
	}
	return inspectOf("owned-created-ref", "created", "", "", labelsFor("s-3f2a91c0", "sess-1")), nil
}

func TestCreatedObservationRetainsActiveAdmissionAcrossDelayedInspect(t *testing.T) {
	f := &heldCreationDocker{fakeDocker: &fakeDocker{}, entered: make(chan struct{}), release: make(chan struct{}), inspected: make(chan struct{}), releaseInspect: make(chan struct{})}
	b, err := dockerbe.New(dockerbe.Config{Instance: testInstance, Client: f})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := b.Create(context.Background(), validSpec()); done <- err }()
	<-f.entered
	observed := make(chan backend.Sandbox, 1)
	go func() {
		sb, err := b.Get(context.Background(), validSpec().ID)
		if err != nil {
			t.Error(err)
		}
		observed <- sb
	}()
	<-f.inspected
	// The original create finishes after Get begins, before its older Created
	// response returns. The read must still carry its original active marker.
	close(f.release)
	if err := <-done; err == nil {
		t.Fatal("fixture did not interrupt create")
	}
	close(f.releaseInspect)
	sb := <-observed
	if !sb.CreatedOnly || !sb.CreationInFlight {
		t.Fatal("delayed observation lost create admission", sb)
	}
	sb, err = b.Get(context.Background(), validSpec().ID)
	if err != nil || !sb.CreatedOnly || sb.CreationInFlight {
		t.Fatal("completed create admission remained active", sb, err)
	}
}

func TestConcurrentSameIDCreateCannotClearAnotherActiveAdmission(t *testing.T) {
	f := &heldCreationDocker{fakeDocker: &fakeDocker{}, entered: make(chan struct{}), release: make(chan struct{})}
	b, err := dockerbe.New(dockerbe.Config{Instance: testInstance, Client: f})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := b.Create(context.Background(), validSpec()); done <- err }()
	<-f.entered
	_, err = b.Create(context.Background(), validSpec())
	if !errors.Is(err, backend.ErrAlreadyExists) {
		t.Fatal(err)
	}
	sb, err := b.Get(context.Background(), validSpec().ID)
	close(f.release)
	<-done
	if err != nil || !sb.CreationInFlight || !sb.CreatedOnly {
		t.Fatal("duplicate cleared original admission", sb, err)
	}
}
