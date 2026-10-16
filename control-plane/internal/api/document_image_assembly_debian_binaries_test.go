package api

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestAssemblyDebianBinaryNormalizePinnedSelection(t *testing.T) {
	source, _ := assemblyDebianSourceFixture(t)
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("testdata", "document-assembly-debian", "capture-v4", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	lock, manifest := read("dependency-lock.fixture"), read("acquisition-manifest.fixture")
	selection, err := normalizeAssemblyDebianBinarySelection(lock, manifest, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.Packages) != 174 {
		t.Fatal("partial scope")
	}
	maximum, total := int64(0), int64(0)
	for _, p := range selection.Packages {
		total += p.Bytes
		if p.Bytes > maximum {
			maximum = p.Bytes
		}
	}
	if total != 144257468 || maximum != 56547048 {
		t.Fatal("retained input profile changed", total, maximum)
	}
	manifest[0] ^= 1
	if _, err = normalizeAssemblyDebianBinarySelection(lock, manifest, source); err == nil {
		t.Fatal("unpinned bytes accepted")
	}
}

func TestAssemblyDebianBinaryPhysicalFieldsPreserveOpaqueValues(t *testing.T) {
	fields, err := parseAssemblyBinaryParagraph([]byte("pAcKaGe: A\r\nVersion: 1:2+BinNMU1\r\nDepends: LibA:any (>= 1) |\r\n\t LibB (= 2) \t\r\n"))
	if err != nil || fields["version"] != "1:2+BinNMU1" || fields["depends"] != "LibA:any (>= 1) | LibB (= 2)" {
		t.Fatal(fields, err)
	}
	for _, raw := range []string{"Package: A\npackage: B\n", " orphan\n", "Package: A\rB\n", "X: " + strings.Repeat("x", 8193) + "\n", "Package: A\n\nPackage: B\n", "X: \xff\n"} {
		if _, err := parseAssemblyBinaryParagraph([]byte(raw)); err == nil {
			t.Fatal("hostile record accepted")
		}
	}
}

func assemblyBinaryFrameBytes(ordinal, kind, slot int, raw []byte) []byte {
	out := make([]byte, 40+len(raw))
	binary.LittleEndian.PutUint16(out, uint16(ordinal))
	out[2], out[3] = byte(kind), byte(slot)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(raw)))
	copy(out[8:40], assemblyBinaryDigestBytes(assemblyCatalogDigest(raw)))
	copy(out[40:], raw)
	return out
}

// Entirely synthetic observations. .deb/index object refs are metadata, not
// SHA preimages, actual signatures, owned reads or a publishable capability.
func assemblyBinaryCoverageFixture(t *testing.T) (assemblyDebianBinarySelectionV1, assemblyDebianSignedIndexBindingsV1, assemblyDebianBinariesV1, []assemblyDebianBinaryPartV1, []assemblyDebianBinaryBundle) {
	t.Helper()
	indexes, _, _, _, _ := assemblyIndexMetadataFixture(t)
	indexRef, err := assemblyDebianRecordRef("signed-index-bindings", indexes)
	if err != nil {
		t.Fatal(err)
	}
	s := assemblyDebianBinarySelectionV1{Schema: 1, DependencyLockSHA256: assemblyDebianLockSHA, AcquisitionManifestSHA256: assemblyDebianManifestSHA, CandidatePolicySHA256: assemblyDebianPolicySourceSHA}
	ids := []string{"current/bookworm", "current/bookworm-security", "current/bookworm-updates", "historical-base-anchor/bookworm", "historical-base-anchor/bookworm-security", "historical-base-anchor/bookworm-updates"}
	sort.Strings(ids)
	base := assemblyDebianBinaryBytes / 174
	for n := 0; n < 174; n++ {
		size := base
		if n == 173 {
			size = assemblyDebianBinaryBytes - base*173
		}
		s.Packages = append(s.Packages, assemblyDebianBinaryExpectedTupleV1{n, fmt.Sprintf("synthetic%d", n), "1:2+BinNMU1", "all", fmt.Sprintf("pool/main/s/synthetic/p%d.deb", n), size, assemblyCatalogDigest([]byte(fmt.Sprint("synthetic-deb", n))), append([]string(nil), ids...)})
	}
	root := assemblyDebianBinariesV1{Schema: 1, RecordType: "luci-debian-binary-bindings-v1", ObservationState: "complete-observations-not-adopted", SignedIndexes: indexRef, DependencyLockSHA256: assemblyDebianLockSHA, AcquisitionManifestSHA256: assemblyDebianManifestSHA, Producer: assemblyDebianObjectRef{"binary-binding-receipt", assemblyCatalogDigest([]byte("synthetic-receipt")), 1}, RetainedManifest: assemblyCatalogRef{1, "evidence-index", assemblyCatalogDigest([]byte("synthetic-index")), 1}, PackageCount: 174, PackageBytes: assemblyDebianBinaryBytes, SourceArchiveBinding: "not-evaluated", DependencyClosure: "not-evaluated", ExpectedInstalledFiles: "not-derived", ObservedInstalledOutput: "not-observed", ServerEligibility: "not-evaluated-server-owned"}
	unique := map[string]assemblyDebianObjectRef{}
	for _, b := range indexes.Bindings {
		if b.IndexPath == "main/binary-amd64/Packages.xz" {
			unique[b.CompressedObject.SHA256] = b.CompressedObject
		}
	}
	for _, object := range unique {
		root.DecodedIndexes = append(root.DecodedIndexes, assemblyDebianBinaryDecodedIndexV1{object, assemblyCatalogDigest([]byte("synthetic-decoded-" + object.SHA256)), 1 << 20})
	}
	sort.Slice(root.DecodedIndexes, func(i, j int) bool {
		return root.DecodedIndexes[i].Compressed.SHA256 < root.DecodedIndexes[j].Compressed.SHA256
	})
	root.Counters = assemblyDebianBinaryCountersV1{2*(assemblyDebianBinaryBytes+assemblyDebianBinaryIndexBytes) + 1000, 10000, 5 << 20, 1000}
	d := root.DecodedIndexes[0]
	ordinals := []int{}
	for n, b := range indexes.Bindings {
		if b.IndexPath == "main/binary-amd64/Packages.xz" && b.CompressedObject == d.Compressed {
			ordinals = append(ordinals, n)
		}
	}
	parts, proofs := []assemblyDebianBinaryPartV1{}, []assemblyDebianBinaryBundle{}
	for partOrdinal := 0; partOrdinal < 11; partOrdinal++ {
		count := 16
		if partOrdinal == 10 {
			count = 14
		}
		raw := []byte("LUCI-DBB1\n")
		for i := 0; i < count; i++ {
			p := s.Packages[partOrdinal*16+i]
			paragraph := []byte(fmt.Sprintf("Package: %s\nVersion: %s\nArchitecture: %s\nSize: %d\nSHA256: %s\nFilename: %s\nDepends: LibA:any (>= 1) | LibB\nSource: sourceA (2)\n", p.Name, p.Version, p.Architecture, p.Bytes, p.SHA256, p.Filename))
			control := []byte(fmt.Sprintf("Package: %s\nVersion: %s\nArchitecture: %s\nDepends: LibA:any (>= 1) | LibB\nSource: sourceB (3)\n", p.Name, p.Version, p.Architecture))
			pf, _ := parseAssemblyBinaryParagraph(paragraph)
			cf, _ := parseAssemblyBinaryParagraph(control)
			claims := assemblyDebianBinarySelectedFieldsV1{1, assemblyBinarySelectedFields(cf), []assemblyDebianBinaryIndexFieldsV1{{append([]int(nil), ordinals...), assemblyBinarySelectedFields(pf)}}}
			selected, _ := json.Marshal(claims)
			for kind, payload := range [][]byte{paragraph, control, selected} {
				raw = append(raw, assemblyBinaryFrameBytes(p.Ordinal, kind+1, 0, payload)...)
			}
		}
		object := assemblyDebianObjectRef{"binary-binding-bundle", assemblyCatalogDigest(raw), int64(len(raw))}
		proof, err := decodeAssemblyDebianBinaryBundle(raw, object, partOrdinal)
		if err != nil {
			t.Fatal(err)
		}
		proofs = append(proofs, proof)
		part := assemblyDebianBinaryPartV1{Schema: 1, RecordType: "luci-debian-binary-binding-part-v1", OperationID: "synthetic-observation", Part: partOrdinal, SignedIndexes: indexRef, DependencyLockSHA256: assemblyDebianLockSHA, FirstLockOrdinal: partOrdinal * 16}
		for i := 0; i < count; i++ {
			p := s.Packages[partOrdinal*16+i]
			frames := proof.frames[i*3 : i*3+3]
			part.Bindings = append(part.Bindings, assemblyDebianBinaryBindingV1{p.Ordinal, p.Name, p.Version, p.Architecture, p.Filename, assemblyDebianObjectRef{"deb-package", p.SHA256, p.Bytes}, []assemblyDebianBinaryMatchV1{{append([]int(nil), ordinals...), d.Compressed, d.DecodedSHA256, d.DecodedBytes, int64(p.Ordinal * 1024), frames[0].span}}, frames[1].span, frames[2].span})
		}
		parts = append(parts, part)
		body, _ := json.Marshal(part)
		root.Parts = append(root.Parts, assemblyDebianBinaryPartRefV1{partOrdinal, part.FirstLockOrdinal, count, assemblyDebianObjectRef{"binary-binding-part", assemblyCatalogDigest(body), int64(len(body))}})
	}
	return s, indexes, root, parts, proofs
}

func TestAssemblyDebianBinaryClosedCoverageIsNotAdoption(t *testing.T) {
	s, indexes, root, parts, proofs := assemblyBinaryCoverageFixture(t)
	if err := checkAssemblyDebianBinaryCoverage(s, indexes, root, parts, proofs); err != nil {
		t.Fatal(err)
	}
	for _, which := range []string{"missing-part", "missing-index", "source-claim", "dependency-claim", "installed-claim", "output-claim", "eligibility", "compressed-budget", "expansion-budget", "read-calls", "role", "cross-scope", "part-digest", "span-overflow", "overlapping-paragraph", "ordinal", "opaque-version", "relation-case", "historical-conflation", "orphan-frame"} {
		t.Run(which, func(t *testing.T) {
			s, indexes, root, parts, proofs := assemblyBinaryCoverageFixture(t)
			switch which {
			case "missing-part":
				parts = parts[:10]
			case "missing-index":
				root.DecodedIndexes = root.DecodedIndexes[:3]
			case "source-claim":
				root.SourceArchiveBinding = "verified"
			case "dependency-claim":
				root.DependencyClosure = "satisfied"
			case "installed-claim":
				root.ExpectedInstalledFiles = "derived"
			case "output-claim":
				root.ObservedInstalledOutput = "verified"
			case "eligibility":
				root.ServerEligibility = "eligible"
			case "compressed-budget":
				root.Counters.CompressedReadBytes = assemblyDebianBinaryReadLimit + 1
			case "expansion-budget":
				root.Counters.ExpandedBytes = 513 << 20
			case "read-calls":
				root.Counters.ReadCalls = 1
			case "role":
				parts[0].Bindings[0].DebObject.Role = "source-archive"
			case "cross-scope":
				root.SignedIndexes.Digest = strings.Repeat("0", 64)
			case "part-digest":
				root.Parts[0].Object.SHA256 = strings.Repeat("0", 64)
			case "span-overflow":
				parts[0].Bindings[0].Matches[0].ParagraphOffset = int64(^uint64(0) >> 1)
			case "overlapping-paragraph":
				parts[0].Bindings[1].Matches[0].ParagraphOffset = parts[0].Bindings[0].Matches[0].ParagraphOffset
			case "ordinal":
				parts[0].Bindings[0].LockOrdinal = 1
			case "opaque-version":
				parts[0].Bindings[0].Version = strings.ToLower(parts[0].Bindings[0].Version)
			case "relation-case":
				proofs[0].frames[1].fields["depends"] = "liba:any (>= 1) | LibB"
			case "historical-conflation":
				parts[0].Bindings[0].Matches[0].IndexBindingOrdinals = []int{11}
			case "orphan-frame":
				proofs[0].frames = append(proofs[0].frames, proofs[0].frames[0])
			}
			// Rebind changed pure part bytes so semantic failures cannot be
			// accidentally tested only by a stale canonical part digest.
			if which != "part-digest" {
				for n, p := range parts {
					body, _ := json.Marshal(p)
					root.Parts[n].Object = assemblyDebianObjectRef{"binary-binding-part", assemblyCatalogDigest(body), int64(len(body))}
				}
			}
			if err := checkAssemblyDebianBinaryCoverage(s, indexes, root, parts, proofs); err == nil {
				t.Fatal("invalid observation accepted", which)
			}
		})
	}
}

func TestAssemblyDebianBinaryBundleRejectsTrailingFramingAndPartialRows(t *testing.T) {
	_, _, _, _, proofs := assemblyBinaryCoverageFixture(t)
	if len(proofs[0].frames) != 48 {
		t.Fatal("fixture")
	}
	// A complete parseable record is required in every framed slot. Explicit
	// duplicate, missing, overflow and foreign/trailing frames refuse.
	for _, which := range []string{"valid", "missing-rows", "duplicate", "size-overflow", "hash", "slot", "foreign-ordinal", "trailing", "malformed-fields", "unknown-claims", "duplicate-claims"} {
		t.Run(which, func(t *testing.T) {
			raw := []byte("LUCI-DBB1\n")
			payload := []byte("Package: synthetic\n")
			for n := 0; n < 16; n++ {
				for kind := 1; kind <= 3; kind++ {
					body := payload
					if kind == 3 {
						body, _ = json.Marshal(assemblyDebianBinarySelectedFieldsV1{1, assemblyDebianBinaryFieldsV1{}, []assemblyDebianBinaryIndexFieldsV1{{[]int{0}, assemblyDebianBinaryFieldsV1{}}}})
					}
					if which == "malformed-fields" && n == 0 && kind == 1 {
						body = []byte("Package: a\npackage: b\n")
					}
					if which == "unknown-claims" && n == 0 && kind == 3 {
						body = append(body[:len(body)-1], []byte(`,"authority":true}`)...)
					}
					if which == "duplicate-claims" && n == 0 && kind == 3 {
						body = append(body[:len(body)-1], []byte(`,"schema":1}`)...)
					}
					frame := assemblyBinaryFrameBytes(n, kind, 0, body)
					if n == 0 && kind == 1 {
						switch which {
						case "size-overflow":
							binary.LittleEndian.PutUint32(frame[4:8], ^uint32(0))
						case "hash":
							frame[8] ^= 1
						case "slot":
							frame[3] = 3
						case "foreign-ordinal":
							binary.LittleEndian.PutUint16(frame, 173)
						}
					}
					raw = append(raw, frame...)
					if which == "duplicate" && n == 0 && kind == 1 {
						raw = append(raw, frame...)
					}
				}
				if which == "missing-rows" {
					break
				}
			}
			if which == "trailing" {
				raw = append(raw, 0)
			}
			ref := assemblyDebianObjectRef{"binary-binding-bundle", assemblyCatalogDigest(raw), int64(len(raw))}
			_, err := decodeAssemblyDebianBinaryBundle(raw, ref, 0)
			if which == "valid" && err != nil {
				t.Fatal("valid complete framing refused", err)
			}
			if which != "valid" && err == nil {
				t.Fatal("bad frame accepted")
			}
		})
	}
}

func TestAssemblyDebianBinaryIncompleteCannotBecomeCompleteRoot(t *testing.T) {
	value := assemblyDebianBinaryIncompleteV1{1, "incomplete-observations", []int{0, 3, 173}, "missing exact package paragraph"}
	if checkAssemblyDebianBinaryIncomplete(value) != nil {
		t.Fatal("diagnostics refused")
	}
	value.State = "complete-observations-not-adopted"
	if checkAssemblyDebianBinaryIncomplete(value) == nil {
		t.Fatal("incomplete state relabelled")
	}
	value.State = "incomplete-observations"
	value.SeenOrdinals = []int{3, 3}
	if checkAssemblyDebianBinaryIncomplete(value) == nil {
		t.Fatal("duplicate diagnostics ordinal")
	}
}
