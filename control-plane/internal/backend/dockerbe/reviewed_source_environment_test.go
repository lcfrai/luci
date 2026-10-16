package dockerbe

import (
	"context"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestReviewedSourceEnvironmentBindsInheritedVersionsAndUniqueExactEntries(t *testing.T) {
	b, f, s := reviewedSourceFixture(t)
	p, err := b.PreparePreview(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := b.CreatePreview(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	row := f.rows[ref]
	versions := map[string]bool{}
	for _, entry := range row.Config.Env {
		if strings.HasPrefix(entry, "NODE_VERSION=") || strings.HasPrefix(entry, "YARN_VERSION=") {
			versions[entry] = true
		}
	}
	if !versions["NODE_VERSION=24.0.0"] || !versions["YARN_VERSION=1.22.22"] || len(row.Config.Env) != 7 || p.Spec.Identity.EnvironmentDigest == "" {
		t.Fatal("complete image environment was not retained and bound")
	}
	reordered := copyCheckRow(row)
	for left, right := 0, len(reordered.Config.Env)-1; left < right; left, right = left+1, right-1 {
		reordered.Config.Env[left], reordered.Config.Env[right] = reordered.Config.Env[right], reordered.Config.Env[left]
	}
	if !previewOwned(p.Spec.Identity, ref, reordered, true) {
		t.Fatal("equivalent unique environment ordering was refused")
	}
	for name, change := range map[string]func(*container.InspectResponse){
		"bad-before-empty": func(r *container.InspectResponse) {
			r.Config.Env = append([]string{"NODE_OPTIONS=--require=synthetic-inert"}, r.Config.Env...)
		},
		"bad-after-empty": func(r *container.InspectResponse) {
			r.Config.Env = append(r.Config.Env, "NODE_OPTIONS=--require=synthetic-inert")
		},
		"duplicate-identical": func(r *container.InspectResponse) { r.Config.Env = append([]string{"NODE_OPTIONS="}, r.Config.Env...) },
		"unknown-extra":       func(r *container.InspectResponse) { r.Config.Env = append(r.Config.Env, "SYNTHETIC_EXTRA=value") },
		"loader-extra": func(r *container.InspectResponse) {
			r.Config.Env = append(r.Config.Env, "LD_PRELOAD=/synthetic/inert-only")
		},
		"missing-node-version": func(r *container.InspectResponse) {
			var env []string
			for _, entry := range r.Config.Env {
				if !strings.HasPrefix(entry, "NODE_VERSION=") {
					env = append(env, entry)
				}
			}
			r.Config.Env = env
		},
		"changed-node-version": func(r *container.InspectResponse) {
			for i, entry := range r.Config.Env {
				if strings.HasPrefix(entry, "NODE_VERSION=") {
					r.Config.Env[i] = "NODE_VERSION=24.99.99"
				}
			}
		},
		"changed-yarn-version": func(r *container.InspectResponse) {
			for i, entry := range r.Config.Env {
				if strings.HasPrefix(entry, "YARN_VERSION=") {
					r.Config.Env[i] = "YARN_VERSION=9.9.9"
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := copyCheckRow(row)
			change(&changed)
			if previewOwned(p.Spec.Identity, ref, changed, true) {
				t.Fatal("strict identity accepted changed/duplicate declared environment")
			}
			if !previewOwned(p.Spec.Identity, ref, changed, false) {
				t.Fatal("environment drift hid the exact owned CID from retirement")
			}
		})
	}
}

func TestReviewedSourceEnvironmentRejectsChangedPreparedBindingBeforeCreate(t *testing.T) {
	b, f, s := reviewedSourceFixture(t)
	p, err := b.PreparePreview(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	s = p.Spec
	s.Identity.EnvironmentDigest = strings.Repeat("f", 64)
	if _, err := b.PreparePreview(context.Background(), s); err == nil {
		t.Fatal("changed complete environment digest was accepted")
	}
	p.SourceEnvironment = append(p.SourceEnvironment, "SYNTHETIC_EXTRA=value")
	if _, err := b.CreatePreview(context.Background(), p); err == nil || len(f.creates) != 0 {
		t.Fatal("changed prepared environment reached ContainerCreate")
	}
}
