package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

const (
	assemblyToolDomainLimit = 64 << 10
	assemblyToolDomainRole = "support:manifests/windows-input-proof-tool-closure-v1.json"
	// Descriptor included: product leaves44 slots for the existing owner's
	// copied/output/request purposes. This is static accounting, not live proof.
	assemblyToolProductProfileObjectLimit = 212
)

type assemblyToolProducerFamily string

const (
	assemblyToolLegacyProducerFamily assemblyToolProducerFamily = ""
	assemblyToolProductProducerFamily assemblyToolProducerFamily = assemblyDebianProductProtocol
)

var errAssemblyToolDomain = errors.New("the build-selected declared tool domain is unavailable")
var assemblyToolDomainID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
var assemblyToolSystemModule = regexp.MustCompile(`^[a-z0-9_.-]+\.dll$`)
var assemblyToolArtifactName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type assemblyToolFileRef struct {
	Role string `json:"role"`
	Bytes int64 `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type assemblyToolArtifactRef struct {
	Filename string `json:"filename"`
	Bytes int64 `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type assemblyToolPackageV1 struct {
	ID string `json:"id"`
	Version string `json:"version"`
	Distributable assemblyToolArtifactRef `json:"distributable"`
	RuntimeFiles []assemblyToolFileRef `json:"runtimeFiles"`
	Notices []assemblyToolFileRef `json:"notices"`
	CorrespondingSources []assemblyToolArtifactRef `json:"correspondingSources"`
	AuthenticationReceipt assemblyToolFileRef `json:"authenticationReceipt"`
}

type assemblyToolPEV1 struct {
	File assemblyToolFileRef `json:"file"`
	PackageID string `json:"packageId"`
	Machine int `json:"machine"`
	PrivateImports []string `json:"privateImports"`
	SystemImports []string `json:"systemImports"`
}

// These are DECLARED prerequisites. A pure constructor cannot observe a host,
// authenticate a system DLL or satisfy UCRT/WOW64/loader policy. The owner must
// independently establish current prerequisites before any native allocation.
type assemblyToolSystemABIV1 struct {
	OS string `json:"os"`
	HostArchitecture string `json:"hostArchitecture"`
	MinimumWindowsBuild uint64 `json:"minimumWindowsBuild"`
	CRTMode string `json:"crtMode"`
	RequiresWOW64 bool `json:"requiresWOW64"`
	SystemModules []string `json:"systemModules"`
	InspectionReceipt assemblyToolFileRef `json:"inspectionReceipt"`
}

type assemblyToolDomainRecordV1 struct {
	Schema int `json:"schema"`
	RecordType string `json:"recordType"`
	ProfileID string `json:"profileId"`
	Protocol string `json:"protocol"`
	ProducerSourceSHA256 string `json:"producerSourceSHA256"`
	CatalogSourceDescriptor assemblyToolFileRef `json:"catalogSourceDescriptor"`
	Packages []assemblyToolPackageV1 `json:"packages"`
	PEObjects []assemblyToolPEV1 `json:"peObjects"`
	SystemABI assemblyToolSystemABIV1 `json:"systemABI"`
	LicenceInventory assemblyToolFileRef `json:"licenceInventory"`
	SourceInventory assemblyToolFileRef `json:"sourceInventory"`
}

// There is deliberately no factory, registry entry or populated production
// value. A future reviewed application build selects every pin below. HTTP,
// package metadata and a manifest's own hash cannot supply this authority.
type applicationPinnedToolDomainV1 struct {
	// Private application choice only. Never decoded from the tool manifest or
	// inferred from its refs/hash. Omission preserves the exact legacy arm.
	ProducerFamily assemblyToolProducerFamily `json:"-"`
	ProfileID string
	ProfileSHA256 string
	ProducerSourceSHA256 string
	CatalogSourceSHA256 string
	Manifest assemblyToolFileRef
	SystemABISHA256 string
	AuthenticationReceipts [2]assemblyToolFileRef
	LicenceInventory assemblyToolFileRef
	SourceInventory assemblyToolFileRef
	InspectionReceipt assemblyToolFileRef
}

// Private declared data only: no catalog adoption, Verified bit, capability,
// fullInputClosure, native helper, path opener or cumulative guard-read counter.
type assemblyDebianToolDomainV1 struct {
	record assemblyToolDomainRecordV1
	canonical []byte
	digest string
}

func (d *assemblyDebianToolDomainV1) canonicalCopy() []byte {
	return append([]byte(nil), d.canonical...)
}

func (d *assemblyDebianToolDomainV1) declaredHostPrerequisites() assemblyToolSystemABIV1 {
	value := d.record.SystemABI
	value.SystemModules = append([]string(nil), value.SystemModules...)
	return value
}

func assemblyToolRefValid(ref assemblyToolFileRef) bool {
	return ref.Role != "" && len(ref.Role) <= 200 && ref.Bytes > 0 && ref.Bytes <= 16<<20 && assemblyToolPinnedHex(ref.SHA256)
}

func assemblyToolPinnedHex(value string) bool {
	return assemblyCatalogHex(value) && strings.Trim(value, "0") != ""
}

func assemblyToolArtifactValid(ref assemblyToolArtifactRef) bool {
	return len(ref.Filename) > 0 && len(ref.Filename) <= 200 && ref.Filename != "." && ref.Filename != ".." && assemblyToolArtifactName.MatchString(ref.Filename) && ref.Bytes > 0 && ref.Bytes <= 64<<20 && assemblyToolPinnedHex(ref.SHA256)
}

func assemblyToolExpectedValid(v applicationPinnedToolDomainV1) bool {
	if v.ProducerFamily != assemblyToolLegacyProducerFamily && v.ProducerFamily != assemblyToolProductProducerFamily { return false }
	if !assemblyToolDomainID.MatchString(v.ProfileID) || !assemblyToolPinnedHex(v.ProfileSHA256) || !assemblyToolPinnedHex(v.ProducerSourceSHA256) || !assemblyToolPinnedHex(v.CatalogSourceSHA256) || !assemblyToolPinnedHex(v.SystemABISHA256) ||
		v.Manifest.Role != assemblyToolDomainRole || v.Manifest.Bytes > assemblyToolDomainLimit {
		return false
	}
	refs := []assemblyToolFileRef{v.Manifest, v.AuthenticationReceipts[0], v.AuthenticationReceipts[1], v.LicenceInventory, v.SourceInventory, v.InspectionReceipt}
	seen := map[string]bool{}
	for _, ref := range refs {
		if !assemblyToolRefValid(ref) || !strings.HasPrefix(ref.Role, "support:") || seen[ref.Role] {
			return false
		}
		seen[ref.Role] = true
	}
	return true
}

// Duplicate keys are rejected at every object depth, before the closed DTO is
// decoded. The byte, depth and token bounds apply to data already read through
// the caller's original guard; this function performs no file or native IO.
func assemblyToolJSONUnique(data []byte) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	tokens := 0
	var read func(int) error
	read = func(depth int) error {
		if depth > 16 || tokens >= 32768 { return errAssemblyToolDomain }
		token, err := d.Token()
		if err != nil { return err }
		tokens++
		delim, compound := token.(json.Delim)
		if !compound { return nil }
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil { return err }
				tokens++
				name, ok := key.(string)
				if !ok || seen[name] { return errAssemblyToolDomain }
				seen[name] = true
				if err := read(depth+1); err != nil { return err }
			}
		case '[':
			for d.More() { if err := read(depth+1); err != nil { return err } }
		default: return errAssemblyToolDomain
		}
		end, err := d.Token()
		if err != nil || end != json.Delim(map[json.Delim]json.Delim{'{': '}', '[': ']'}[delim]) { return errAssemblyToolDomain }
		tokens++
		return nil
	}
	if err := read(0); err != nil { return err }
	if _, err := d.Token(); err != io.EOF { return errAssemblyToolDomain }
	return nil
}

func newAssemblyDebianToolDomainV1(data []byte, selected runtimeinstall.BundledDebianInputProofProfile, expected applicationPinnedToolDomainV1) (*assemblyDebianToolDomainV1, error) {
	if len(data) == 0 || len(data) > assemblyToolDomainLimit || !assemblyToolExpectedValid(expected) || selected.ID != expected.ProfileID || selected.SHA256 != expected.ProfileSHA256 || selected.SourceSHA256 != expected.ProducerSourceSHA256 {
		return nil, errAssemblyToolDomain
	}
	// Only the application's closed private choice selects a reviewer-owned
	// structural validator. Neither choice grants current host/native authority.
	var files map[string]runtimeinstall.DebianInputProofFile
	var err error
	profileObjectLimit := 256
	switch expected.ProducerFamily {
	case assemblyToolLegacyProducerFamily:
		files, err = validateDebianProducerProfile(selected)
	case assemblyToolProductProducerFamily:
		profileObjectLimit = assemblyToolProductProfileObjectLimit
		files, err = validateDebianProductProfile(selected)
	default:
		return nil, errAssemblyToolDomain
	}
	if err != nil { return nil, errAssemblyToolDomain }
	profileBytes, err := canonicalDebianProducerProfile(selected)
	if err != nil || len(profileBytes) > assemblyToolDomainLimit || len(selected.Files)+1 > profileObjectLimit { return nil, errAssemblyToolDomain }
	total := int64(len(profileBytes))
	for _, f := range selected.Files {
		if f.Bytes <= 0 || f.Bytes > 16<<20 || f.Bytes > (64<<20)-total { return nil, errAssemblyToolDomain }
		total += f.Bytes
	}
	match := func(ref assemblyToolFileRef) bool {
		f, ok := files[ref.Role]
		return assemblyToolRefValid(ref) && ok && f.Bytes == ref.Bytes && f.SHA256 == ref.SHA256
	}
	if !match(expected.Manifest) || expected.Manifest.Bytes != int64(len(data)) || assemblyCatalogDigest(data) != expected.Manifest.SHA256 { return nil, errAssemblyToolDomain }
	if assemblyToolJSONUnique(data) != nil { return nil, errAssemblyToolDomain }
	var value assemblyToolDomainRecordV1
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil { return nil, errAssemblyToolDomain }
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, data) || value.Schema != 1 || value.RecordType != "luci-bundled-debian-input-proof-tools-v1" || value.ProfileID != expected.ProfileID || value.Protocol != runtimeinstall.DebianInputProofProtocol || value.ProducerSourceSHA256 != expected.ProducerSourceSHA256 ||
		value.CatalogSourceDescriptor.Role != "catalog-source-descriptor" || value.CatalogSourceDescriptor.SHA256 != expected.CatalogSourceSHA256 || !match(value.CatalogSourceDescriptor) || len(value.Packages) != 2 || len(value.PEObjects) < 3 || len(value.PEObjects) > 128 {
		return nil, errAssemblyToolDomain
	}
	abi := value.SystemABI
	abiBytes, err := json.Marshal(abi)
	if err != nil || assemblyCatalogDigest(abiBytes) != expected.SystemABISHA256 || abi.OS != "windows" || abi.HostArchitecture != "amd64" || abi.MinimumWindowsBuild == 0 || abi.MinimumWindowsBuild > 4294967295 || abi.CRTMode != "supported-system-ucrt" || abi.InspectionReceipt != expected.InspectionReceipt || !match(abi.InspectionReceipt) ||
		value.LicenceInventory != expected.LicenceInventory || !match(value.LicenceInventory) || value.SourceInventory != expected.SourceInventory || !match(value.SourceInventory) || len(abi.SystemModules) == 0 || len(abi.SystemModules) > 128 {
		return nil, errAssemblyToolDomain
	}
	systems := map[string]bool{}
	previous := ""
	for _, name := range abi.SystemModules {
		if name <= previous || len(name) > 128 || !assemblyToolSystemModule.MatchString(name) { return nil, errAssemblyToolDomain }
		systems[name], previous = true, name
	}
	used := map[string]bool{expected.Manifest.Role: true, value.CatalogSourceDescriptor.Role: true, value.LicenceInventory.Role: true, value.SourceInventory.Role: true, abi.InspectionReceipt.Role: true}
	runtimePackage := map[string]string{}
	for i, pkg := range value.Packages {
		id, version, wrapper := "cpython", "3.13.16", (assemblyToolArtifactRef{"python-3.13.16-embed-amd64.zip", 11439094, "97dae5274cc54867065e8d5a3226e48c35017ed332a0fdb0e27d5b5821961297"})
		if i == 1 { id, version, wrapper = "gnupg", "2.5.24_20260923", (assemblyToolArtifactRef{"gnupg-w32-2.5.24_20260923.exe", 5804864, "ea6f9dee2cccd83287432625b8b399173805add81d27067fcaf019c0c4da1ad8"}) }
		if pkg.ID != id || pkg.Version != version || pkg.Distributable != wrapper || pkg.AuthenticationReceipt != expected.AuthenticationReceipts[i] || !match(pkg.AuthenticationReceipt) || used[pkg.AuthenticationReceipt.Role] || len(pkg.RuntimeFiles) == 0 || len(pkg.RuntimeFiles) > 230 || len(pkg.Notices) == 0 || len(pkg.Notices) > 32 || len(pkg.CorrespondingSources) == 0 || len(pkg.CorrespondingSources) > 64 {
			return nil, errAssemblyToolDomain
		}
		used[pkg.AuthenticationReceipt.Role] = true
		previous = ""
		for _, ref := range pkg.RuntimeFiles {
			f := files[ref.Role]
			if ref.Role <= previous || !match(ref) || used[ref.Role] || ref.Role != "cp-python" && ref.Role != "gpg" && ref.Role != "gpgv" && !strings.HasPrefix(ref.Role, "support:") || !strings.HasPrefix(f.RelativePath, "tools/"+id+"/") { return nil, errAssemblyToolDomain }
			used[ref.Role], runtimePackage[ref.Role], previous = true, id, ref.Role
		}
		previous = ""
		for _, ref := range pkg.Notices {
			if ref.Role <= previous || !match(ref) || used[ref.Role] || !strings.HasPrefix(ref.Role, "support:") { return nil, errAssemblyToolDomain }
			used[ref.Role], previous = true, ref.Role
		}
		previous = ""
		for _, ref := range pkg.CorrespondingSources {
			if !assemblyToolArtifactValid(ref) || ref.Filename <= previous { return nil, errAssemblyToolDomain }
			previous = ref.Filename
		}
		// Metadata references, not guard-opened archives or authenticated source.
		foundSource := false
		for _, ref := range pkg.CorrespondingSources {
			if i == 0 && ref == (assemblyToolArtifactRef{"Python-3.13.16.tar.xz", 23225704, "f4b1bfb3c79b5bb11b8d228a12504163b4c0dab4d679828d8f5f26b6cb6ab35d"}) { foundSource = true }
			if i == 1 && ref.Filename == "gnupg-w32-2.5.24_20260923.tar.xz" && ref.Bytes == 15295680 { foundSource = true }
		}
		if !foundSource { return nil, errAssemblyToolDomain }
	}
	if runtimePackage["cp-python"] != "cpython" || runtimePackage["gpg"] != "gnupg" || runtimePackage["gpgv"] != "gnupg" { return nil, errAssemblyToolDomain }
	pe := map[string]assemblyToolPEV1{}
	previous = ""
	needsWOW64 := false
	for _, item := range value.PEObjects {
		ext := strings.ToLower(filepath.Ext(files[item.File.Role].RelativePath))
		if item.File.Role <= previous || !match(item.File) || runtimePackage[item.File.Role] != item.PackageID || ext != ".exe" && ext != ".dll" && ext != ".pyd" || item.Machine != 332 && item.Machine != 34404 || item.PackageID == "cpython" && item.Machine != 34404 || item.PrivateImports == nil || item.SystemImports == nil || len(item.PrivateImports) > 128 || len(item.SystemImports) > 128 {
			return nil, errAssemblyToolDomain
		}
		previous, pe[item.File.Role] = item.File.Role, item
		needsWOW64 = needsWOW64 || item.Machine == 332
	}
	if needsWOW64 != abi.RequiresWOW64 { return nil, errAssemblyToolDomain }
	for role := range runtimePackage {
		ext := strings.ToLower(filepath.Ext(files[role].RelativePath))
		if ext == ".exe" || ext == ".dll" || ext == ".pyd" { if _, ok := pe[role]; !ok { return nil, errAssemblyToolDomain } }
	}
	for _, role := range []string{"cp-python", "gpg", "gpgv"} { if _, ok := pe[role]; !ok { return nil, errAssemblyToolDomain } }
	for _, item := range value.PEObjects {
		previous = ""
		basenames := map[string]bool{}
		for _, role := range item.PrivateImports {
			dependency, ok := pe[role]
			base := strings.ToLower(filepath.Base(files[role].RelativePath))
			if role <= previous || !ok || dependency.Machine != item.Machine || dependency.PackageID != item.PackageID || basenames[base] { return nil, errAssemblyToolDomain }
			basenames[base], previous = true, role
		}
		previous = ""
		for _, name := range item.SystemImports {
			if name <= previous || !systems[name] || basenames[name] { return nil, errAssemblyToolDomain }
			previous = name
		}
	}
	// Check each executable/module graph separately, including declared dynamic
	// Python module roots. Do not conflate independent x64 Python and x86 GPG
	// processes. Transitive basename aliases within one graph are ambiguous.
	for root := range pe {
		visited, basenames, systemImports := map[string]bool{}, map[string]string{}, map[string]bool{}
		var walk func(string) bool
		walk = func(role string) bool {
			if visited[role] { return true }
			visited[role] = true
			item := pe[role]
			base := strings.ToLower(filepath.Base(files[role].RelativePath))
			if systemImports[base] { return false }
			if other, exists := basenames[base]; exists && other != role { return false }
			basenames[base] = role
			for _, dependency := range item.PrivateImports { if !walk(dependency) { return false } }
			for _, name := range item.SystemImports {
				if _, exists := basenames[name]; exists { return false }
				systemImports[name] = true
			}
			return true
		}
		if !walk(root) { return nil, errAssemblyToolDomain }
	}
	// All optional files are explicitly inventoried; input roles are accounted
	// by the existing descriptor validator, not reclassified as tool proof.
	for role := range files { if strings.HasPrefix(role, "support:") && !used[role] { return nil, errAssemblyToolDomain } }
	// JSON decoding and Marshal allocate fresh slices. No input data, selected
	// Files slice/map or mutable expected collection is retained in the result.
	return &assemblyDebianToolDomainV1{record: value, canonical: append([]byte(nil), canonical...), digest: assemblyCatalogDigest(canonical)}, nil
}
