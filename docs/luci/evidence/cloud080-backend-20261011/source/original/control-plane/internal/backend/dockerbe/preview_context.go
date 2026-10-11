package dockerbe

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const maxPreviewContext = 64 << 20
const maxPreviewContextFiles = 10000

type previewFile struct {
	name      string
	mode      int64
	data      []byte
	directory bool
}

func previewContext(spec backend.PreviewSpec) ([]previewFile, string, error) {
	if (spec.SourceDirectory == "") == (len(spec.SourceArchive) == 0) {
		return nil, "", backend.ErrUnsupported
	}
	files := []previewFile{}
	total := 0
	add := func(f previewFile) error {
		total += len(f.data)
		if total > maxPreviewContext || len(files) >= maxPreviewContextFiles {
			return backend.ErrUnsupported
		}
		files = append(files, f)
		return nil
	}
	if spec.SourceDirectory != "" {
		if !checkSourceDirectory(spec.SourceDirectory) {
			return nil, "", backend.ErrUnsupported
		}
		err := filepath.WalkDir(spec.SourceDirectory, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p == spec.SourceDirectory {
				return nil
			}
			rel, err := filepath.Rel(spec.SourceDirectory, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if !previewContextPath(rel) {
				return backend.ErrUnsupported
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() && !info.Mode().IsRegular() {
				return backend.ErrUnsupported
			}
			f := previewFile{name: rel, mode: int64(info.Mode().Perm()), directory: info.IsDir()}
			if !f.directory {
				if info.Size() > int64(maxPreviewContext-total) {
					return backend.ErrUnsupported
				}
				f.data, err = os.ReadFile(p)
				if err != nil {
					return err
				}
			}
			return add(f)
		})
		if err != nil {
			return nil, "", err
		}
	} else {
		if len(spec.SourceArchive) > maxPreviewContext {
			return nil, "", backend.ErrUnsupported
		}
		r := tar.NewReader(bytes.NewReader(spec.SourceArchive))
		seen := map[string]bool{}
		for {
			h, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, "", err
			}
			name := strings.TrimSuffix(h.Name, "/")
			if !previewContextPath(name) || seen[name] || h.Size < 0 || h.Size > int64(maxPreviewContext-total) {
				return nil, "", backend.ErrUnsupported
			}
			seen[name] = true
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA && h.Typeflag != tar.TypeDir {
				return nil, "", backend.ErrUnsupported
			}
			f := previewFile{name: name, mode: h.Mode & 0777, directory: h.Typeflag == tar.TypeDir}
			if !f.directory {
				f.data, err = io.ReadAll(io.LimitReader(r, h.Size+1))
				if err != nil || int64(len(f.data)) != h.Size {
					return nil, "", backend.ErrUnsupported
				}
			}
			if err := add(f); err != nil {
				return nil, "", err
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	raw, err := previewTar(files)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if spec.ContextDigest != "" && spec.ContextDigest != digest {
		return nil, "", backend.ErrLaunchFenceOwnership
	}
	return files, digest, nil
}
func previewContextPath(p string) bool {
	return p != "" && p != "." && path.Clean(p) == p && !strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "../") && !strings.ContainsAny(p, "\\\x00:\r\n") && p != ".git" && !strings.HasPrefix(p, ".git/")
}
func previewTar(files []previewFile) ([]byte, error) {
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for _, f := range files {
		typ := byte(tar.TypeReg)
		if f.directory {
			typ = tar.TypeDir
		}
		h := &tar.Header{Name: f.name, Mode: f.mode, Size: int64(len(f.data)), Typeflag: typ}
		if err := w.WriteHeader(h); err != nil {
			return nil, err
		}
		if _, err := w.Write(f.data); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// The bounded classic-builder path accepts ordinary Dockerfiles. Dynamic FROM,
// heredocs and alternate syntax directives need a separately budgeted builder;
// they are refused before mutation, never silently reinterpreted. Stage aliases
// and case-insensitive FROM/AS are preserved. Every external base is pinned by
// immutable local image identity; preparation never pulls an uncharged image.
func (b *Backend) preparePreviewContext(ctx context.Context, spec backend.PreviewSpec) ([]byte, string, []string, uint64, error) {
	files, digest, err := previewContext(spec)
	if err != nil {
		return nil, "", nil, 0, err
	}
	dockerfile := spec.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	if !previewContextPath(dockerfile) {
		return nil, "", nil, 0, backend.ErrUnsupported
	}
	index := -1
	for i := range files {
		if files[i].name == dockerfile && !files[i].directory {
			index = i
		}
	}
	if index < 0 || len(files[index].data) > 1<<20 {
		return nil, "", nil, 0, backend.ErrUnsupported
	}
	images, ok := b.api.(imageResourceClient)
	if !ok {
		return nil, "", nil, 0, backend.ErrUnsupported
	}
	lines := strings.Split(strings.ReplaceAll(string(files[index].data), "\r\n", "\n"), "\n")
	stages := map[string]bool{}
	bases := []string{}
	var baseBytes uint64
	out := []string{}
	fromCount := 0
	continued := false
	labels := previewIdentityLabels(spec.Identity)
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	labelLine := "LABEL"
	for _, k := range keys {
		labelLine += fmt.Sprintf(" %s=%q", k, labels[k])
	}
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		lower := strings.ToLower(clean)
		if strings.Contains(line, "\x00") || strings.HasPrefix(lower, "# syntax=") || strings.HasPrefix(lower, "# escape=") || strings.Contains(line, "<<") || strings.Contains(lower, "luci.") {
			return nil, "", nil, 0, backend.ErrUnsupported
		}
		if clean == "" || strings.HasPrefix(clean, "#") {
			out = append(out, line)
			continue
		}
		fields := strings.Fields(clean)
		isFrom := !continued && strings.EqualFold(fields[0], "FROM")
		if isFrom {
			if len(fields) != 2 && (len(fields) != 4 || !strings.EqualFold(fields[2], "AS")) {
				return nil, "", nil, 0, backend.ErrUnsupported
			}
			base := fields[1]
			if strings.ContainsAny(base, "$\\") || strings.HasPrefix(base, "--") {
				return nil, "", nil, 0, backend.ErrUnsupported
			}
			fromCount++
			if fromCount > 16 {
				return nil, "", nil, 0, backend.ErrUnsupported
			}
			if !stages[strings.ToLower(base)] && base != "scratch" {
				row, e := images.ImageInspect(ctx, base)
				if e != nil {
					return nil, "", nil, 0, fmt.Errorf("preview base image %q must be available locally before its build can be admitted: %w", base, backend.ErrUnavailable)
				}
				if !previewImageID(row.ID) || row.Size <= 0 || row.Os != "linux" || row.Config == nil || len(row.Config.OnBuild) != 0 || len(row.Config.Volumes) != 0 {
					return nil, "", nil, 0, backend.ErrUnsupported
				}
				fields[1] = row.ID
				bases = append(bases, row.ID)
				baseBytes += uint64(row.Size)
			}
			if len(fields) == 4 {
				alias := strings.ToLower(fields[3])
				if alias == "" || stages[alias] {
					return nil, "", nil, 0, backend.ErrUnsupported
				}
				stages[alias] = true
			}
			out = append(out, strings.Join(fields, " "), labelLine)
		} else {
			out = append(out, line)
		}
		continued = strings.HasSuffix(clean, "\\")
	}
	if fromCount == 0 || continued {
		return nil, "", nil, 0, backend.ErrUnsupported
	}
	files[index].data = []byte(strings.Join(out, "\n"))
	prepared, err := previewTar(files)
	if err != nil {
		return nil, "", nil, 0, err
	}
	// Includes context, base materialization per stage and a working reserve. It
	// is a conservative admission estimate, not a per-build filesystem quota.
	demand := baseBytes*2 + uint64(len(prepared))*2 + (1 << 30)
	return prepared, digest, bases, demand, nil
}
