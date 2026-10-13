//go:build linux

// The trusted template invokes this once as root through authenticated envd.
// Existing sandboxd/entrypoint/upstream harnesses remain the execution engine.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/microvmguest"
)

func run() error {
	var binding e2bbe.TransferBinding
	var kernelIdentity e2bbe.CaptureIdentity
	archive := flag.String("archive", "/luci-transfer/input.tar", "fixed uploaded input")
	digest := flag.String("archive-sha256", "", "exact reviewed archive digest")
	flag.StringVar(&binding.Identity.OwnerID, "owner", "", "controller owner")
	flag.StringVar(&binding.Identity.ProjectID, "project", "", "project")
	flag.StringVar(&binding.Identity.TaskID, "task", "", "task")
	flag.StringVar(&binding.Identity.AttemptID, "attempt", "", "attempt")
	flag.StringVar(&binding.Identity.OperationID, "operation", "", "operation")
	flag.StringVar(&binding.SourceDigest, "source-sha256", "", "reviewed source digest")
	flag.StringVar(&binding.ContextDigest, "context-sha256", "", "reviewed context digest")
	flag.StringVar(&kernelIdentity.SandboxID, "sandbox", "", "exact owned microVM")
	flag.StringVar(&kernelIdentity.RequestDigest, "request-sha256", "", "exact reserved request")
	flag.StringVar(&kernelIdentity.LaunchDigest, "launch-sha256", "", "exact reserved upstream launch")
	flag.Parse()
	if os.Geteuid() != 0 || flag.NArg() != 0 || *archive != "/luci-transfer/input.tar" {
		return microvmguest.ErrRefused
	}
	for _, p := range []string{"/luci-transfer", "/luci-work"} {
		info, e := os.Lstat(p)
		if e != nil {
			return e
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || st.Uid != 0 || info.Mode().Perm()&0022 != 0 {
			return microvmguest.ErrRefused
		}
	}
	info, e := os.Lstat(*archive)
	if e != nil {
		return e
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || st.Uid != 0 || st.Nlink != 1 || info.Mode().Perm()&0077 != 0 {
		return microvmguest.ErrRefused
	}
	if os.Getenv("LUCI_ATTACH_TOKEN") == "" || os.Getenv("LUCI_COMMAND") == "" {
		return errors.New("trusted task execution configuration is missing")
	}
	binding.Direction = "input"
	input, e := os.Open(*archive)
	if e != nil {
		return e
	}
	defer input.Close()
	prepared, e := microvmguest.Prepare(context.Background(), "/luci-work/attempt", input, binding, *digest)
	if e != nil {
		return e
	}
	if e = microvmguest.Protect(prepared, 1001, 1001); e != nil {
		return e
	}
	if _, e = microvmguest.InitializeRepository(context.Background(), prepared); e != nil {
		return e
	}
	if e = microvmguest.ExposeReviewedContext(prepared); e != nil {
		return e
	}
	if e = os.Chdir(filepath.Join(prepared.Directory(), "source")); e != nil {
		return e
	}
	if e = microvmguest.ReserveLaunch(context.Background(), prepared); e != nil {
		return e
	}
	kernelIdentity.Identity = binding.Identity
	kernelIdentity.InputBundleDigest = *digest
	kernelIdentity.SourceDigest, kernelIdentity.ContextDigest = binding.SourceDigest, binding.ContextDigest
	if _, e = microvmguest.BindTaskCgroup(context.Background(), prepared, kernelIdentity); e != nil {
		return e
	}
	// The trusted existing entrypoint drops UID/GID and all capabilities before
	// invoking sandboxd. No token or model credential is copied into a receipt.
	if e = os.Setenv("LUCI_HOST_UID", "1001"); e != nil {
		return e
	}
	if e = os.Setenv("LUCI_HOST_GID", "1001"); e != nil {
		return e
	}
	return syscall.Exec("/usr/local/bin/entrypoint.sh", []string{"/usr/local/bin/entrypoint.sh"}, os.Environ())
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "Luci guest bootstrap refused or remains unconfirmed:", e)
		os.Exit(1)
	}
}
