package dockerbe

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func reviewedSourceFixture(t *testing.T) (*Backend, *previewProbe, backend.PreviewSpec) {
	t.Helper()
	b, f, s := newPreviewProbe(t)
	f.image.Config.Env = []string{"NODE_VERSION=24.0.0", "PATH=/usr/local/bin:/usr/bin:/bin", "YARN_VERSION=1.22.22"}
	f.images[b.resourceImage] = f.image
	s.Kind, s.Identity.Kind, s.Dockerfile = backend.ReviewedSourcePreview, backend.ReviewedSourcePreview, ""
	for name, data := range map[string]string{"index.html": "<script type=module src=assets/app.mjs></script>", "assets/app.mjs": "export const fixture = true;", "assets/theme.css": "body { color: green }", "notes.md": "not a browser asset", ".private/hidden.json": "synthetic-private"} {
		file := filepath.Join(s.SourceDirectory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return b, f, s
}

func TestReviewedSourceFreezesWholeSnapshotAndServesOnlyStaticBytes(t *testing.T) {
	b, f, s := reviewedSourceFixture(t)
	p, err := b.PreparePreview(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spec.ImageID != f.image.ID || p.Spec.Identity.RuntimeImageID != f.image.ID || p.Spec.Identity.SnapshotDigest != p.ContextDigest || p.ContextDigest != sourceDigest(p.PreparedContext) || !fenceHex(p.Spec.Identity.CommandDigest, 32) || p.Spec.SourceDirectory != "" || !bytes.Equal(p.Spec.SourceArchive, p.PreparedContext) || len(f.creates)+len(f.buildOptions) != 0 {
		t.Fatal("incomplete or mutating static preparation")
	}
	var assets []sourceAsset
	if err := json.Unmarshal(p.SourcePayload, &assets); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, asset := range assets {
		seen[asset.Name] = true
	}
	if !seen["index.html"] || !seen["assets/app.mjs"] || !seen["assets/theme.css"] || seen["notes.md"] || seen[".private/hidden.json"] || seen["Dockerfile"] {
		t.Fatal("static projection omitted assets or exposed nonassets/private metadata")
	}
	changed := s
	if err := os.WriteFile(filepath.Join(s.SourceDirectory, "notes.md"), []byte("changed nonserved source"), 0600); err != nil {
		t.Fatal(err)
	}
	next, err := b.PreparePreview(context.Background(), changed)
	if err != nil || next.ContextDigest == p.ContextDigest {
		t.Fatal("full context digest ignored nonserved bytes", err)
	}
	// The product removes its temporary archive when Start returns. The frozen
	// payload remains complete and does not mount or reread that mutable path.
	if err := os.RemoveAll(s.SourceDirectory); err != nil {
		t.Fatal(err)
	}
	ref, err := b.CreatePreview(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	r := f.rows[ref]
	if len(f.buildOptions) != 0 || len(r.Mounts) != 0 || !r.HostConfig.ReadonlyRootfs || r.Config.User != "1001:1001" || !previewOwned(p.Spec.Identity, ref, r, true) || r.Config.Labels[previewPrefix+"image"] != f.image.ID {
		t.Fatal("source preview did not use strict ordinary serving configuration")
	}
	if err := b.StartPreview(context.Background(), p.Spec.Identity, ref); err != nil {
		t.Fatal(err)
	}
	if err := b.RemovePreview(context.Background(), p.Spec.Identity, ref); err != nil {
		t.Fatal(err)
	}
	observed, err := b.InspectPreview(context.Background(), p.Spec.Identity, ref)
	if err != nil || observed.State != "absent" || len(f.removes) != 1 || len(f.starts) != 1 {
		t.Fatal("exact static retirement was not positive", observed, err)
	}
}

func TestReviewedSourceRejectsUnsafeOrOversizedSnapshotBeforeCreate(t *testing.T) {
	for _, name := range []string{"environment", "dockerfile", "missing-index", "unsafe-asset-path", "private-asset", "asset-overflow", "snapshot-overflow", "tar-overhead", "too-many-entries", "link", "traversal", "duplicate", "runtime-absent", "runtime-replaced", "runtime-capability", "runtime-private-env", "command-replaced", "snapshot-replaced"} {
		t.Run(name, func(t *testing.T) {
			b, f, s := reviewedSourceFixture(t)
			switch name {
			case "environment":
				s.Environment = map[string]string{"FIXTURE_PRIVATE": "must-not-enter-server"}
			case "dockerfile":
				s.Dockerfile = "Dockerfile"
			case "missing-index":
				if err := os.Remove(filepath.Join(s.SourceDirectory, "index.html")); err != nil {
					t.Fatal(err)
				}
			case "unsafe-asset-path":
				if err := os.WriteFile(filepath.Join(s.SourceDirectory, "percent%20.js"), []byte("x"), 0600); err != nil {
					t.Fatal(err)
				}
			case "private-asset":
				if err := os.WriteFile(filepath.Join(s.SourceDirectory, "secrets.json"), []byte("synthetic private fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			case "asset-overflow":
				if err := os.WriteFile(filepath.Join(s.SourceDirectory, "large.js"), bytes.Repeat([]byte("x"), sourceAssetLimit+1), 0600); err != nil {
					t.Fatal(err)
				}
			case "snapshot-overflow":
				if err := os.WriteFile(filepath.Join(s.SourceDirectory, "large.bin"), bytes.Repeat([]byte("x"), sourceSnapshotLimit+1), 0600); err != nil {
					t.Fatal(err)
				}
			case "tar-overhead":
				// File data fits but complete canonical tar headers/padding do not.
				if err := os.WriteFile(filepath.Join(s.SourceDirectory, "large.bin"), bytes.Repeat([]byte("x"), sourceSnapshotLimit-1024), 0600); err != nil {
					t.Fatal(err)
				}
			case "too-many-entries":
				files := []previewFile{{name: "index.html", data: []byte("ok")}}
				for i := 0; i < sourceSnapshotFiles; i++ {
					files = append(files, previewFile{name: strings.Repeat("x", i+1) + ".bin"})
				}
				var err error
				s.SourceArchive, err = previewTar(files)
				if err != nil {
					t.Fatal(err)
				}
				s.SourceDirectory = ""
			case "link", "traversal", "duplicate":
				var buf bytes.Buffer
				w := tar.NewWriter(&buf)
				header := &tar.Header{Name: "index.html", Mode: 0600, Typeflag: tar.TypeReg, Size: 2}
				if name == "link" {
					header.Typeflag = tar.TypeSymlink
					header.Linkname = "outside"
					header.Size = 0
				}
				if name == "traversal" {
					header.Name = "../index.html"
				}
				if err := w.WriteHeader(header); err != nil {
					t.Fatal(err)
				}
				if header.Size > 0 {
					if _, err := w.Write([]byte("ok")); err != nil {
						t.Fatal(err)
					}
				}
				if name == "duplicate" {
					if err := w.WriteHeader(header); err != nil {
						t.Fatal(err)
					}
					if _, err := w.Write([]byte("ok")); err != nil {
						t.Fatal(err)
					}
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				s.SourceArchive = buf.Bytes()
				s.SourceDirectory = ""
			case "runtime-absent":
				delete(f.images, b.resourceImage)
			case "runtime-replaced":
				s.ImageID = "sha256:" + strings.Repeat("e", 64)
			case "runtime-capability":
				f.images[b.resourceImage].Config.Env = []string{"NODE_VERSION=20.0.0"}
			case "runtime-private-env":
				f.images[b.resourceImage].Config.Env = append(f.images[b.resourceImage].Config.Env, "FIXTURE_PRIVATE=not-an-app-environment")
			case "command-replaced":
				s.Identity.CommandDigest = strings.Repeat("e", 64)
			case "snapshot-replaced":
				s.Identity.SnapshotDigest = strings.Repeat("e", 64)
			}
			if _, err := b.PreparePreview(context.Background(), s); err == nil {
				t.Fatal("unsupported static profile was admitted")
			}
			if len(f.creates)+len(f.starts)+len(f.buildOptions) != 0 {
				t.Fatal("refusal mutated runtime")
			}
		})
	}
}

func TestReviewedSourceStrictOwnershipBindsCommandImageAndOrdinaryLimits(t *testing.T) {
	b, f, s := reviewedSourceFixture(t)
	p, err := b.PreparePreview(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := b.CreatePreview(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*container.InspectResponse){
		"writable-root": func(r *container.InspectResponse) { r.HostConfig.ReadonlyRootfs = false },
		"server-command": func(r *container.InspectResponse) {
			r.Config.Cmd[2] = "project supplied server"
			r.Config.Labels[previewPrefix+"command"] = sourceCommandDigest([]string(r.Config.Cmd))
		},
		"payload": func(r *container.InspectResponse) {
			r.Config.Cmd[8] = "[]"
			r.Config.Cmd[5] = sourceDigest([]byte("[]"))
			r.Config.Labels[previewPrefix+"command"] = sourceCommandDigest([]string(r.Config.Cmd))
		},
		"image": func(r *container.InspectResponse) {
			r.Image = "sha256:" + strings.Repeat("e", 64)
			r.Config.Image = r.Image
			r.Config.Labels[previewPrefix+"image"] = r.Image
		},
		"user": func(r *container.InspectResponse) { r.Config.User = "0" },
		"node-preload": func(r *container.InspectResponse) {
			r.Config.Env = append(r.Config.Env, "NODE_OPTIONS=--require=private")
		},
		"health-command": func(r *container.InspectResponse) { r.Config.Healthcheck.Test = []string{"CMD", "project-script"} },
		"memory":         func(r *container.InspectResponse) { r.HostConfig.Memory = 0 },
		"network": func(r *container.InspectResponse) {
			for key, values := range r.HostConfig.PortBindings {
				values[0].HostIP = "0.0.0.0"
				r.HostConfig.PortBindings[key] = values
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			row := copyCheckRow(f.rows[ref])
			mutate(&row)
			if previewOwned(p.Spec.Identity, ref, row, true) {
				t.Fatal("changed source ownership or resource envelope was accepted")
			}
		})
	}
}
