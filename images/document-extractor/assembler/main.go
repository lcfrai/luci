// The separately reviewed offline helper runs only inside its exact contained
// assembly profile. No host installer, Docker client, image registration or URL.
package main

import (
	"context"
	"crypto/sha256"
	"debug/elf"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const resultMagic = "LUCI-DOCUMENT-OCI/1\n"
const resultEnd = "\nLUCI-DOCUMENT-OCI-END/1\n"

var errRefused = errors.New("fixed offline assembly refused")

func main() {
	if err := run(); err != nil {
		os.Exit(2)
	}
}
func run() error {
	flags := flag.NewFlagSet("document-image-assembler", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	manifestPath := flags.String("manifest", "", "")
	inputDigest := flags.String("input-digest", "", "")
	sourceDigest := flags.String("source-digest", "", "")
	helperDigest := flags.String("helper-digest", "", "")
	recipeDigest := flags.String("recipe-digest", "", "")
	if flags.Parse(os.Args[1:]) != nil || flags.NArg() != 0 || len(os.Args) != 11 || *manifestPath != "/input/assembly-manifest.json" || !documentHex(*inputDigest) || !documentHex(*sourceDigest) || !documentHex(*helperDigest) || *recipeDigest != DocumentImageAssemblyRecipeDigest() {
		return errRefused
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()
	self, err := os.Open("/tool/luci-document-image-assembler")
	if err != nil {
		return errRefused
	}
	defer self.Close()
	info, err := self.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > DocumentImageAssemblyHelperMaxBytes || hashExact(ctx, self, info.Size(), *helperDigest) != nil {
		return errRefused
	}
	if _, err = self.Seek(0, io.SeekStart); err != nil {
		return errRefused
	}
	binary, err := elf.NewFile(self)
	if err != nil || binary.Class != elf.ELFCLASS64 || binary.Machine != elf.EM_X86_64 || binary.Type != elf.ET_EXEC {
		return errRefused
	}
	for _, program := range binary.Progs {
		if program.Type == elf.PT_INTERP {
			return errRefused
		}
	}
	manifest, err := readManifest(ctx, *manifestPath, *inputDigest)
	if err != nil || manifest.SourceDigest != *sourceDigest || manifest.HelperDigest != *helperDigest || manifest.RecipeDigest != *recipeDigest {
		return errRefused
	}
	return assemble(ctx, manifest, *inputDigest)
}
func hashExact(ctx context.Context, reader io.Reader, size int64, digest string) error {
	if size < 1 || size > DocumentImageAssemblyInputMaxBytes {
		return errRefused
	}
	h := sha256.New()
	var buffer [64 << 10]byte
	var total int64
	for {
		if ctx.Err() != nil {
			return errRefused
		}
		n, err := reader.Read(buffer[:])
		total += int64(n)
		if total > size {
			return errRefused
		}
		_, _ = h.Write(buffer[:n])
		if err == io.EOF {
			break
		}
		if err != nil || n == 0 {
			return errRefused
		}
	}
	if total != size || hex.EncodeToString(h.Sum(nil)) != digest {
		return errRefused
	}
	return nil
}
func readManifest(ctx context.Context, name, digest string) (DocumentImageAssemblyManifest, error) {
	var manifest DocumentImageAssemblyManifest
	file, err := os.Open(name)
	if err != nil {
		return manifest, errRefused
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > DocumentImageAssemblyManifestMaxBytes {
		return manifest, errRefused
	}
	data, err := io.ReadAll(io.LimitReader(file, DocumentImageAssemblyManifestMaxBytes+1))
	h := sha256.Sum256(data)
	if err != nil || int64(len(data)) != info.Size() || hex.EncodeToString(h[:]) != digest || ctx.Err() != nil || uniqueJSON(data) != nil {
		return manifest, errRefused
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&manifest) != nil || !ValidateDocumentImageAssemblyManifest(manifest) {
		return manifest, errRefused
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return manifest, errRefused
	}
	return manifest, nil
}
func uniqueJSON(data []byte) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	tokens := 0
	var value func(int) error
	value = func(depth int) error {
		if depth > 16 || tokens > 20000 {
			return errRefused
		}
		tokens++
		token, err := decoder.Token()
		if err != nil {
			return errRefused
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				token, err = decoder.Token()
				key, ok := token.(string)
				if err != nil || !ok || seen[key] {
					return errRefused
				}
				seen[key] = true
				if err = value(depth + 1); err != nil {
					return err
				}
			}
			token, err = decoder.Token()
			if err != nil || token != json.Delim('}') {
				return errRefused
			}
		case '[':
			for decoder.More() {
				if err = value(depth + 1); err != nil {
					return err
				}
			}
			token, err = decoder.Token()
			if err != nil || token != json.Delim(']') {
				return errRefused
			}
		default:
			return errRefused
		}
		return nil
	}
	if value(0) != nil {
		return errRefused
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errRefused
	}
	return nil
}
func inputName(name string) bool {
	return relativeUnixName(name) && !strings.Contains(name, ":")
}

// These paths exist only inside a Linux rootfs/archive, never as Windows host
// staging/output names. Multiarch package metadata legitimately contains ':'.
func relativeUnixName(name string) bool {
	if !utf8.ValidString(name) {
		return false
	}
	for _, c := range name {
		if c < 32 || c == 127 {
			return false
		}
	}
	return name != "" && len(name) <= 1024 && !strings.ContainsAny(name, "\\\x00\r\n") && !strings.HasPrefix(name, "/") && path.Clean(name) == name && name != "." && name != ".." && !strings.HasPrefix(name, "../")
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if r.ctx.Err() != nil {
		return 0, errRefused
	}
	if len(p) > 64<<10 {
		p = p[:64<<10]
	}
	return r.reader.Read(p)
}

// Virtual-root resolution treats absolute symlinks as chroot paths. It never
// follows an archive/package link into the outer /input, /tool, /proc or root.
func rooted(root, name string) (string, error) {
	if !relativeUnixName(name) {
		return "", errRefused
	}
	for links := 0; links < 32; links++ {
		parts := strings.Split(name, "/")
		replaced := false
		for end := 1; end <= len(parts); end++ {
			prefix := strings.Join(parts[:end], "/")
			current := filepath.Join(root, filepath.FromSlash(prefix))
			info, err := os.Lstat(current)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return "", errRefused
			}
			if info.Mode()&os.ModeSymlink == 0 {
				if end < len(parts) && !info.IsDir() {
					return "", errRefused
				}
				continue
			}
			target, err := os.Readlink(current)
			if err != nil || strings.ContainsAny(target, "\\\x00\r\n") {
				return "", errRefused
			}
			if strings.HasPrefix(target, "/") {
				name = strings.TrimPrefix(target, "/")
			} else {
				name = path.Join(path.Dir(prefix), target)
			}
			if end < len(parts) {
				name = path.Join(name, strings.Join(parts[end:], "/"))
			}
			name = path.Clean(name)
			if !relativeUnixName(name) {
				return "", errRefused
			}
			replaced = true
			break
		}
		if !replaced {
			return filepath.Join(root, filepath.FromSlash(name)), nil
		}
	}
	return "", errRefused
}
func makeRootDir(root, name string, mode os.FileMode) (string, error) {
	if name == "." {
		info, err := os.Lstat(root)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errRefused
		}
		return root, nil
	}
	full, err := rooted(root, name)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(full, mode); err != nil {
		return "", errRefused
	}
	return full, nil
}
