package dockerbe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type imagePublicationClient interface {
	ImageTag(context.Context, string, string) error
	ImagePush(context.Context, string, image.PushOptions) (io.ReadCloser, error)
	DistributionInspect(context.Context, string, string) (registry.DistributionInspect, error)
}

var _ backend.ImagePublisher = (*Backend)(nil)

func validPublicationRepository(v string) bool {
	if len(v) < 3 || len(v) > 240 || strings.ContainsAny(v, "@\\\x00\r\n") {
		return false
	}
	named, err := reference.ParseNormalizedNamed(v)
	if err != nil || named.String() != v || reference.IsNameOnly(named) == false {
		return false
	}
	// Reject implicit docker.io/library rewrites and daemon-local short names.
	host := reference.Domain(named)
	return strings.Contains(v, "/") && (strings.Contains(host, ".") || strings.Contains(host, ":")) && host != "localhost"
}
func publicationDigest(v string) bool {
	return strings.HasPrefix(v, "sha256:") && fenceHex(strings.TrimPrefix(v, "sha256:"), 32)
}

func (b *Backend) publicationImage(ctx context.Context, p backend.ImagePublication) error {
	if err := b.previewIdentity(ctx, p.Identity); err != nil {
		return err
	}
	api, ok := b.api.(imageResourceClient)
	if !ok {
		return backend.ErrUnsupported
	}
	row, err := api.ImageInspect(ctx, p.ImageID)
	if err != nil || row.ID != p.ImageID || row.Config == nil || row.Size <= 0 {
		return backend.ErrUnavailable
	}
	for k, v := range previewIdentityLabels(p.Identity) {
		if row.Config.Labels[k] != v {
			return backend.ErrLaunchFenceOwnership
		}
	}
	return ctx.Err()
}
func (b *Backend) PublishPreviewImage(ctx context.Context, p backend.ImagePublication, before func(context.Context) error) (backend.ImagePublicationResult, error) {
	out := backend.ImagePublicationResult{}
	if before == nil || !validPreviewIdentity(p.Identity, true) || p.Identity.Kind != "" || !previewImageID(p.ImageID) || !validPublicationRepository(p.Repository) || p.Tag != "luci-"+p.Identity.AllocationID || len(p.Tag) > 128 || len(p.RegistryAuth) > 32<<10 || strings.ContainsAny(p.RegistryAuth, "\x00\r\n") {
		return out, backend.ErrUnsupported
	}
	api, ok := b.api.(imagePublicationClient)
	if !ok {
		return out, backend.ErrUnsupported
	}
	images, ok := b.api.(imageResourceClient)
	if !ok {
		return out, backend.ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := b.publicationImage(ctx, p); err != nil {
		return out, err
	}
	target := p.Repository + ":" + p.Tag
	// One original new tag. Never overwrite/adopt a pre-existing remote or local
	// tag, even if it currently points at the same bytes. This is not remote CAS:
	// registry-side tag mutation by another principal can still race the request.
	if _, err := api.DistributionInspect(ctx, target, p.RegistryAuth); !errdefs.IsNotFound(err) {
		return out, backend.ErrLaunchFenceOwnership
	}
	if _, err := images.ImageInspect(ctx, target); !errdefs.IsNotFound(err) {
		return out, backend.ErrLaunchFenceOwnership
	}
	if err := before(ctx); err != nil {
		return out, err
	}
	if err := b.publicationImage(ctx, p); err != nil {
		return out, err
	}
	if err := api.ImageTag(ctx, p.ImageID, target); err != nil {
		return out, backend.ErrUnavailable
	}
	if err := before(ctx); err != nil {
		return out, err
	}
	if err := b.publicationImage(ctx, p); err != nil {
		return out, err
	}
	tagged, err := images.ImageInspect(ctx, target)
	if err != nil || tagged.ID != p.ImageID {
		return out, backend.ErrLaunchFenceOwnership
	}
	// PrivilegeFunc remains nil: no ambient credentials, interactive login,
	// retry, dependency installation or second push under any outcome.
	body, err := api.ImagePush(ctx, target, image.PushOptions{RegistryAuth: p.RegistryAuth})
	if err != nil || body == nil {
		return out, backend.ErrUnavailable
	}
	closed := false
	defer func() {
		if !closed {
			_ = body.Close()
		}
	}()
	limit := &io.LimitedReader{R: body, N: 4 << 20}
	decoder := json.NewDecoder(limit)
	var digest string
	failed := false
	for {
		var row struct {
			Error       string `json:"error"`
			ErrorDetail *struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
			Aux json.RawMessage `json:"aux"`
		}
		err = decoder.Decode(&row)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || limit.N == 0 || ctx.Err() != nil {
			return out, backend.ErrUnavailable
		}
		if row.Error != "" || row.ErrorDetail != nil {
			failed = true
		}
		if len(row.Aux) > 0 && string(row.Aux) != "null" {
			var aux struct {
				Tag    string `json:"Tag"`
				Digest string `json:"Digest"`
				Size   int64  `json:"Size"`
			}
			if json.Unmarshal(row.Aux, &aux) != nil || !publicationDigest(aux.Digest) || aux.Tag != p.Tag || aux.Size <= 0 || digest != "" {
				return out, backend.ErrUnavailable
			}
			digest = aux.Digest
		}
	}
	if limit.N == 0 || ctx.Err() != nil {
		return out, backend.ErrUnavailable
	}
	closeErr := body.Close()
	closed = true
	if closeErr != nil {
		return out, backend.ErrUnavailable
	}
	// Actual EOF and a parsed daemon error settle transport, not successful
	// publication. Missing/malformed original success remains unknown.
	if failed {
		out.Settled = true
		return out, backend.ErrUnavailable
	}
	if digest == "" {
		return out, backend.ErrUnavailable
	}
	out.Settled = true
	if err = b.publicationImage(ctx, p); err != nil {
		return out, err
	}
	observed, err := api.DistributionInspect(ctx, p.Repository+"@"+digest, p.RegistryAuth)
	if err != nil || observed.Descriptor.Digest.String() != digest {
		return out, backend.ErrUnavailable
	}
	out.Reference, out.Digest = p.Repository+"@"+digest, digest
	return out, nil
}
