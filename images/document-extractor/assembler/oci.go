package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type assemblyBoundedWriter struct {
	ctx        context.Context
	writer     io.Writer
	bytes, max int64
}

func (w *assemblyBoundedWriter) Write(p []byte) (int, error) {
	if w.ctx.Err() != nil || int64(len(p)) > w.max-w.bytes {
		return 0, errRefused
	}
	n, err := w.writer.Write(p)
	w.bytes += int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

func blobDescriptor(media string, data []byte) ociDescriptor {
	sum := sha256.Sum256(data)
	return ociDescriptor{media, "sha256:" + hex.EncodeToString(sum[:]), int64(len(data))}
}
func rootfsNames(ctx context.Context, root string) ([]string, error) {
	names := []string{}
	retained := 0
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || ctx.Err() != nil {
			return errRefused
		}
		if name == root {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return errRefused
		}
		rel = filepath.ToSlash(rel)
		retained += len(rel)
		if !relativeUnixName(rel) || len(names) >= 100000 || retained > 16<<20 {
			return errRefused
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() && !info.IsDir() && info.Mode()&os.ModeSymlink == 0 || info.Size() > 128<<20 {
			return errRefused
		}
		names = append(names, rel)
		return nil
	})
	sort.Strings(names)
	return names, err
}
func stageLayer(ctx context.Context, root string) (ociDescriptor, string, error) {
	zero := ociDescriptor{}
	names, err := rootfsNames(ctx, root)
	if err != nil {
		return zero, "", err
	}
	file, err := os.OpenFile("/work/layer.tar.gz", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0400)
	if err != nil {
		return zero, "", errRefused
	}
	compressedHash := sha256.New()
	output := &assemblyBoundedWriter{ctx: ctx, writer: io.MultiWriter(file, compressedHash), max: DocumentImageAssemblyOutputMaxBytes}
	compressed := gzip.NewWriter(output)
	compressed.Header.ModTime = time.Unix(0, 0)
	compressed.Header.OS = 255
	diff := sha256.New()
	expanded := &assemblyBoundedWriter{ctx: ctx, writer: io.MultiWriter(compressed, diff), max: 2 << 30}
	archive := tar.NewWriter(expanded)
	fail := func() (ociDescriptor, string, error) {
		_ = archive.Close()
		_ = compressed.Close()
		_ = file.Close()
		return zero, "", errRefused
	}
	for _, name := range names {
		if ctx.Err() != nil {
			return fail()
		}
		full := filepath.Join(root, filepath.FromSlash(name))
		info, err := os.Lstat(full)
		if err != nil {
			return fail()
		}
		target := ""
		if info.Mode()&os.ModeSymlink != 0 {
			target, err = os.Readlink(full)
			if err != nil {
				return fail()
			}
		}
		header, err := tar.FileInfoHeader(info, target)
		if err != nil {
			return fail()
		}
		header.Name = name
		header.ModTime = time.Unix(0, 0)
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
		header.Uname = ""
		header.Gname = ""
		header.Format = tar.FormatPAX
		if archive.WriteHeader(header) != nil {
			return fail()
		}
		if info.Mode().IsRegular() {
			input, err := os.Open(full)
			if err != nil {
				return fail()
			}
			n, copyErr := io.Copy(archive, &contextReader{ctx: ctx, reader: input})
			closeErr := input.Close()
			if copyErr != nil || closeErr != nil || n != info.Size() {
				return fail()
			}
		}
	}
	if archive.Close() != nil || compressed.Close() != nil || file.Close() != nil || output.bytes < 1 {
		return zero, "", errRefused
	}
	return ociDescriptor{"application/vnd.oci.image.layer.v1.tar+gzip", "sha256:" + hex.EncodeToString(compressedHash.Sum(nil)), output.bytes}, "sha256:" + hex.EncodeToString(diff.Sum(nil)), nil
}
func emitOCI(ctx context.Context, root string, manifest DocumentImageAssemblyManifest, inputDigest string, output io.Writer) error {
	layer, diff, err := stageLayer(ctx, root)
	if err != nil {
		return err
	}
	config := struct {
		Architecture string `json:"architecture"`
		OS           string `json:"os"`
		Config       struct {
			User       string            `json:"User"`
			Env        []string          `json:"Env"`
			Entrypoint []string          `json:"Entrypoint"`
			WorkingDir string            `json:"WorkingDir"`
			Labels     map[string]string `json:"Labels"`
		} `json:"config"`
		RootFS struct {
			Type    string   `json:"type"`
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
	}{Architecture: "amd64", OS: "linux"}
	config.Config.User = "1001:1001"
	config.Config.WorkingDir = "/tmp"
	config.Config.Entrypoint = []string{"/usr/bin/python3", "-I", "/opt/luci/document_extractor.py"}
	config.Config.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/tmp", "TMPDIR=/tmp", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "OMP_THREAD_LIMIT=1", "OMP_NUM_THREADS=1", "TESSDATA_PREFIX=/opt/luci/tessdata", "PYTHONDONTWRITEBYTECODE=1"}
	config.Config.Labels = map[string]string{"org.opencontainers.image.revision": manifest.SourceCommit, "luci.document-source": manifest.SourceDigest, "luci.document-input-manifest": inputDigest, "luci.document-helper": manifest.HelperDigest, "luci.document-recipe": manifest.RecipeDigest, "luci.document-source-retention": manifest.SourceRetentionDigest}
	config.RootFS.Type = "layers"
	config.RootFS.DiffIDs = []string{diff}
	configData, err := json.Marshal(config)
	if err != nil {
		return errRefused
	}
	configDescriptor := blobDescriptor("application/vnd.oci.image.config.v1+json", configData)
	manifestData, err := json.Marshal(struct {
		SchemaVersion int             `json:"schemaVersion"`
		MediaType     string          `json:"mediaType"`
		Config        ociDescriptor   `json:"config"`
		Layers        []ociDescriptor `json:"layers"`
	}{2, "application/vnd.oci.image.manifest.v1+json", configDescriptor, []ociDescriptor{layer}})
	if err != nil {
		return errRefused
	}
	manifestDescriptor := blobDescriptor("application/vnd.oci.image.manifest.v1+json", manifestData)
	index, err := json.Marshal(struct {
		SchemaVersion int             `json:"schemaVersion"`
		Manifests     []ociDescriptor `json:"manifests"`
	}{2, []ociDescriptor{manifestDescriptor}})
	if err != nil {
		return errRefused
	}
	metadata := map[string][]byte{"oci-layout": []byte(`{"imageLayoutVersion":"1.0.0"}`), "index.json": index, "blobs/sha256/" + strings.TrimPrefix(configDescriptor.Digest, "sha256:"): configData, "blobs/sha256/" + strings.TrimPrefix(manifestDescriptor.Digest, "sha256:"): manifestData}
	// First pass computes the exact tar byte length/hash without staging a second
	// huge archive. Only helper-owned immutable layer/metadata feed both passes.
	h := sha256.New()
	measure := &assemblyBoundedWriter{ctx: ctx, writer: h, max: DocumentImageAssemblyOutputMaxBytes}
	if writeOCITar(ctx, measure, metadata, layer, "/work/layer.tar.gz") != nil || measure.bytes < 1 {
		return errRefused
	}
	var prefix [8]byte
	binary.BigEndian.PutUint64(prefix[:], uint64(measure.bytes))
	if writeAll(output, []byte(resultMagic)) != nil || writeAll(output, prefix[:]) != nil || writeAll(output, h.Sum(nil)) != nil {
		return errRefused
	}
	check := sha256.New()
	emit := &assemblyBoundedWriter{ctx: ctx, writer: io.MultiWriter(output, check), max: measure.bytes}
	if writeOCITar(ctx, emit, metadata, layer, "/work/layer.tar.gz") != nil || emit.bytes != measure.bytes || !sameHash(h, check) {
		return errRefused
	}
	return writeAll(output, []byte(resultEnd))
}
func sameHash(a, b hash.Hash) bool {
	return hex.EncodeToString(a.Sum(nil)) == hex.EncodeToString(b.Sum(nil))
}
func writeAll(w io.Writer, p []byte) error {
	n, err := w.Write(p)
	if err != nil || n != len(p) {
		return errRefused
	}
	return nil
}
func writeOCITar(ctx context.Context, sink io.Writer, metadata map[string][]byte, layer ociDescriptor, layerPath string) error {
	archive := tar.NewWriter(sink)
	names := []string{}
	for name := range metadata {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := metadata[name]
		if len(data) > 1<<20 || archive.WriteHeader(&tar.Header{Name: name, Size: int64(len(data)), Mode: 0444, ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR}) != nil || writeAll(archive, data) != nil {
			return errRefused
		}
	}
	if archive.WriteHeader(&tar.Header{Name: "blobs/sha256/" + strings.TrimPrefix(layer.Digest, "sha256:"), Size: layer.Size, Mode: 0444, ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR}) != nil {
		return errRefused
	}
	file, err := os.Open(layerPath)
	if err != nil {
		return errRefused
	}
	defer file.Close()
	h := sha256.New()
	n, err := io.Copy(archive, io.TeeReader(&contextReader{ctx: ctx, reader: io.LimitReader(file, layer.Size+1)}, h))
	if err != nil || n != layer.Size || "sha256:"+hex.EncodeToString(h.Sum(nil)) != layer.Digest || archive.Close() != nil {
		return errRefused
	}
	return nil
}
