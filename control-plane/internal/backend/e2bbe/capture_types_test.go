package e2bbe

import (
	"strings"
	"testing"
	"time"
)

func captureFixture() (Record, CaptureIdentity, CaptureReceipt) {
	req := taskRequest("alpha", "one")
	r := Record{Request: req, RequestDigest: digest(req), SandboxID: "vm-one", LaunchRequested: true, LaunchDigest: strings.Repeat("4", 64), ProcessPID: 321}
	id := CaptureIdentity{Version: 1, Identity: req.Identity, RequestDigest: r.RequestDigest, SandboxID: r.SandboxID, LaunchDigest: r.LaunchDigest, ProcessPID: r.ProcessPID, InputBundleDigest: req.BundleDigest, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, BootID: "10000000-0000-0000-0000-000000000001", CgroupRelativePath: "luci-tasks/" + req.Identity.OperationID, CgroupInode: 37}
	receipt := CaptureReceipt{Version: 1, CaptureID: "capture-one", Identity: id, ArchiveSHA256: strings.Repeat("5", 64), ArchiveBytes: 4096, CapturedAt: time.Unix(1700000000, 0).UTC(), SourceManifestSHA256: strings.Repeat("6", 64), ChangeManifestSHA256: strings.Repeat("7", 64)}
	return r, id, receipt
}

func TestCaptureRequiresExactPreviouslyObservedLaunchKernelIdentity(t *testing.T) {
	r, id, receipt := captureFixture()
	if e := ValidateCaptureIdentity(id, r); e != nil {
		t.Fatal(e)
	}
	if e := ValidateCaptureReceipt(receipt, id, r); e != nil {
		t.Fatal(e)
	}
	for name, change := range map[string]func(*CaptureIdentity){
		"foreign-task":   func(v *CaptureIdentity) { v.Identity.TaskID = "someone-else" },
		"foreign-VM":     func(v *CaptureIdentity) { v.SandboxID = "vm-other" },
		"request":        func(v *CaptureIdentity) { v.RequestDigest = strings.Repeat("9", 64) },
		"launch":         func(v *CaptureIdentity) { v.LaunchDigest = strings.Repeat("9", 64) },
		"PID":            func(v *CaptureIdentity) { v.ProcessPID++ },
		"archive":        func(v *CaptureIdentity) { v.InputBundleDigest = strings.Repeat("9", 64) },
		"source":         func(v *CaptureIdentity) { v.SourceDigest = strings.Repeat("9", 64) },
		"context":        func(v *CaptureIdentity) { v.ContextDigest = strings.Repeat("9", 64) },
		"zero-boot":      func(v *CaptureIdentity) { v.BootID = "00000000-0000-0000-0000-000000000000" },
		"boot-shape":     func(v *CaptureIdentity) { v.BootID = "unknown" },
		"zero-inode":     func(v *CaptureIdentity) { v.CgroupInode = 0 },
		"foreign-group":  func(v *CaptureIdentity) { v.CgroupRelativePath = "luci-tasks/foreign" },
		"absolute-group": func(v *CaptureIdentity) { v.CgroupRelativePath = "/sys/fs/cgroup/" + v.CgroupRelativePath },
		"version":        func(v *CaptureIdentity) { v.Version++ },
	} {
		t.Run(name, func(t *testing.T) {
			wrong := id
			change(&wrong)
			if ValidateCaptureIdentity(wrong, r) == nil {
				t.Fatal("wrong kernel launch binding accepted")
			}
		})
	}
	for _, field := range []string{"boot", "inode"} {
		t.Run("capture-cannot-rebind-"+field, func(t *testing.T) {
			wrong := receipt
			if field == "boot" {
				wrong.Identity.BootID = "20000000-0000-0000-0000-000000000001"
			} else {
				wrong.Identity.CgroupInode++
			}
			if ValidateCaptureIdentity(wrong.Identity, r) != nil {
				t.Fatal("fixture must retain valid shape")
			}
			if ValidateCaptureReceipt(wrong, id, r) == nil {
				t.Fatal("collector replaced authoritative launch kernel identity")
			}
		})
	}
	for _, field := range []string{"unlaunched", "unknown-PID"} {
		t.Run(field, func(t *testing.T) {
			wrong := r
			if field == "unlaunched" {
				wrong.LaunchRequested = false
			} else {
				wrong.ProcessPID = 0
			}
			if ValidateCaptureIdentity(id, wrong) == nil {
				t.Fatal("unconfirmed launch accepted")
			}
		})
	}
}

func TestGuestCaptureShapeCannotReplaceControllerAcknowledgement(t *testing.T) {
	r, id, _ := captureFixture()
	if err := ValidateCaptureIdentityShape(id); err != nil {
		t.Fatal(err)
	}
	// A well-formed but different operation belongs to a different attempt.
	// The guest shape boundary cannot assert a controller acknowledgement.
	foreign := id
	foreign.Identity.OperationID = "operation-other"
	foreign.CgroupRelativePath = "luci-tasks/operation-other"
	if ValidateCaptureIdentityShape(foreign) != nil || ValidateCaptureIdentity(foreign, r) == nil {
		t.Fatal("shape validation replaced exact controller ownership")
	}
	for name, change := range map[string]func(*CaptureIdentity){
		"owner":   func(v *CaptureIdentity) { v.Identity.OwnerID = "" },
		"project": func(v *CaptureIdentity) { v.Identity.ProjectID = "../other" },
		"task":    func(v *CaptureIdentity) { v.Identity.TaskID = "other/task" },
		"attempt": func(v *CaptureIdentity) { v.Identity.AttemptID = "" },
		"operation": func(v *CaptureIdentity) {
			v.Identity.OperationID = "../escape"
			v.CgroupRelativePath = "luci-tasks/../escape"
		},
		"VM":      func(v *CaptureIdentity) { v.SandboxID = "" },
		"request": func(v *CaptureIdentity) { v.RequestDigest = "not-a-hash" },
		"launch":  func(v *CaptureIdentity) { v.LaunchDigest = "" },
		"input":   func(v *CaptureIdentity) { v.InputBundleDigest = "" },
		"source":  func(v *CaptureIdentity) { v.SourceDigest = "" },
		"context": func(v *CaptureIdentity) { v.ContextDigest = "" },
		"PID":     func(v *CaptureIdentity) { v.ProcessPID = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			wrong := id
			change(&wrong)
			if ValidateCaptureIdentityShape(wrong) == nil {
				t.Fatal("malformed guest kernel identity accepted")
			}
		})
	}
}

func TestCaptureReceiptBoundsAndMetadataDoNotGrantTaskSuccess(t *testing.T) {
	r, id, receipt := captureFixture()
	for name, change := range map[string]func(*CaptureReceipt){
		"missing-id":              func(v *CaptureReceipt) { v.CaptureID = "" },
		"missing-hash":            func(v *CaptureReceipt) { v.ArchiveSHA256 = "" },
		"zero-bytes":              func(v *CaptureReceipt) { v.ArchiveBytes = 0 },
		"oversize":                func(v *CaptureReceipt) { v.ArchiveBytes = MaxCaptureArchiveBytes + 1 },
		"missing-time":            func(v *CaptureReceipt) { v.CapturedAt = time.Time{} },
		"missing-source-manifest": func(v *CaptureReceipt) { v.SourceManifestSHA256 = "" },
		"missing-change-manifest": func(v *CaptureReceipt) { v.ChangeManifestSHA256 = "" },
	} {
		t.Run(name, func(t *testing.T) {
			wrong := receipt
			change(&wrong)
			if ValidateCaptureReceipt(wrong, id, r) == nil {
				t.Fatal("incomplete capture accepted")
			}
		})
	}
	// Pure receipt validation deliberately does not require a successful native
	// outcome: failed task output is useful evidence but cannot become success.
	if e := ValidateCaptureReceipt(receipt, id, r); e != nil {
		t.Fatal(e)
	}
}

func TestNativeExitReceiptRejectsTextAndWrongLaunchBindings(t *testing.T) {
	r, _, _ := captureFixture()
	for _, reason := range []string{"exit status 0", "exit status 17", "signal: killed", "signal: segmentation fault (core dumped)"} {
		v := NativeExitReceipt{Version: 1, SandboxID: r.SandboxID, RequestDigest: r.RequestDigest, LaunchDigest: r.LaunchDigest, ProcessPID: r.ProcessPID, Reason: reason, FrameSHA256: bytesDigest(append([]byte{0x03}, []byte(reason)...)), ObservedAt: time.Unix(1700000000, 0).UTC()}
		if e := ValidateNativeExitReceipt(v, r); e != nil {
			t.Fatal(reason, e)
		}
		v.FrameSHA256 = bytesDigest(append([]byte{0x00}, []byte(reason)...))
		if ValidateNativeExitReceipt(v, r) == nil {
			t.Fatal("stdout status text became native evidence")
		}
	}
	v := NativeExitReceipt{Version: 1, SandboxID: r.SandboxID, RequestDigest: r.RequestDigest, LaunchDigest: r.LaunchDigest, ProcessPID: r.ProcessPID, Reason: "exit status 0", FrameSHA256: bytesDigest([]byte("\x03exit status 0")), ObservedAt: time.Unix(1700000000, 0).UTC()}
	for name, change := range map[string]func(*NativeExitReceipt){
		"VM":           func(v *NativeExitReceipt) { v.SandboxID = "other" },
		"launch":       func(v *NativeExitReceipt) { v.LaunchDigest = strings.Repeat("9", 64) },
		"PID":          func(v *NativeExitReceipt) { v.ProcessPID++ },
		"message":      func(v *NativeExitReceipt) { v.Reason = "model says exit status 0" },
		"status-range": func(v *NativeExitReceipt) { v.Reason = "exit status 256" },
		"missing-time": func(v *NativeExitReceipt) { v.ObservedAt = time.Time{} },
	} {
		t.Run(name, func(t *testing.T) {
			wrong := v
			change(&wrong)
			if ValidateNativeExitReceipt(wrong, r) == nil {
				t.Fatal("foreign or non-native receipt accepted")
			}
		})
	}
}

func TestBootstrapArgsCarryExactFencedLaunchIdentity(t *testing.T) {
	r, _, _ := captureFixture()
	r.Request.Identity.RunID = "run-owned"
	r.RequestDigest = digest(r.Request)
	args := bootstrapArgs(r)
	values := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) || values[args[i]] != "" {
			t.Fatal("invalid or duplicate bootstrap arguments")
		}
		values[args[i]] = args[i+1]
	}
	for flag, want := range map[string]string{"--run": r.Request.Identity.RunID, "--sandbox": r.SandboxID, "--request-sha256": r.RequestDigest, "--launch-sha256": r.LaunchDigest} {
		if values[flag] != want || want == "" {
			t.Fatal(flag, values[flag], want)
		}
	}
}
