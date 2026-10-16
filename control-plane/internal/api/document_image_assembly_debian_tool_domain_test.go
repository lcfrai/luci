package api

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// Synthetic declared data only. No upstream archive, signature, PE file, host
// observation, filesystem guard, licence authentication or runtime is proved.
type assemblyToolDomainFixture struct {
	value assemblyToolDomainRecordV1
	profile runtimeinstall.BundledDebianInputProofProfile
	expected applicationPinnedToolDomainV1
}

func newAssemblyToolDomainFixture(t *testing.T, mixed bool) assemblyToolDomainFixture {
	t.Helper()
	f := assemblyToolDomainFixture{}
	f.profile = runtimeinstall.BundledDebianInputProofProfile{ID: "synthetic-tools-v1", Protocol: runtimeinstall.DebianInputProofProtocol, Root: filepath.Join(t.TempDir(), "synthetic-private-bundle")}
	add := func(role, path string) assemblyToolFileRef {
		ref := assemblyToolFileRef{role, 1, assemblyCatalogDigest([]byte("synthetic:"+role))}
		// Declared metadata pins satisfy the structural profile validator only.
		// Bytes remain synthetic: no SHA preimage, real tool or native eligibility
		// is asserted. The product caller separately rejects this legacy family.
		switch role {
		case "projection-source": ref.SHA256 = assemblyDebianProjectionSHA
		case "producer-config": ref.SHA256 = assemblyDebianConfigSHA
		case "gpg": ref.SHA256 = "a5140c85353e8399da8d6bd7e3741524cd76a4e69be88af12042b1c9ef022984"
		case "gpgv": ref.SHA256 = "f4d13204d77fdf63c02b0e6742230f83a833128c28f7b715709c2c63a96c427b"
		}
		f.profile.Files = append(f.profile.Files, runtimeinstall.DebianInputProofFile{Role: role, RelativePath: path, Bytes: ref.Bytes, SHA256: ref.SHA256})
		return ref
	}
	required := []string{"projection-entry", "projection-source", "producer-source-receipt", "producer-config", "policy-source", "dependency-lock", "acquisition-manifest", "snapshot-source", "catalog-source-descriptor", "public-key-0", "public-key-1", "public-key-2", "signed-release-0", "signed-release-1", "signed-release-2", "signed-release-3", "signed-release-4"}
	for _, role := range required { add(role, "inputs/"+role+".json") }
	python := add("cp-python", "tools/cpython/python.exe")
	gpg := add("gpg", "tools/gnupg/bin/gpg.exe")
	gpgv := add("gpgv", "tools/gnupg/bin/gpgv.exe")
	pythonDLL := add("support:tools/cpython/python313.dll", "tools/cpython/python313.dll")
	pythonZIP := add("support:tools/cpython/python313.zip", "tools/cpython/python313.zip")
	gpgDLL := add("support:tools/gnupg/bin/libgcrypt-20.dll", "tools/gnupg/bin/libgcrypt-20.dll")
	pythonNotice := add("support:licences/cpython/LICENSE.txt", "licences/cpython/LICENSE.txt")
	gpgNotice := add("support:licences/gnupg/COPYING", "licences/gnupg/COPYING")
	authPython := add("support:receipts/cpython-authentication-v1.json", "receipts/cpython-authentication-v1.json")
	authGPG := add("support:receipts/gnupg-authentication-v1.json", "receipts/gnupg-authentication-v1.json")
	inspection := add("support:receipts/windows-input-proof-inspection-v1.json", "receipts/windows-input-proof-inspection-v1.json")
	licences := add("support:manifests/windows-input-proof-licences-v1.json", "manifests/windows-input-proof-licences-v1.json")
	sources := add("support:manifests/windows-input-proof-sources-v1.json", "manifests/windows-input-proof-sources-v1.json")
	manifest := add(assemblyToolDomainRole, "manifests/windows-input-proof-tool-closure-v1.json")
	ref := func(role string) assemblyToolFileRef {
		for _, v := range f.profile.Files { if v.Role == role { return assemblyToolFileRef{v.Role, v.Bytes, v.SHA256} } }
		t.Fatal("missing synthetic role", role)
		return assemblyToolFileRef{}
	}
	f.profile.SourceSHA256 = ref("producer-source-receipt").SHA256
	f.value = assemblyToolDomainRecordV1{
		Schema: 1, RecordType: "luci-bundled-debian-input-proof-tools-v1", ProfileID: f.profile.ID, Protocol: f.profile.Protocol,
		ProducerSourceSHA256: f.profile.SourceSHA256, CatalogSourceDescriptor: ref("catalog-source-descriptor"),
		Packages: []assemblyToolPackageV1{
			{ID: "cpython", Version: "3.13.16", Distributable: assemblyToolArtifactRef{"python-3.13.16-embed-amd64.zip", 11439094, "97dae5274cc54867065e8d5a3226e48c35017ed332a0fdb0e27d5b5821961297"}, RuntimeFiles: []assemblyToolFileRef{python, pythonDLL, pythonZIP}, Notices: []assemblyToolFileRef{pythonNotice}, CorrespondingSources: []assemblyToolArtifactRef{{"Python-3.13.16.tar.xz", 23225704, "f4b1bfb3c79b5bb11b8d228a12504163b4c0dab4d679828d8f5f26b6cb6ab35d"}}, AuthenticationReceipt: authPython},
			{ID: "gnupg", Version: "2.5.24_20260923", Distributable: assemblyToolArtifactRef{"gnupg-w32-2.5.24_20260923.exe", 5804864, "ea6f9dee2cccd83287432625b8b399173805add81d27067fcaf019c0c4da1ad8"}, RuntimeFiles: []assemblyToolFileRef{gpg, gpgv, gpgDLL}, Notices: []assemblyToolFileRef{gpgNotice}, CorrespondingSources: []assemblyToolArtifactRef{{"gnupg-w32-2.5.24_20260923.tar.xz", 15295680, assemblyCatalogDigest([]byte("synthetic-only-unknown-upstream-source"))}}, AuthenticationReceipt: authGPG},
		},
		PEObjects: []assemblyToolPEV1{
			{python, "cpython", 34404, []string{pythonDLL.Role}, []string{"kernel32.dll"}},
			{gpg, "gnupg", 34404, []string{gpgDLL.Role}, []string{"kernel32.dll"}},
			{gpgv, "gnupg", 34404, []string{gpgDLL.Role}, []string{"kernel32.dll"}},
			{pythonDLL, "cpython", 34404, []string{}, []string{"ucrtbase.dll"}},
			{gpgDLL, "gnupg", 34404, []string{}, []string{"kernel32.dll"}},
		},
		SystemABI: assemblyToolSystemABIV1{OS: "windows", HostArchitecture: "amd64", MinimumWindowsBuild: 19041, CRTMode: "supported-system-ucrt", RequiresWOW64: mixed, SystemModules: []string{"kernel32.dll", "ucrtbase.dll"}, InspectionReceipt: inspection},
		LicenceInventory: licences, SourceInventory: sources,
	}
	if mixed { for i := range f.value.PEObjects { if f.value.PEObjects[i].PackageID == "gnupg" { f.value.PEObjects[i].Machine = 332 } } }
	f.expected = applicationPinnedToolDomainV1{ProfileID: f.profile.ID, ProducerSourceSHA256: f.profile.SourceSHA256, CatalogSourceSHA256: f.value.CatalogSourceDescriptor.SHA256, Manifest: manifest, AuthenticationReceipts: [2]assemblyToolFileRef{authPython, authGPG}, LicenceInventory: licences, SourceInventory: sources, InspectionReceipt: inspection}
	return f
}

func (f *assemblyToolDomainFixture) seal(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(f.value)
	if err != nil { t.Fatal(err) }
	return f.sealBytes(t, data)
}

func (f *assemblyToolDomainFixture) sealBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	f.expected.Manifest.Bytes, f.expected.Manifest.SHA256 = int64(len(data)), assemblyCatalogDigest(data)
	for i := range f.profile.Files { if f.profile.Files[i].Role == assemblyToolDomainRole { f.profile.Files[i].Bytes, f.profile.Files[i].SHA256 = f.expected.Manifest.Bytes, f.expected.Manifest.SHA256 } }
	sort.Slice(f.profile.Files, func(i, j int) bool { return f.profile.Files[i].Role < f.profile.Files[j].Role })
	canonical, err := canonicalDebianProducerProfile(f.profile)
	if err != nil { t.Fatal(err) }
	f.profile.SHA256 = assemblyCatalogDigest(canonical)
	f.expected.ProfileSHA256 = f.profile.SHA256
	abi, err := json.Marshal(f.value.SystemABI)
	if err != nil { t.Fatal(err) }
	f.expected.SystemABISHA256 = assemblyCatalogDigest(abi)
	return data
}

func TestAssemblyToolDomainDeclaredProfilesAndHostPrerequisitesStayDataOnly(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		f := newAssemblyToolDomainFixture(t, mixed)
		data := f.seal(t)
		domain, err := newAssemblyDebianToolDomainV1(data, f.profile, f.expected)
		if err != nil { t.Fatal("complete synthetic declared profile refused", mixed, err) }
		if domain.digest != assemblyCatalogDigest(data) || domain.declaredHostPrerequisites().RequiresWOW64 != mixed { t.Fatal("declared prerequisites changed") }
		// No host query, guard or capability is present; return value only retains
		// declared prerequisites. A future caller still needs current host proof.
	}
}

func TestAssemblyToolDomainMissingExpectedPinsNeverSelfAuthorize(t *testing.T) {
	for _, field := range []string{"profile", "source", "catalog-source", "manifest", "abi", "auth", "licence", "source-inventory", "inspection"} {
		t.Run(field, func(t *testing.T) {
			f := newAssemblyToolDomainFixture(t, false)
			data := f.seal(t)
			zero := strings.Repeat("0", 64)
			switch field {
			case "profile": f.expected.ProfileSHA256 = zero
			case "source": f.expected.ProducerSourceSHA256 = zero
			case "catalog-source": f.expected.CatalogSourceSHA256 = zero
			case "manifest": f.expected.Manifest.SHA256 = zero
			case "abi": f.expected.SystemABISHA256 = zero
			case "auth": f.expected.AuthenticationReceipts[0] = assemblyToolFileRef{}
			case "licence": f.expected.LicenceInventory = assemblyToolFileRef{}
			case "source-inventory": f.expected.SourceInventory = assemblyToolFileRef{}
			case "inspection": f.expected.InspectionReceipt = assemblyToolFileRef{}
			}
			if _, err := newAssemblyDebianToolDomainV1(data, f.profile, f.expected); err == nil { t.Fatal("manifest authorized its own missing application pin") }
		})
	}
}

func TestAssemblyToolDomainStrictDecoderRejectsDuplicatesUnknownAndNoncanonical(t *testing.T) {
	for _, mutation := range []string{"duplicate", "nested-duplicate", "unknown", "trailing", "whitespace", "null-array", "oversize"} {
		t.Run(mutation, func(t *testing.T) {
			f := newAssemblyToolDomainFixture(t, false)
			data := f.seal(t)
			s := string(data)
			switch mutation {
			case "duplicate": s = strings.Replace(s, `"schema":1`, `"schema":1,"schema":1`, 1)
			case "nested-duplicate": s = strings.Replace(s, `"machine":34404`, `"machine":34404,"machine":34404`, 1)
			case "unknown": s = strings.Replace(s, `"schema":1`, `"schema":1,"verified":true`, 1)
			case "trailing": s += `{}`
			case "whitespace": s = " " + s
			case "null-array": s = strings.Replace(s, `"privateImports":[]`, `"privateImports":null`, 1)
			case "oversize": s += strings.Repeat(" ", assemblyToolDomainLimit)
			}
			data = f.sealBytes(t, []byte(s))
			if _, err := newAssemblyDebianToolDomainV1(data, f.profile, f.expected); err == nil { t.Fatal("closed/canonical decoder accepted", mutation) }
		})
	}
}

func TestAssemblyToolDomainRejectsChangedPackagesInputsAndDeclaredClosure(t *testing.T) {
	cases := map[string]func(*assemblyToolDomainFixture){
		"version": func(f *assemblyToolDomainFixture) { f.value.Packages[1].Version = "2.4.9" },
		"wrapper": func(f *assemblyToolDomainFixture) { f.value.Packages[0].Distributable.SHA256 = strings.Repeat("a", 64) },
		"schema": func(f *assemblyToolDomainFixture) { f.value.Schema = 2 },
		"kind": func(f *assemblyToolDomainFixture) { f.value.RecordType = "verified-full-input-closure" },
		"source": func(f *assemblyToolDomainFixture) { f.value.ProducerSourceSHA256 = strings.Repeat("b", 64) },
		"catalog-source": func(f *assemblyToolDomainFixture) { f.value.CatalogSourceDescriptor.Role = "producer-source-receipt" },
		"auth": func(f *assemblyToolDomainFixture) { f.value.Packages[0].AuthenticationReceipt = f.value.Packages[1].AuthenticationReceipt },
		"licence": func(f *assemblyToolDomainFixture) { f.value.Packages[1].Notices = nil },
		"source-inventory": func(f *assemblyToolDomainFixture) { f.value.SourceInventory = f.value.LicenceInventory },
		"corresponding-source": func(f *assemblyToolDomainFixture) { f.value.Packages[1].CorrespondingSources[0].Filename = "gnupg-2.5.24.tar.bz2" },
		"file-pin": func(f *assemblyToolDomainFixture) { f.value.Packages[0].RuntimeFiles[0].SHA256 = strings.Repeat("c", 64) },
		"duplicate-runtime": func(f *assemblyToolDomainFixture) { f.value.Packages[0].RuntimeFiles = append(f.value.Packages[0].RuntimeFiles, f.value.Packages[0].RuntimeFiles[0]) },
		"missing-pe": func(f *assemblyToolDomainFixture) { f.value.PEObjects = f.value.PEObjects[:4] },
		"wrong-package": func(f *assemblyToolDomainFixture) { f.value.PEObjects[0].PackageID = "gnupg" },
		"unresolved-private-import": func(f *assemblyToolDomainFixture) { f.value.PEObjects[0].PrivateImports = []string{"support:tools/cpython/missing.dll"} },
		"ambient-system-import": func(f *assemblyToolDomainFixture) { f.value.PEObjects[0].SystemImports = []string{"unknown-path.dll"} },
		"cross-package-import": func(f *assemblyToolDomainFixture) { f.value.PEObjects[0].PrivateImports = []string{f.value.PEObjects[4].File.Role} },
		"nil-prerequisite": func(f *assemblyToolDomainFixture) { f.value.SystemABI.MinimumWindowsBuild = 0 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newAssemblyToolDomainFixture(t, false)
			mutate(&f)
			data := f.seal(t) // Re-pin synthetic data to reach semantic validation.
			if _, err := newAssemblyDebianToolDomainV1(data, f.profile, f.expected); err == nil { t.Fatal("changed declared closure accepted") }
		})
	}
}

func TestAssemblyToolDomainPerProcessABIRejectsCrossEdgesAndMissingWOW64(t *testing.T) {
	for _, change := range []string{"private-edge", "python-x86", "wow64-undeclared"} {
		t.Run(change, func(t *testing.T) {
			f := newAssemblyToolDomainFixture(t, true)
			switch change {
			case "private-edge": f.value.PEObjects[4].Machine = 34404
			case "python-x86": f.value.PEObjects[0].Machine = 332
			case "wow64-undeclared": f.value.SystemABI.RequiresWOW64 = false
			}
			data := f.seal(t)
			if _, err := newAssemblyDebianToolDomainV1(data, f.profile, f.expected); err == nil { t.Fatal("cross-ABI edge/unselected WOW64 declaration accepted") }
		})
	}
}

func TestAssemblyToolDomainStaticBoundsCountDescriptorAndEveryDeclaredFile(t *testing.T) {
	for _, change := range []string{"file", "total", "objects", "extra-support"} {
		t.Run(change, func(t *testing.T) {
			f := newAssemblyToolDomainFixture(t, false)
			switch change {
			case "file": f.profile.Files[0].Bytes = 16<<20 + 1
			case "total":
				for i := 0; i < 4; i++ { f.profile.Files[i].Bytes = 16<<20 }
			case "objects":
				for i := len(f.profile.Files); i < 256; i++ { f.profile.Files = append(f.profile.Files, runtimeinstall.DebianInputProofFile{Role: "support:extra/file-"+strconv.Itoa(i)+".json", RelativePath: "extra/file-"+strconv.Itoa(i)+".json", Bytes: 1, SHA256: strings.Repeat("d", 64)}) }
			case "extra-support": f.profile.Files = append(f.profile.Files, runtimeinstall.DebianInputProofFile{Role: "support:extra/unused.json", RelativePath: "extra/unused.json", Bytes: 1, SHA256: strings.Repeat("d", 64)})
			}
			data := f.seal(t)
			if _, err := newAssemblyDebianToolDomainV1(data, f.profile, f.expected); err == nil { t.Fatal("static declared bound/inventory accepted") }
		})
	}
	// This proves no handle or cumulative-read enforcement: those counters and
	// originals belong to the existing guard/caller, not this pure constructor.
}

func TestAssemblyToolDomainReturnedDataDoesNotAliasMutableCallerBuffers(t *testing.T) {
	f := newAssemblyToolDomainFixture(t, true)
	data := f.seal(t)
	domain, err := newAssemblyDebianToolDomainV1(data, f.profile, f.expected)
	if err != nil { t.Fatal(err) }
	digest := assemblyCatalogDigest(domain.canonicalCopy())
	data[0] = '!'
	f.profile.Files[0].RelativePath = "mutated/after-construction"
	f.value.SystemABI.SystemModules[0] = "mutated.dll"
	copyBytes := domain.canonicalCopy()
	copyBytes[0] = '!'
	prerequisites := domain.declaredHostPrerequisites()
	prerequisites.SystemModules[0] = "mutated-again.dll"
	if assemblyCatalogDigest(domain.canonicalCopy()) != digest || domain.declaredHostPrerequisites().SystemModules[0] != "kernel32.dll" { t.Fatal("private output retained mutable caller aliases") }
}

func TestAssemblyToolDomainImmutableSelectedEntryConfigAndSourcePins(t *testing.T) {
	for _, role := range []string{"projection-entry", "producer-config", "producer-source-receipt", "catalog-source-descriptor"} {
		t.Run(role, func(t *testing.T) {
			f := newAssemblyToolDomainFixture(t, false)
			data := f.seal(t)
			for i := range f.profile.Files { if f.profile.Files[i].Role == role { f.profile.Files[i].SHA256 = strings.Repeat("e", 64) } }
			if _, err := newAssemblyDebianToolDomainV1(data, f.profile, f.expected); err == nil { t.Fatal("changed selected input retained application approval") }
		})
	}
}

func TestAssemblyToolDomainTransitiveBasenameAliasCannotHideBehindSeparateImports(t *testing.T) {
	f := newAssemblyToolDomainFixture(t, false)
	add := func(path string) assemblyToolFileRef {
		role := "support:"+path
		ref := assemblyToolFileRef{role, 1, assemblyCatalogDigest([]byte("synthetic:"+path))}
		f.profile.Files = append(f.profile.Files, runtimeinstall.DebianInputProofFile{Role: role, RelativePath: path, Bytes: ref.Bytes, SHA256: ref.SHA256})
		f.value.Packages[1].RuntimeFiles = append(f.value.Packages[1].RuntimeFiles, ref)
		return ref
	}
	alias := add("tools/gnupg/other/libgcrypt-20.dll")
	shim := add("tools/gnupg/shim.dll")
	f.value.PEObjects[1].PrivateImports = append(f.value.PEObjects[1].PrivateImports, shim.Role)
	f.value.PEObjects = append(f.value.PEObjects,
		assemblyToolPEV1{alias, "gnupg", 34404, []string{}, []string{}},
		assemblyToolPEV1{shim, "gnupg", 34404, []string{alias.Role}, []string{}},
	)
	sort.Slice(f.value.Packages[1].RuntimeFiles, func(i,j int) bool { return f.value.Packages[1].RuntimeFiles[i].Role < f.value.Packages[1].RuntimeFiles[j].Role })
	sort.Slice(f.value.PEObjects, func(i,j int) bool { return f.value.PEObjects[i].File.Role < f.value.PEObjects[j].File.Role })
	data := f.seal(t)
	if _, err := newAssemblyDebianToolDomainV1(data, f.profile, f.expected); err == nil { t.Fatal("one process graph accepted two private DLLs with the same basename") }
}
