package dockerbe

import (
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
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

const runtimeGoBase = "golang:1.26-bookworm"
const runtimeNodeBase = "node:24-bookworm-slim"

// runtimeContext snapshots only the bundled build inputs. It deliberately does
// not walk the surrounding checkout or apply a user-controlled .dockerignore.
// Reads stay within independently opened directory roots and have an actual
// byte limit, including when an ordinary file grows after its initial stat.
func runtimeContext(spec backend.RuntimeImageSpec) ([]previewFile, string, error) {
	if !filepath.IsAbs(spec.SourceDirectory) || filepath.Clean(spec.SourceDirectory) != spec.SourceDirectory || !checkSourceDirectory(spec.SourceDirectory) {
		return nil, "", backend.ErrUnsupported
	}
	before, err := os.Lstat(spec.SourceDirectory)
	if err != nil {
		return nil, "", err
	}
	root, err := os.OpenRoot(spec.SourceDirectory)
	if err != nil {
		return nil, "", err
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !runtimeSameSource(before, opened) {
		return nil, "", backend.ErrLaunchFenceOwnership
	}
	files := []previewFile{}
	total := int64(0)
	add := func(f previewFile) error {
		if !previewContextPath(f.name) || len(files) >= maxPreviewContextFiles || int64(len(f.data)) > int64(maxPreviewContext)-total {
			return backend.ErrUnsupported
		}
		total += int64(len(f.data))
		files = append(files, f)
		return nil
	}
	readFile := func(parent *os.Root, name, relative string) error {
		if len(files) >= maxPreviewContextFiles {
			return backend.ErrUnsupported
		}
		info, e := parent.Lstat(name)
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > int64(maxPreviewContext)-total {
			return backend.ErrUnsupported
		}
		absolute := filepath.Join(spec.SourceDirectory, filepath.FromSlash(relative))
		if e = hostpath.OneName(absolute); e != nil {
			return e
		}
		f, e := parent.Open(name)
		if e != nil {
			return e
		}
		defer f.Close()
		opened, e := f.Stat()
		if e != nil || !runtimeSameSource(info, opened) {
			return backend.ErrLaunchFenceOwnership
		}
		data, e := io.ReadAll(io.LimitReader(f, info.Size()+1))
		if e != nil {
			return e
		}
		after, e := f.Stat()
		if e != nil || int64(len(data)) != info.Size() || !runtimeSameSource(info, after) {
			return backend.ErrLaunchFenceOwnership
		}
		if e = hostpath.OneName(absolute); e != nil {
			return e
		}
		named, e := parent.Lstat(name)
		if e != nil || !runtimeSameSource(info, named) {
			return backend.ErrLaunchFenceOwnership
		}
		return add(previewFile{name: relative, mode: int64(info.Mode().Perm()), data: data})
	}
	var walk func(*os.Root, string, string) error
	walk = func(parent *os.Root, name, relative string) error {
		child, info, e := runtimeChildRoot(parent, name)
		if e != nil {
			return e
		}
		defer child.Close()
		if e = add(previewFile{name: relative, mode: int64(info.Mode().Perm()), directory: true}); e != nil {
			return e
		}
		directory, e := child.Open(".")
		if e != nil {
			return e
		}
		defer directory.Close()
		for {
			// Avoid ReadDir(-1): even a refused source must not first allocate
			// an unbounded directory listing.
			entries, readErr := directory.ReadDir(128)
			for _, entry := range entries {
				p := path.Join(relative, entry.Name())
				if !previewContextPath(p) || entry.Name() == ".git" || len(files) >= maxPreviewContextFiles {
					return backend.ErrUnsupported
				}
				if entry.IsDir() {
					e = walk(child, entry.Name(), p)
				} else {
					e = readFile(child, entry.Name(), p)
				}
				if e != nil {
					return e
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return readErr
			}
		}
		after, e := parent.Lstat(name)
		if e != nil || !runtimeSameSource(info, after) {
			return backend.ErrLaunchFenceOwnership
		}
		return nil
	}
	if err = walk(root, "sandboxd", "sandboxd"); err != nil {
		return nil, "", err
	}
	images, imagesInfo, err := runtimeChildRoot(root, "images")
	if err != nil {
		return nil, "", err
	}
	defer images.Close()
	sandbox, sandboxInfo, err := runtimeChildRoot(images, "sandbox")
	if err != nil {
		return nil, "", err
	}
	defer sandbox.Close()
	for _, f := range []previewFile{
		{name: "images", mode: int64(imagesInfo.Mode().Perm()), directory: true},
		{name: "images/sandbox", mode: int64(sandboxInfo.Mode().Perm()), directory: true},
	} {
		if err = add(f); err != nil {
			return nil, "", err
		}
	}
	for _, name := range []string{"Dockerfile", "entrypoint.sh"} {
		if err = readFile(sandbox, name, "images/sandbox/"+name); err != nil {
			return nil, "", err
		}
	}
	for _, check := range []struct {
		root *os.Root
		name string
		info os.FileInfo
	}{{images, "sandbox", sandboxInfo}, {root, "images", imagesInfo}} {
		after, e := check.root.Lstat(check.name)
		if e != nil || !runtimeSameSource(check.info, after) {
			return nil, "", backend.ErrLaunchFenceOwnership
		}
	}
	after, err := os.Lstat(spec.SourceDirectory)
	if err != nil || !runtimeSameSource(before, after) || !checkSourceDirectory(spec.SourceDirectory) {
		return nil, "", backend.ErrLaunchFenceOwnership
	}
	required := map[string]bool{"sandboxd/go.mod": false, "sandboxd/vendor/modules.txt": false}
	for _, f := range files {
		if _, ok := required[f.name]; ok && !f.directory {
			required[f.name] = true
		}
	}
	for _, present := range required {
		if !present {
			return nil, "", backend.ErrUnsupported
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	raw, err := previewTar(files)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if spec.Identity.SourceDigest != "" && spec.Identity.SourceDigest != digest {
		return nil, "", backend.ErrLaunchFenceOwnership
	}
	return files, digest, nil
}

func runtimeSameSource(before, after os.FileInfo) bool {
	return before != nil && after != nil && os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func runtimeChildRoot(parent *os.Root, name string) (*os.Root, os.FileInfo, error) {
	before, err := parent.Lstat(name)
	if err != nil {
		return nil, nil, err
	}
	if !before.IsDir() || before.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return nil, nil, backend.ErrUnsupported
	}
	child, err := parent.OpenRoot(name)
	if err != nil {
		return nil, nil, err
	}
	after, err := child.Stat(".")
	if err != nil || !runtimeSameSource(before, after) {
		child.Close()
		return nil, nil, backend.ErrLaunchFenceOwnership
	}
	return child, before, nil
}

// runtimePreparedContext accepts the bundled two-stage build dialect only.
// Prepare leaves the approved literal bases in place without downloading them;
// Build supplies both independently inspected immutable IDs after admission.
func runtimePreparedContext(files []previewFile, identity backend.RuntimeImageIdentity, baseIDs map[string]string) ([]byte, []string, error) {
	bases := []string{runtimeGoBase, runtimeNodeBase}
	if baseIDs != nil {
		if len(baseIDs) != len(bases) {
			return nil, nil, backend.ErrUnsupported
		}
		for _, base := range bases {
			if !previewImageID(baseIDs[base]) {
				return nil, nil, backend.ErrUnsupported
			}
		}
	}
	index := -1
	for i, f := range files {
		if f.name == "images/sandbox/Dockerfile" && !f.directory {
			if index >= 0 || len(f.data) > 1<<20 {
				return nil, nil, backend.ErrUnsupported
			}
			index = i
		}
	}
	if index < 0 {
		return nil, nil, backend.ErrUnsupported
	}
	labels := runtimeIdentityLabels(identity)
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	labelLine := "LABEL"
	for _, key := range keys {
		labelLine += fmt.Sprintf(" %s=%q", key, labels[key])
	}
	lines := strings.Split(strings.ReplaceAll(string(files[index].data), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines)+2)
	logical := ""
	fromCount := 0
	copies := map[string]bool{}
	for _, line := range lines {
		clean := strings.TrimSpace(line)
		if strings.ContainsAny(line, "\x00\r") || strings.Contains(line, "<<") {
			return nil, nil, backend.ErrUnsupported
		}
		if strings.HasPrefix(clean, "#") {
			directive := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(clean, "#")))
			if strings.HasPrefix(directive, "syntax") || strings.HasPrefix(directive, "escape") {
				return nil, nil, backend.ErrUnsupported
			}
			out = append(out, line)
			continue
		}
		if clean == "" {
			out = append(out, line)
			continue
		}
		continued := strings.HasSuffix(clean, "\\")
		logical += " " + strings.TrimSuffix(clean, "\\")
		fields := strings.Fields(logical)
		if continued {
			if len(fields) == 0 || fields[0] != "RUN" {
				return nil, nil, backend.ErrUnsupported
			}
			out = append(out, line)
			continue
		}
		instruction := strings.Join(fields, " ")
		logical = ""
		if len(fields) < 2 {
			return nil, nil, backend.ErrUnsupported
		}
		if fields[0] == "FROM" {
			expected := "FROM " + runtimeGoBase + " AS build"
			if fromCount == 1 {
				expected = "FROM " + runtimeNodeBase
			}
			if fromCount >= 2 || instruction != expected {
				return nil, nil, backend.ErrUnsupported
			}
			if baseIDs != nil {
				fields[1] = baseIDs[bases[fromCount]]
			}
			out = append(out, strings.Join(fields, " "), labelLine)
			fromCount++
			continue
		}
		if fromCount == 0 {
			return nil, nil, backend.ErrUnsupported
		}
		switch fields[0] {
		case "RUN":
			// The source is the trusted bundle, not a project Dockerfile.
			// RUN keeps its original shell text and ordinary continuations.
		case "COPY":
			allowed := fromCount == 1 && instruction == "COPY sandboxd ./sandboxd" || fromCount == 2 && (instruction == "COPY --from=build /out/sandboxd /usr/local/bin/sandboxd" || instruction == "COPY --from=build /usr/local/go /usr/local/go" || instruction == "COPY images/sandbox/entrypoint.sh /usr/local/bin/entrypoint.sh")
			if !allowed || copies[instruction] {
				return nil, nil, backend.ErrUnsupported
			}
			copies[instruction] = true
		case "WORKDIR":
			if fromCount == 1 && instruction != "WORKDIR /src" || fromCount == 2 && instruction != "WORKDIR /home/dev" {
				return nil, nil, backend.ErrUnsupported
			}
		case "ENV":
			if fromCount != 2 || instruction != `ENV PATH="/usr/local/go/bin:${PATH}"` {
				return nil, nil, backend.ErrUnsupported
			}
		case "USER":
			if fromCount != 2 || instruction != "USER dev" {
				return nil, nil, backend.ErrUnsupported
			}
		case "EXPOSE":
			if fromCount != 2 || instruction != "EXPOSE 8080" {
				return nil, nil, backend.ErrUnsupported
			}
		case "ENTRYPOINT":
			if fromCount != 2 || instruction != `ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]` {
				return nil, nil, backend.ErrUnsupported
			}
		default:
			// This also refuses LABEL, ARG, ONBUILD, VOLUME and remote ADD.
			return nil, nil, backend.ErrUnsupported
		}
		out = append(out, line)
	}
	if logical != "" || fromCount != 2 || len(copies) != 4 {
		return nil, nil, backend.ErrUnsupported
	}
	preparedFiles := append([]previewFile(nil), files...)
	preparedFiles[index].data = []byte(strings.Join(out, "\n"))
	archive, err := previewTar(preparedFiles)
	return archive, bases, err
}
