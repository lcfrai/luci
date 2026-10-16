//go:build linux

package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestSandboxRejectsWritableHostDiskMissingMountAndExtraCapability(t *testing.T) {
	rows := []string{}
	id := 1
	for name, mode := range map[string]string{"/": "ro", "/input": "ro", "/tool/luci-document-image-assembler": "ro", "/etc/hosts": "ro", "/etc/hostname": "ro", "/etc/resolv.conf": "ro", "/work": "rw", "/tmp": "rw"} {
		rows = append(rows, fmt.Sprintf("%d 0 0:1 / %s %s - tmpfs none %s", id, name, mode, mode))
		id++
	}
	valid := strings.Join(rows, "\n")
	if validateAssemblyMounts([]byte(valid)) != nil {
		t.Fatal("fixed inert mount fixture refused")
	}
	if validateAssemblyMounts([]byte(valid+"\n51 0 0:1 /null /proc/keys rw - tmpfs tmpfs rw")) != nil {
		t.Fatal("known null-mask projection refused")
	}
	for _, bad := range []string{valid + "\n50 0 0:1 / /writable rw - ext4 /disk rw", valid + "\n51 0 0:1 /host-file /proc/keys rw - tmpfs tmpfs rw", strings.Replace(valid, "/input ro", "/input rw", 1), strings.Replace(valid, "/work rw", "/work ro", 1)} {
		if validateAssemblyMounts([]byte(bad)) == nil {
			t.Fatal("unsafe mount accepted")
		}
	}
	const caps = uint64((1 << 0) | (1 << 1) | (1 << 3) | (1 << 4) | (1 << 6) | (1 << 7) | (1 << 18) | (1 << 27))
	status := fmt.Sprintf("CapEff:\t%x\nCapPrm:\t%x\nCapBnd:\t%x\nCapAmb:\t0\nCapInh:\t0\nNoNewPrivs:\t1\nSeccomp:\t2\n", caps, caps, caps)
	if validateAssemblyStatus([]byte(status)) != nil {
		t.Fatal("fixed capability fixture refused")
	}
	for _, bad := range []string{strings.Replace(status, fmt.Sprintf("CapEff:\t%x", caps), fmt.Sprintf("CapEff:\t%x", caps|(1<<21)), 1), strings.Replace(status, "NoNewPrivs:\t1", "NoNewPrivs:\t0", 1), strings.Replace(status, "Seccomp:\t2", "Seccomp:\t0", 1)} {
		if validateAssemblyStatus([]byte(bad)) == nil {
			t.Fatal("unsafe caps/seccomp accepted")
		}
	}
}
