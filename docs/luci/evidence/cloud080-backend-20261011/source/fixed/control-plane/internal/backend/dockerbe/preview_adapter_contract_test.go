package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/network"
)

type previewNetworkProbe struct {
	*previewProbe
	observation network.Inspect
	lookups     []string
}

func (f *previewNetworkProbe) NetworkInspect(_ context.Context, id string, _ network.InspectOptions) (network.Inspect, error) {
	f.lookups = append(f.lookups, id)
	if id != f.observation.ID {
		return network.Inspect{}, errors.New("network unavailable")
	}
	return f.observation, nil
}

func TestPreviewAcceptsProductExtraLabelContract(t *testing.T) {
	b, f, s := newPreviewProbe(t)
	s.Network = "bridge"
	s.Labels = map[string]string{"luci.release-id": s.Identity.ReleaseID, "luci.project-id": s.Identity.ProjectID}
	if _, err := b.PreparePreview(context.Background(), s); err != nil {
		t.Fatal("actual product extra label shape refused", err)
	}
	if len(f.buildOptions)+len(f.creates)+len(f.starts) != 0 {
		t.Fatal("label validation mutated resources")
	}
	s.Labels["luci.preview"] = "true"
	if _, err := b.PreparePreview(context.Background(), s); err == nil {
		t.Fatal("caller could supply mandatory runtime ownership labels")
	}
}

func TestPreviewNetworkUsesTrustedDerivedPlatformInstance(t *testing.T) {
	for _, mode := range []string{"owned-derived-instance", "wrong-platform-instance", "wrong-project", "missing-trusted-instance", "wrong-trusted-project", "mutable-network-name", "wrong-driver"} {
		t.Run(mode, func(t *testing.T) {
			b, original, s := newPreviewProbe(t)
			// Same derivation as platform.New: state directory + instance ID.
			sum := sha256.Sum256([]byte("/private/platform-state" + b.instance))
			derived := hex.EncodeToString(sum[:8])
			if derived == b.instance {
				t.Fatal("fixture did not distinguish instance namespaces")
			}
			s.Network = strings.Repeat("f", 64)
			s.Labels = map[string]string{"ai.lcfr.luci.platform.instance": derived, "ai.lcfr.luci.platform.project": s.Identity.ProjectID, "ai.lcfr.luci.platform.resource": s.Identity.ReleaseID}
			f := &previewNetworkProbe{previewProbe: original, observation: network.Inspect{ID: s.Network, Driver: "bridge", Labels: map[string]string{"ai.lcfr.luci.platform.instance": derived, "ai.lcfr.luci.platform.project": s.Identity.ProjectID}}}
			b.api = f
			switch mode {
			case "wrong-platform-instance":
				f.observation.Labels["ai.lcfr.luci.platform.instance"] = "0000000000000000"
			case "wrong-project":
				f.observation.Labels["ai.lcfr.luci.platform.project"] = "other-project"
			case "missing-trusted-instance":
				delete(s.Labels, "ai.lcfr.luci.platform.instance")
			case "wrong-trusted-project":
				s.Labels["ai.lcfr.luci.platform.project"] = "other-project"
			case "mutable-network-name":
				s.Network = "project-network"
				f.observation.ID = s.Network
			case "wrong-driver":
				f.observation.Driver = "host"
			}
			_, err := b.PreparePreview(context.Background(), s)
			if (err == nil) != (mode == "owned-derived-instance") {
				t.Fatal("network ownership decision did not bind trusted adapter namespace", mode, err)
			}
			if len(f.buildOptions)+len(f.creates)+len(f.starts) != 0 {
				t.Fatal("network preparation mutated resources")
			}
		})
	}
}
