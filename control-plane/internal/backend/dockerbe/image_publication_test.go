package dockerbe

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type publicationProbe struct {
	*previewProbe
	tags, pushes int
	stream       string
	options      image.PushOptions
	remoteExists bool
	closeError   error
	remoteDigest string
}

func (p *publicationProbe) ImageTag(_ context.Context, source, target string) error {
	p.tags++
	p.images[target] = p.images[source]
	return nil
}
func (p *publicationProbe) ImagePush(_ context.Context, _ string, options image.PushOptions) (io.ReadCloser, error) {
	p.pushes++
	p.options = options
	return publicationBody{Reader: strings.NewReader(p.stream), closeError: p.closeError}, nil
}
func (p *publicationProbe) DistributionInspect(_ context.Context, ref, auth string) (registry.DistributionInspect, error) {
	if strings.Contains(ref, "@") || p.remoteExists {
		return registry.DistributionInspect{Descriptor: ocispec.Descriptor{Digest: digest.Digest(p.remoteDigest)}}, nil
	}
	return registry.DistributionInspect{}, checkMissing{}
}

type publicationBody struct {
	io.Reader
	closeError error
}

func (b publicationBody) Close() error { return b.closeError }
func publicationFixture(t *testing.T) (*Backend, *publicationProbe, backend.ImagePublication) {
	t.Helper()
	b, base, s := newPreviewProbe(t)
	prepared, err := b.PreparePreview(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	imageID := previewBuiltImage(base, prepared)
	p := &publicationProbe{previewProbe: base, remoteDigest: "sha256:" + strings.Repeat("e", 64)}
	b.api = p
	request := backend.ImagePublication{Identity: prepared.Spec.Identity, ImageID: imageID, Repository: "registry.example/team/app", Tag: "luci-" + prepared.Spec.Identity.AllocationID, RegistryAuth: "explicit-private-auth"}
	p.stream = `{"aux":{"Tag":"` + request.Tag + `","Digest":"` + p.remoteDigest + `","Size":123}}` + "\n"
	return b, p, request
}
func TestImagePublicationRequiresOriginalEOFAndDigestWithoutRetry(t *testing.T) {
	cases := []struct {
		name, stream     string
		settled, success bool
	}{
		{"normal-original-digest", "valid", true, true},
		{"missing-original-digest", "{}\n", false, false},
		{"partial-original-reply", `{"aux":`, false, false},
		{"duplicate-original-aux", "duplicate", false, false},
		{"stream-close-error", "close-error", false, false},
		{"definitive-daemon-error", `{"error":"private diagnostics withheld"}` + "\n", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, p, request := publicationFixture(t)
			if tc.stream == "close-error" {
				p.closeError = errors.New("controlled close failure")
			} else if tc.stream == "duplicate" {
				p.stream += p.stream
			} else if tc.stream != "valid" {
				p.stream = tc.stream
			}
			calls := 0
			out, err := b.PublishPreviewImage(context.Background(), request, func(context.Context) error { calls++; return nil })
			if out.Settled != tc.settled || (err == nil) != tc.success || p.tags != 1 || p.pushes != 1 || calls != 2 {
				t.Fatal(out, err, p.tags, p.pushes, calls)
			}
			if tc.success && out.Reference != request.Repository+"@"+p.remoteDigest {
				t.Fatal("digest was not correlated to original publisher", out)
			}
			if !tc.success && out.Reference != "" {
				t.Fatal("unconfirmed image reference escaped")
			}
			if p.options.RegistryAuth != request.RegistryAuth || p.options.PrivilegeFunc != nil || p.options.All {
				t.Fatal("publisher acquired ambient credentials or enabled implicit retry")
			}
		})
	}
}
func TestImagePublicationRefusesExistingTagsAndLateAuthorityBeforePush(t *testing.T) {
	t.Run("existing-remote-not-adopted", func(t *testing.T) {
		b, p, req := publicationFixture(t)
		p.remoteExists = true
		out, err := b.PublishPreviewImage(context.Background(), req, allowPreviewMutation)
		if err == nil || out.Settled || p.tags != 0 || p.pushes != 0 {
			t.Fatal(out, err, p.tags, p.pushes)
		}
	})
	t.Run("late-grant-after-tag", func(t *testing.T) {
		b, p, req := publicationFixture(t)
		calls := 0
		out, err := b.PublishPreviewImage(context.Background(), req, func(context.Context) error {
			calls++
			if calls == 2 {
				return errors.New("grant revoked")
			}
			return nil
		})
		if err == nil || out.Settled || p.tags != 1 || p.pushes != 0 {
			t.Fatal(out, err, p.tags, p.pushes)
		}
	})
	t.Run("foreign-local-image", func(t *testing.T) {
		b, p, req := publicationFixture(t)
		row := p.images[req.ImageID]
		row.Config.Labels[previewPrefix+"commit"] = strings.Repeat("f", 40)
		p.images[req.ImageID] = row
		if _, err := b.PublishPreviewImage(context.Background(), req, allowPreviewMutation); err == nil || p.tags != 0 || p.pushes != 0 {
			t.Fatal("foreign source image was published", err)
		}
	})
}
