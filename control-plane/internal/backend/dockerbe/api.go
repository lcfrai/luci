// Package dockerbe runs sandboxes as Docker containers. This is not
// scaffolding: docs/PROGRAM.md ships a "local machine" alongside cloud
// machines, and this package is that feature. katabe is the dormant one.
package dockerbe

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// APIClient is the slice of the Docker SDK this package uses. *client.Client
// satisfies it structurally, so production passes the real client and tests
// pass a fake — there is no Docker daemon on a CI agent.
//
// Signatures copied verbatim from
// github.com/docker/docker@v28.3.3+incompatible/client/client_interfaces.go,
// lines 72–103. They must not be "tidied": v28 renamed types.Container to
// container.Summary and types.ContainerJSON to container.InspectResponse, and a
// signature written from memory will not compile. docker.go asserts
// *client.Client still satisfies this, so an SDK bump breaks the build here
// rather than at run time.
type APIClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	Ping(ctx context.Context) (types.Ping, error)
	Close() error
}
