package api

import (
	"errors"
	"strings"
	"unicode/utf16"
)

var errDebianHostFacts = errors.New("bounded current-host facts unavailable")
var errDebianHostCensus = errors.New("current-host observation requires original-owner census admission")

const (
	debianHostMachineUnknown = uint16(0)
	debianHostMachineI386 = uint16(0x014c)
	debianHostMachineAMD64 = uint16(0x8664)
	debianHostDirectoryUnits = 32768
)

// These private values describe requirements and observations, never launch
// authority, package authentication, module proof, or loaded-image attestation.
// Arrays and strings leave no mutable caller-owned slice/map aliases.
type debianHostRequirements struct {
	minimumBuild uint32
	i386Required bool
	toolDomainDigest string
	moduleCount int
	modules [128]string
}

func debianHostRequirementsFromDomain(d *assemblyDebianToolDomainV1) (debianHostRequirements, error) {
	if d == nil || !assemblyToolPinnedHex(d.digest) { return debianHostRequirements{}, errDebianHostFacts }
	v := d.declaredHostPrerequisites()
	if v.OS != "windows" || v.HostArchitecture != "amd64" || v.MinimumWindowsBuild == 0 || v.MinimumWindowsBuild > 4294967295 || v.CRTMode != "supported-system-ucrt" || len(v.SystemModules) == 0 || len(v.SystemModules) > 128 {
		return debianHostRequirements{}, errDebianHostFacts
	}
	r := debianHostRequirements{minimumBuild:uint32(v.MinimumWindowsBuild), i386Required:v.RequiresWOW64, toolDomainDigest:d.digest, moduleCount:len(v.SystemModules)}
	copy(r.modules[:], v.SystemModules)
	return r, nil
}

func (r debianHostRequirements) valid() bool {
	return r.minimumBuild != 0 && assemblyToolPinnedHex(r.toolDomainDigest) && r.moduleCount > 0 && r.moduleCount <= len(r.modules)
}

type debianHostFacts struct {
	processMachine, nativeMachine uint16
	major, minor, build, platform uint32
	i386Queried, i386Supported bool
	nativeDirectory, i386Directory string
	toolDomainDigest string
}

func validateDebianHostFacts(r debianHostRequirements, f debianHostFacts) error {
	if !r.valid() || f.toolDomainDigest != r.toolDomainDigest || f.processMachine != debianHostMachineUnknown || f.nativeMachine != debianHostMachineAMD64 || f.platform != 2 || f.major < 10 || f.build < r.minimumBuild || !debianHostDirectoryValid(f.nativeDirectory) {
		return errDebianHostFacts
	}
	if r.i386Required {
		if !f.i386Queried || !f.i386Supported || !debianHostDirectoryValid(f.i386Directory) { return errDebianHostFacts }
	} else if f.i386Queried || f.i386Supported || f.i386Directory != "" { return errDebianHostFacts }
	return nil
}

func debianHostDirectoryValid(v string) bool {
	if len(v) < 3 || len(v) > (debianHostDirectoryUnits-1)*4 || v[1] != ':' || v[2] != '\\' || !(v[0] >= 'A' && v[0] <= 'Z' || v[0] >= 'a' && v[0] <= 'z') || strings.ContainsAny(v[2:], "/:\x00") { return false }
	for _, part := range strings.Split(v[3:], "\\") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") { return false }
		for _, c := range part { if c < 32 { return false } }
	}
	return true
}

// n is the returned length excluding NUL. No resizing or second query is used.
// A full buffer, missing terminator, embedded NUL or invalid UTF-16 refuses.
func decodeDebianHostDirectory(b *[debianHostDirectoryUnits]uint16, n uint32) (string, error) {
	if b == nil || n == 0 || n >= uint32(len(b)) || b[n] != 0 { return "", errDebianHostFacts }
	for i := uint32(0); i < n; i++ {
		c := b[i]
		if c == 0 { return "", errDebianHostFacts }
		if c >= 0xd800 && c <= 0xdbff {
			if i+1 >= n || b[i+1] < 0xdc00 || b[i+1] > 0xdfff { return "", errDebianHostFacts }; i++
		} else if c >= 0xdc00 && c <= 0xdfff { return "", errDebianHostFacts }
	}
	v := string(utf16.Decode(b[:n]))
	if !debianHostDirectoryValid(v) { return "", errDebianHostFacts }
	return v, nil
}
