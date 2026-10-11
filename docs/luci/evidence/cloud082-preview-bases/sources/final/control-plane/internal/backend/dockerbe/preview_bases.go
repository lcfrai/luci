package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Supported cold-image policy, not a measurement or a daemon disk/cgroup quota.
// Each cold FROM stage reserves twice this maximum virtual size; preparation
// also reserves context copies and work space, with an aggregate admission cap.
const previewColdBaseVirtualLimit uint64 = 2 << 30
const previewColdDiskLimit uint64 = 32 << 30

type previewContextPlan struct {
	prepared []byte
	digest   string
	images   []string
	bases    []backend.PreviewBaseImage
	demand   uint64
}

type previewPullClient interface {
	ImagePull(context.Context, string, image.PullOptions) (io.ReadCloser, error)
}

// Classic COPY --from may implicitly pull an external image even when
// PullParent is false. Only completed prior stages belong to this FROM plan.
// Validate the whole file before inspecting any base or allocating resources.
func validatePreviewCopyStages(lines []string) error {
	aliases := map[string]int{}
	current := -1
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if strings.EqualFold(fields[0], "FROM") {
			current++
			if len(fields) == 4 && strings.EqualFold(fields[2], "AS") {
				aliases[strings.ToLower(fields[3])] = current
			}
			continue
		}
		if !strings.EqualFold(fields[0], "COPY") {
			continue
		}
		for _, field := range fields[1:] {
			if !strings.HasPrefix(field, "--") {
				break
			}
			flag, value, hasValue := strings.Cut(field, "=")
			// Docker keeps quoted flag values together. With this bounded
			// whitespace split, reject quotes spanning fields or escaped flags
			// before skipping another flag: otherwise a quoted --chown value
			// could hide a later --from. Whole quoted values remain supported.
			if strings.ContainsAny(flag, "\\\"'") {
				return backend.ErrUnsupported
			}
			if len(value) > 0 && (value[0] == '\'' || value[0] == '"') {
				if len(value) < 2 || value[len(value)-1] != value[0] {
					return backend.ErrUnsupported
				}
				value = value[1 : len(value)-1]
			}
			if strings.ContainsAny(value, "\\\"'") {
				return backend.ErrUnsupported
			}
			if !strings.EqualFold(flag, "--from") {
				continue
			}
			if !hasValue || value == "" {
				return backend.ErrUnsupported
			}
			if stage, ok := aliases[strings.ToLower(value)]; ok && stage < current {
				continue
			}
			stage, err := strconv.Atoi(value)
			if err != nil || strconv.Itoa(stage) != value || stage < 0 || stage >= current {
				return backend.ErrUnsupported
			}
		}
	}
	return nil
}

func previewBaseReference(raw string) (string, error) {
	if previewImageID(raw) {
		return raw, nil
	}
	if len(raw) > 512 {
		return "", backend.ErrUnsupported
	}
	ref, err := reference.ParseDockerRef(raw)
	if err != nil {
		return "", backend.ErrUnsupported
	}
	return ref.String(), nil
}

func previewPlanBase(ctx context.Context, images imageResourceClient, raw, canonical string, resolved map[string]backend.PreviewBaseImage) (backend.PreviewBaseImage, error) {
	planned := backend.PreviewBaseImage{Reference: canonical}
	lookup := raw
	if resolved != nil {
		var found bool
		planned, found = resolved[canonical]
		if !found || planned.Reference != canonical || !previewImageID(planned.ImageID) {
			return backend.PreviewBaseImage{}, backend.ErrLaunchFenceOwnership
		}
		lookup = planned.ImageID
	}
	row, err := images.ImageInspect(ctx, lookup)
	if errdefs.IsNotFound(err) && resolved == nil && !previewImageID(raw) {
		planned.VirtualSizeBytes = previewColdBaseVirtualLimit
		return planned, nil
	}
	if err != nil {
		return backend.PreviewBaseImage{}, fmt.Errorf("preview base image %q cannot be inspected: %w", canonical, backend.ErrUnavailable)
	}
	if !runtimeBaseValid(row) {
		return backend.PreviewBaseImage{}, backend.ErrUnsupported
	}
	if resolved != nil && (row.ID != planned.ImageID || uint64(row.Size) != planned.VirtualSizeBytes) {
		return backend.PreviewBaseImage{}, backend.ErrLaunchFenceOwnership
	}
	planned.ImageID, planned.VirtualSizeBytes = row.ID, uint64(row.Size)
	return planned, nil
}

func previewBasePlanDigest(p backend.PreparedPreview) string {
	data, _ := json.Marshal(struct {
		Identity            backend.PreviewIdentity
		Context, Dockerfile string
		Bases               []backend.PreviewBaseImage
	}{p.Spec.Identity, p.ContextDigest, p.Spec.Dockerfile, p.BasePlan})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Docker's SDK normalizes the repository name and prefers digest over tag.
// The pinned classic daemon formats the terminal status with FamiliarString,
// so docker.io/library/node@sha256:... completes as node@sha256:..., not a tag.
func consumePreviewPull(ctx context.Context, body io.Reader, canonical string) (string, error) {
	ref, err := reference.ParseDockerRef(canonical)
	if err != nil || ref.String() != canonical {
		return "", backend.ErrUnsupported
	}
	expected := reference.FamiliarString(ref)
	reader := &io.LimitedReader{R: body, N: runtimeImageStreamLimit + 1}
	decoder := json.NewDecoder(reader)
	terminal, failure := false, false
	manifestDigest := ""
	for {
		var message runtimeStreamMessage
		err := decoder.Decode(&message)
		if err == io.EOF {
			break
		}
		if err != nil || ctx.Err() != nil {
			return "", backend.ErrUnavailable
		}
		if runtimeMessageError(message) != "" {
			failure = true
		}
		if strings.HasPrefix(message.Status, "Digest: ") {
			value := strings.TrimPrefix(message.Status, "Digest: ")
			if terminal || !previewImageID(value) || manifestDigest != "" && manifestDigest != value {
				return "", backend.ErrUnavailable
			}
			manifestDigest = value
		}
		const downloaded = "Status: Downloaded newer image for "
		const present = "Status: Image is up to date for "
		if strings.HasPrefix(message.Status, downloaded) || strings.HasPrefix(message.Status, present) {
			if message.Status != downloaded+expected && message.Status != present+expected {
				return "", backend.ErrUnavailable
			}
			terminal = true
		}
	}
	if ctx.Err() != nil || reader.N == 0 || failure || !terminal || manifestDigest == "" {
		return "", backend.ErrUnavailable
	}
	if requested, ok := ref.(reference.Canonical); ok && requested.Digest().String() != manifestDigest {
		return "", backend.ErrUnavailable
	}
	return ref.Name() + "@" + manifestDigest, nil
}

// No current mutation is left unacknowledged when settled is true. This permits
// the coordinator to release an allocation whose guard refused before Build;
// later image presence never settles an interrupted pull.
func (b *Backend) provisionPreviewBases(ctx context.Context, p backend.PreparedPreview, guard func(context.Context) error) (previewContextPlan, bool, error) {
	images, ok := b.api.(imageResourceClient)
	if !ok {
		return previewContextPlan{}, true, backend.ErrUnsupported
	}
	resolved := make(map[string]backend.PreviewBaseImage, len(p.BasePlan))
	for _, base := range p.BasePlan {
		if base.ImageID != "" {
			resolved[base.Reference] = base
			continue
		}
		puller, ok := b.api.(previewPullClient)
		if !ok {
			return previewContextPlan{}, true, backend.ErrUnsupported
		}
		if err := b.previewIdentity(ctx, p.Spec.Identity); err != nil {
			return previewContextPlan{}, true, err
		}
		if _, digest, err := previewContext(p.Spec); err != nil || digest != p.ContextDigest {
			return previewContextPlan{}, true, backend.ErrLaunchFenceOwnership
		}
		if err := guard(ctx); err != nil {
			return previewContextPlan{}, true, err
		}
		if err := ctx.Err(); err != nil {
			return previewContextPlan{}, true, err
		}
		body, err := puller.ImagePull(ctx, base.Reference, image.PullOptions{})
		if err != nil || body == nil {
			if body != nil {
				_ = body.Close()
			}
			return previewContextPlan{}, false, backend.ErrUnavailable
		}
		digestRef, pullErr := consumePreviewPull(ctx, body, base.Reference)
		closeErr := body.Close()
		if pullErr != nil || closeErr != nil {
			return previewContextPlan{}, false, backend.ErrUnavailable
		}
		// The pull's complete terminal reply is known before checking cancellation
		// or image policy. These read-only failures do not imply an active pull.
		if err := ctx.Err(); err != nil {
			return previewContextPlan{}, true, err
		}
		row, err := images.ImageInspect(ctx, digestRef)
		if err != nil {
			return previewContextPlan{}, true, backend.ErrUnavailable
		}
		matchesDigest := false
		for _, candidate := range row.RepoDigests {
			canonical, err := previewBaseReference(candidate)
			if err == nil && canonical == digestRef {
				matchesDigest = true
			}
		}
		if !matchesDigest || !runtimeBaseValid(row) || uint64(row.Size) > previewColdBaseVirtualLimit {
			return previewContextPlan{}, true, backend.ErrUnsupported
		}
		base.ImageID, base.VirtualSizeBytes = row.ID, uint64(row.Size)
		resolved[base.Reference] = base
	}
	plan, err := b.planPreviewContext(ctx, p.Spec, resolved)
	if err != nil {
		return previewContextPlan{}, true, err
	}
	if plan.digest != p.ContextDigest || plan.demand > p.RequiredDiskBytes {
		return previewContextPlan{}, true, backend.ErrLaunchFenceOwnership
	}
	return plan, true, nil
}
