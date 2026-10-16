//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func assemble(ctx context.Context, manifest DocumentImageAssemblyManifest, inputDigest string) error {
	if os.Getuid() != 0 || os.Getgid() != 0 || !ValidateDocumentImageAssemblyManifest(manifest) || verifyAssemblySandbox() != nil {
		return errRefused
	}
	// Every path below is newly owned in the separately enforced /work tmpfs.
	if os.Mkdir("/work/staged", 0700) != nil || os.Mkdir("/work/rootfs", 0700) != nil {
		return errRefused
	}
	for _, item := range DocumentImageAssemblyInputs(manifest) {
		if copyPinned(ctx, "/input", "/work/staged", item) != nil {
			return errRefused
		}
	}
	// Closure flags in an input manifest cannot relabel the existing unresolved
	// dependency lock. Refuse it before any package/native payload execution.
	lockData, lockErr := readSmall(filepath.Join("/work/staged", manifest.DependencyLock.Path), 2<<20)
	if lockErr != nil || !validClosedDependencyLock(lockData) {
		return errRefused
	}
	manifestInfo, err := os.Lstat("/input/assembly-manifest.json")
	if err != nil {
		return errRefused
	}
	if copyPinned(ctx, "/input", "/work/staged", DocumentImageAssemblyInput{Path: "assembly-manifest.json", Bytes: manifestInfo.Size(), Digest: inputDigest}) != nil {
		return errRefused
	}
	if unpackBase(ctx, filepath.Join("/work/staged", manifest.BaseLayer.Path), "/work/rootfs") != nil {
		return errRefused
	}
	root := "/work/rootfs"
	luci, err := makeRootDir(root, "opt/luci", 0755)
	if err != nil || os.Chmod(luci, 0755) != nil {
		return errRefused
	}
	install, err := makeRootDir(root, "opt/luci/build-input", 0700)
	if err != nil {
		return errRefused
	}
	installInfo, err := os.Lstat(install)
	if err != nil || !installInfo.IsDir() {
		return errRefused
	}
	for _, item := range DocumentImageAssemblyInputs(manifest) {
		if item.Path == manifest.BaseLayer.Path {
			continue
		}
		if copyPinned(ctx, "/work/staged", install, item) != nil {
			return errRefused
		}
	}
	// The closed manifest retains exact build/source/notice bindings in output.
	if copyPinned(ctx, "/work/staged", install, DocumentImageAssemblyInput{Path: "assembly-manifest.json", Bytes: manifestInfo.Size(), Digest: inputDigest}) != nil {
		return errRefused
	}
	devDir, err := rooted(root, "dev")
	if err != nil {
		return errRefused
	}
	if os.MkdirAll(devDir, 0755) != nil {
		return errRefused
	}
	devNull := filepath.Join(devDir, "null")
	if _, err = os.Lstat(devNull); !os.IsNotExist(err) {
		return errRefused
	}
	// Only this owned character device is created. No /proc mount or fake file.
	if syscall.Mknod(devNull, syscall.S_IFCHR|0666, 0x103) != nil {
		return errRefused
	}
	nullInfo, err := os.Lstat(devNull)
	if err != nil || nullInfo.Mode()&os.ModeCharDevice == 0 {
		return errRefused
	}
	if stat, ok := nullInfo.Sys().(*syscall.Stat_t); !ok || stat.Rdev != 0x103 {
		return errRefused
	}
	args := []string{"--unpack"}
	for _, item := range manifest.Packages {
		args = append(args, "/opt/luci/build-input/"+item.Path)
	}
	if runChroot(ctx, root, "/usr/bin/dpkg", args) != nil {
		return errRefused
	}
	if runChroot(ctx, root, "/usr/bin/dpkg", []string{"--configure", "-a"}) != nil {
		return errRefused
	}
	if runChroot(ctx, root, "/usr/bin/python3.11", []string{"-I", "-m", "pip", "install", "--break-system-packages", "--no-index", "--no-deps", "--no-cache-dir", "--no-compile", "--require-hashes", "--only-binary=:all:", "--find-links=/opt/luci/build-input/wheels", "-r", "/opt/luci/build-input/wheel-requirements.txt"}) != nil {
		return errRefused
	}
	if ensureQuietProcesses(ctx) != nil {
		return errRefused
	}
	for _, name := range []string{"input", "opt/luci/tessdata", "opt/luci/licenses"} {
		if _, err = makeRootDir(root, name, 0755); err != nil {
			return errRefused
		}
	}
	original, err := rooted(root, "input/original")
	if err != nil {
		return errRefused
	}
	placeholder, err := os.OpenFile(original, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0444)
	if err != nil {
		return errRefused
	}
	if placeholder.Close() != nil {
		return errRefused
	}
	for _, language := range []string{"eng", "osd"} {
		if copyOwnedFile(ctx, root, "usr/share/tesseract-ocr/5/tessdata/"+language+".traineddata", "opt/luci/tessdata/"+language+".traineddata", 128<<20) != nil {
			return errRefused
		}
	}
	for _, pair := range [][2]string{{manifest.Extractor.Path, "opt/luci/document_extractor.py"}, {manifest.DependencyLock.Path, "opt/luci/dependency-lock.json"}, {"assembly-manifest.json", "opt/luci/assembly-manifest.json"}} {
		if copyOwnedFile(ctx, root, "opt/luci/build-input/"+pair[0], pair[1], DocumentImageAssemblyManifestMaxBytes*8) != nil {
			return errRefused
		}
	}
	for _, item := range manifest.Notices {
		if copyOwnedFile(ctx, root, "opt/luci/build-input/"+item.Path, "opt/luci/licenses/"+strings.TrimPrefix(item.Path, "notices/"), 1<<20) != nil {
			return errRefused
		}
	}
	if runChroot(ctx, root, "/usr/bin/python3.11", []string{"-I", "/opt/luci/build-input/verify_image.py", "/opt/luci/dependency-lock.json"}) != nil || ensureQuietProcesses(ctx) != nil {
		return errRefused
	}
	// Retire only precisely retained, positively owned transient paths. A
	// maintainer script replacing either object is a refusal, not cleanup consent.
	freshNull, e1 := os.Lstat(devNull)
	freshInstall, e2 := os.Lstat(install)
	if e1 != nil || e2 != nil || !os.SameFile(nullInfo, freshNull) || !os.SameFile(installInfo, freshInstall) || !freshInstall.IsDir() || freshInstall.Mode()&os.ModeSymlink != 0 {
		return errRefused
	}
	if os.Remove(devNull) != nil || os.RemoveAll(install) != nil {
		return errRefused
	}
	return emitOCI(ctx, root, manifest, inputDigest, os.Stdout)
}

// Source opening refuses redirects and hardlinks; copying completes and hashes
// into owned tmpfs before dpkg/pip can execute any payload. No member extraction
// into a host path or writable project mount exists.
func copyPinned(ctx context.Context, source, destination string, item DocumentImageAssemblyInput) error {
	if !inputName(item.Path) || item.Bytes < 1 || item.Bytes > DocumentImageAssemblyInputMaxBytes || !documentHex(item.Digest) {
		return errRefused
	}
	current := source
	for _, part := range strings.Split(item.Path, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errRefused
		}
	}
	before, err := os.Lstat(current)
	if err != nil || !before.Mode().IsRegular() || before.Size() != item.Bytes {
		return errRefused
	}
	stat, ok := before.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return errRefused
	}
	input, err := os.Open(current)
	if err != nil {
		return errRefused
	}
	defer input.Close()
	opened, err := input.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return errRefused
	}
	name := filepath.Join(destination, filepath.FromSlash(item.Path))
	if os.MkdirAll(filepath.Dir(name), 0700) != nil {
		return errRefused
	}
	output, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0400)
	if err != nil {
		return errRefused
	}
	h := sha256.New()
	var buffer [64 << 10]byte
	var total int64
	for {
		if ctx.Err() != nil {
			output.Close()
			return errRefused
		}
		n, e := input.Read(buffer[:])
		total += int64(n)
		if total > item.Bytes {
			output.Close()
			return errRefused
		}
		_, _ = h.Write(buffer[:n])
		if written, writeErr := output.Write(buffer[:n]); writeErr != nil || written != n {
			output.Close()
			return errRefused
		}
		if e == io.EOF {
			break
		}
		if e != nil || n == 0 {
			output.Close()
			return errRefused
		}
	}
	if output.Close() != nil || total != item.Bytes || hex.EncodeToString(h.Sum(nil)) != item.Digest {
		return errRefused
	}
	return nil
}
func copyOwnedFile(ctx context.Context, root, source, destination string, maximum int64) error {
	in, err := rooted(root, source)
	if err != nil {
		return errRefused
	}
	info, err := os.Lstat(in)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
		return errRefused
	}
	file, err := os.Open(in)
	if err != nil {
		return errRefused
	}
	defer file.Close()
	out, err := rooted(root, destination)
	if err != nil {
		return errRefused
	}
	if _, err = makeRootDir(root, path.Dir(destination), 0755); err != nil {
		return errRefused
	}
	target, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0444)
	if err != nil {
		return errRefused
	}
	written, err := io.Copy(target, io.LimitReader(&contextReader{ctx: ctx, reader: file}, maximum+1))
	closed := target.Close()
	if err != nil || closed != nil || written != info.Size() {
		return errRefused
	}
	return nil
}

type diagnosticWriter struct {
	mu    sync.Mutex
	file  *os.File
	bytes int
}

func (w *diagnosticWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(p) > (1<<20)-w.bytes {
		return 0, errRefused
	}
	n, err := w.file.Write(p)
	w.bytes += n
	return n, err
}
func runChroot(ctx context.Context, root, executable string, args []string) error {
	if ctx.Err() != nil || root != "/work/rootfs" {
		return errRefused
	}
	// This function is private and every caller supplies one fixed allowlisted
	// executable/argv. Manifest values can only be confined data filenames.
	switch executable {
	case "/usr/bin/dpkg", "/usr/bin/python3.11":
	default:
		return errRefused
	}
	file, err := os.OpenFile("/tmp/assembly-diagnostics", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return errRefused
	}
	defer file.Close()
	writer := &diagnosticWriter{file: file}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: root}
	cmd.Dir = "/"
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/tmp", "TMPDIR=/tmp", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "DEBIAN_FRONTEND=noninteractive", "PIP_NO_INDEX=1", "PIP_DISABLE_PIP_VERSION_CHECK=1", "OMP_THREAD_LIMIT=1", "OMP_NUM_THREADS=1", "PYTHONDONTWRITEBYTECODE=1"}
	cmd.Stdout, cmd.Stderr = writer, writer
	cmd.WaitDelay = time.Second
	if cmd.Run() != nil || writer.bytes > 1<<20 || ctx.Err() != nil {
		return errRefused
	}
	return nil
}
func ensureQuietProcesses(ctx context.Context) error {
	// Outer private PID namespace only. Never mount or synthesize inner /proc.
	for attempt := 0; attempt < 10; attempt++ {
		if ctx.Err() != nil {
			return errRefused
		}
		file, err := os.Open("/proc")
		if err != nil {
			return errRefused
		}
		names, err := file.Readdirnames(1024)
		file.Close()
		if err != nil && err != io.EOF {
			return errRefused
		}
		if len(names) >= 1024 {
			return errRefused
		}
		quiet := true
		count := 0
		for _, name := range names {
			pid, e := strconv.Atoi(name)
			if e == nil {
				count++
				if pid != 1 && pid != os.Getpid() {
					quiet = false
				}
			}
		}
		if count > 64 {
			return errRefused
		}
		if quiet && count >= 1 {
			return nil
		}
		select {
		case <-ctx.Done():
			return errRefused
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errRefused
}
