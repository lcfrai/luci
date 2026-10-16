package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// These functions validate data only. They cannot mint a captured run, certify
// original ownership, evaluate current installer authority or publish records.
type assemblyBinaryRawTuple struct {
	Name                  string   `json:"name"`
	Version               string   `json:"version"`
	Architecture          string   `json:"architecture"`
	File                  string   `json:"file"`
	Bytes                 int64    `json:"bytes"`
	SHA256                string   `json:"sha256"`
	URL                   string   `json:"url"`
	Acquired              bool     `json:"acquired"`
	ReleaseCandidateIDs   []string `json:"releaseCandidateIDs"`
	TupleCoverageVerified bool     `json:"tupleCoverageVerified"`
}

func normalizeAssemblyDebianBinarySelection(lock, manifest, policy []byte) (assemblyDebianBinarySelectionV1, error) {
	var empty assemblyDebianBinarySelectionV1
	if len(lock) > 1<<20 || len(manifest) > 256<<10 || len(policy) > 256<<10 || assemblyCatalogDigest(lock) != assemblyDebianLockSHA || assemblyCatalogDigest(manifest) != assemblyDebianManifestSHA || assemblyCatalogDigest(policy) != assemblyDebianPolicySourceSHA {
		return empty, errAssemblyDebianEvidence
	}
	var l struct {
		Packages []assemblyBinaryRawTuple `json:"debianPackages"`
	}
	var m struct {
		Packages []assemblyBinaryRawTuple `json:"debianControlInputs"`
	}
	var p struct {
		Packages []assemblyBinaryRawTuple `json:"packageCandidateBindings"`
	}
	if json.Unmarshal(lock, &l) != nil || json.Unmarshal(manifest, &m) != nil || json.Unmarshal(policy, &p) != nil || len(l.Packages) != 174 || len(m.Packages) != 174 || len(p.Packages) != 174 {
		return empty, errAssemblyDebianEvidence
	}
	locked, candidates := map[string]assemblyBinaryRawTuple{}, map[string]assemblyBinaryRawTuple{}
	for _, row := range l.Packages {
		if _, ok := locked[row.File]; ok {
			return empty, errAssemblyDebianEvidence
		}
		locked[row.File] = row
	}
	for _, row := range p.Packages {
		if _, ok := candidates[row.File]; ok {
			return empty, errAssemblyDebianEvidence
		}
		candidates[row.File] = row
	}
	out := assemblyDebianBinarySelectionV1{Schema: 1, DependencyLockSHA256: assemblyDebianLockSHA, AcquisitionManifestSHA256: assemblyDebianManifestSHA, CandidatePolicySHA256: assemblyDebianPolicySourceSHA}
	for ordinal, row := range m.Packages {
		base, ok := locked[row.File]
		candidate, found := candidates[row.File]
		if !ok || !found || row.Acquired || base.Acquired || candidate.TupleCoverageVerified || !sameAssemblyBinaryRawTuple(row, base) || !sameAssemblyBinaryRawTuple(row, candidate) || row.URL != base.URL {
			return empty, errAssemblyDebianEvidence
		}
		u, err := url.Parse(row.URL)
		if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" || (u.Host != "deb.debian.org" && u.Host != "security.debian.org") {
			return empty, errAssemblyDebianEvidence
		}
		path := u.EscapedPath() // NEVER percent-decode a signed Filename comparison.
		var filename string
		for _, prefix := range []string{"/debian/", "/debian-security/"} {
			if strings.HasPrefix(path, prefix) {
				filename = strings.TrimPrefix(path, prefix)
			}
		}
		if filename == "" || filename[strings.LastIndex(filename, "/")+1:] != row.File {
			return empty, errAssemblyDebianEvidence
		}
		ids := append([]string(nil), candidate.ReleaseCandidateIDs...)
		sort.Strings(ids)
		out.Packages = append(out.Packages, assemblyDebianBinaryExpectedTupleV1{ordinal, row.Name, row.Version, row.Architecture, filename, row.Bytes, row.SHA256, ids})
	}
	if checkAssemblyDebianBinarySelection(out) != nil {
		return empty, errAssemblyDebianEvidence
	}
	return out, nil
}

func sameAssemblyBinaryRawTuple(a, b assemblyBinaryRawTuple) bool {
	return a.Name == b.Name && a.Version == b.Version && a.Architecture == b.Architecture && a.File == b.File && a.Bytes == b.Bytes && a.SHA256 == b.SHA256
}

func assemblyBinaryText(s string, maximum int) bool {
	if len(s) == 0 || len(s) > maximum || !utf8.ValidString(s) {
		return false
	}
	for _, c := range []byte(s) {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}

func checkAssemblyDebianBinarySelection(s assemblyDebianBinarySelectionV1) error {
	if s.Schema != 1 || s.DependencyLockSHA256 != assemblyDebianLockSHA || s.AcquisitionManifestSHA256 != assemblyDebianManifestSHA || s.CandidatePolicySHA256 != assemblyDebianPolicySourceSHA || len(s.Packages) != 174 {
		return errAssemblyDebianEvidence
	}
	seen, total := map[string]bool{}, int64(0)
	allowed := map[string]bool{"current/bookworm": true, "current/bookworm-updates": true, "current/bookworm-security": true, "historical-base-anchor/bookworm": true, "historical-base-anchor/bookworm-updates": true, "historical-base-anchor/bookworm-security": true}
	for ordinal, p := range s.Packages {
		key := p.Name + "\x00" + p.Version + "\x00" + p.Architecture
		if p.Ordinal != ordinal || seen[key] || !assemblyBinaryText(p.Name, 128) || strings.Trim(p.Name, "abcdefghijklmnopqrstuvwxyz0123456789+.-") != "" || !assemblyBinaryText(p.Version, 256) || strings.ContainsAny(p.Version, " \t") || (p.Architecture != "amd64" && p.Architecture != "all") || !strings.HasPrefix(p.Filename, "pool/") || !assemblyBinaryText(p.Filename, 1024) || strings.ContainsAny(p.Filename, "\\?#") || p.Bytes <= 0 || p.Bytes > 64<<20 || p.Bytes > assemblyDebianBinaryBytes-total || !assemblyCatalogHex(p.SHA256) || len(p.AllowedCandidateIDs) == 0 || len(p.AllowedCandidateIDs) > 6 {
			return errAssemblyDebianEvidence
		}
		for _, part := range strings.Split(p.Filename, "/") {
			if part == "" || part == "." || part == ".." {
				return errAssemblyDebianEvidence
			}
		}
		previous := ""
		for _, id := range p.AllowedCandidateIDs {
			if !allowed[id] || id <= previous {
				return errAssemblyDebianEvidence
			}
			previous = id
		}
		seen[key] = true
		total += p.Bytes
	}
	if total != assemblyDebianBinaryBytes {
		return errAssemblyDebianEvidence
	}
	return nil
}

var assemblyBinaryFieldNames = []string{"package", "version", "architecture", "depends", "pre-depends", "provides", "breaks", "conflicts", "replaces", "source", "built-using", "static-built-using", "multi-arch"}

// One physical record only. Values unfold continuation SP/TAB edges to a
// single ASCII space; internal case/whitespace/operators are never changed.
func parseAssemblyBinaryParagraph(raw []byte) (map[string]string, error) {
	if len(raw) == 0 || len(raw) > 65536 || !utf8.Valid(raw) {
		return nil, errAssemblyDebianEvidence
	}
	fields, previous, ended := map[string]string{}, "", false
	for len(raw) > 0 {
		line := raw
		if end := bytes.IndexByte(raw, '\n'); end >= 0 {
			line = raw[:end+1]
			raw = raw[end+1:]
		} else {
			raw = nil
		}
		if len(line) > 16384 {
			return nil, errAssemblyDebianEvidence
		}
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
		}
		for _, c := range line {
			if c < 32 && c != '\t' || c == 127 {
				return nil, errAssemblyDebianEvidence
			}
		}
		if len(line) == 0 {
			if len(fields) > 0 {
				ended = true
			}
			continue
		}
		if ended {
			return nil, errAssemblyDebianEvidence
		}
		if line[0] == ' ' || line[0] == '\t' {
			if previous == "" {
				return nil, errAssemblyDebianEvidence
			}
			fields[previous] += " " + strings.Trim(string(line), " \t")
		} else {
			colon := bytes.IndexByte(line, ':')
			if colon < 1 || colon > 64 {
				return nil, errAssemblyDebianEvidence
			}
			name := string(line[:colon])
			if strings.Trim(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-") != "" {
				return nil, errAssemblyDebianEvidence
			}
			name = strings.ToLower(name)
			if _, ok := fields[name]; ok || len(fields) >= 128 {
				return nil, errAssemblyDebianEvidence
			}
			fields[name] = strings.Trim(string(line[colon+1:]), " \t")
			previous = name
		}
		if len(fields[previous]) > 8192 {
			return nil, errAssemblyDebianEvidence
		}
	}
	if len(fields) == 0 {
		return nil, errAssemblyDebianEvidence
	}
	return fields, nil
}

func assemblyBinarySelectedFields(fields map[string]string) assemblyDebianBinaryFieldsV1 {
	p := func(key string) *string {
		value, ok := fields[key]
		if !ok {
			return nil
		}
		return &value
	}
	return assemblyDebianBinaryFieldsV1{p("package"), p("version"), p("architecture"), p("depends"), p("pre-depends"), p("provides"), p("breaks"), p("conflicts"), p("replaces"), p("source"), p("built-using"), p("static-built-using"), p("multi-arch")}
}

func assemblyBinaryFieldValue(value string) bool {
	if len(value) > 8192 || !utf8.ValidString(value) {
		return false
	}
	for _, c := range []byte(value) {
		if c < 32 && c != '\t' || c == 127 {
			return false
		}
	}
	return true
}

func checkAssemblyBinaryClaimKeys(data []byte) error {
	var raw struct {
		Schema  int                `json:"schema"`
		Control map[string]*string `json:"control"`
		Indexes []struct {
			BindingOrdinals []int              `json:"bindingOrdinals"`
			Fields          map[string]*string `json:"fields"`
		} `json:"indexes"`
	}
	if decodeAssemblyDebianRaw(data, &raw) != nil || raw.Schema != 1 || len(raw.Indexes) == 0 || len(raw.Indexes) > 4 {
		return errAssemblyDebianEvidence
	}
	check := func(fields map[string]*string) bool {
		if len(fields) != len(assemblyBinaryFieldNames) {
			return false
		}
		for _, key := range assemblyBinaryFieldNames {
			value, ok := fields[key]
			if !ok {
				return false
			}
			if value != nil && !assemblyBinaryFieldValue(*value) {
				return false
			}
		}
		return true
	}
	if !check(raw.Control) {
		return errAssemblyDebianEvidence
	}
	for _, row := range raw.Indexes {
		if !check(row.Fields) {
			return errAssemblyDebianEvidence
		}
	}
	return nil
}

func decodeAssemblyDebianBinaryBundle(data []byte, ref assemblyDebianObjectRef, part int) (assemblyDebianBinaryBundle, error) {
	var empty assemblyDebianBinaryBundle
	if !checkAssemblyDebianObject(ref, "binary-binding-bundle", assemblyDebianBinaryBundleLimit) || int64(len(data)) != ref.Bytes || assemblyCatalogDigest(data) != ref.SHA256 || part < 0 || part > 10 || !bytes.HasPrefix(data, []byte("LUCI-DBB1\n")) {
		return empty, errAssemblyDebianEvidence
	}
	count := 16
	if part == 10 {
		count = 14
	}
	out := assemblyDebianBinaryBundle{object: ref, first: part * 16, count: count}
	cursor, previous := int64(len("LUCI-DBB1\n")), -1
	seen := map[int]int{}
	paragraphSlots := map[int]int{}
	for cursor < int64(len(data)) {
		if int64(len(data))-cursor < 40 || len(out.frames) >= 96 {
			return empty, errAssemblyDebianEvidence
		}
		header := data[cursor : cursor+40]
		ordinal := int(binary.LittleEndian.Uint16(header[:2]))
		kind, slot := int(header[2]), int(header[3])
		size := int64(binary.LittleEndian.Uint32(header[4:8]))
		cursor += 40
		key := ordinal*16 + kind*4 + slot
		if ordinal < out.first || ordinal >= out.first+count || kind < 1 || kind > 3 || slot >= 4 || kind != 1 && slot != 0 || key <= previous || size <= 0 || size > 65536 || size > int64(len(data))-cursor {
			return empty, errAssemblyDebianEvidence
		}
		previous = key
		payload := data[cursor : cursor+size]
		digest := assemblyCatalogDigest(payload)
		if !bytes.Equal(header[8:], assemblyBinaryDigestBytes(digest)) {
			return empty, errAssemblyDebianEvidence
		}
		frame := assemblyDebianBinaryFrame{ordinal: ordinal, kind: kind, slot: slot, span: assemblyDebianBinarySpanV1{ref, cursor, size, digest}}
		if kind == 1 || kind == 2 {
			fields, err := parseAssemblyBinaryParagraph(payload)
			if err != nil {
				return empty, err
			}
			frame.fields = fields
		} else {
			if checkAssemblyBinaryClaimKeys(payload) != nil {
				return empty, errAssemblyDebianEvidence
			}
			var claims assemblyDebianBinarySelectedFieldsV1
			if decodeAssemblyDebianRaw(payload, &claims) != nil {
				return empty, errAssemblyDebianEvidence
			}
			frame.claims = &claims
		}
		if kind == 1 {
			if slot != paragraphSlots[ordinal] {
				return empty, errAssemblyDebianEvidence
			}
			paragraphSlots[ordinal]++
		}
		seen[ordinal] |= 1 << kind
		out.frames = append(out.frames, frame)
		cursor += size
	}
	if len(seen) != count {
		return empty, errAssemblyDebianEvidence
	}
	for _, mask := range seen {
		if mask != 14 {
			return empty, errAssemblyDebianEvidence
		}
	}
	return out, nil
}

func assemblyBinaryDigestBytes(digest string) []byte {
	// digest comes from assemblyCatalogDigest, never caller hex.
	const digits = "0123456789abcdef"
	out := make([]byte, 32)
	for i := range out {
		out[i] = byte(strings.IndexByte(digits, digest[2*i])<<4 | strings.IndexByte(digits, digest[2*i+1]))
	}
	return out
}

func checkAssemblyDebianBinaryIncomplete(value assemblyDebianBinaryIncompleteV1) error {
	if value.Schema != 1 || value.State != "incomplete-observations" || len(value.SeenOrdinals) > 174 || !assemblyBinaryText(value.Reason, 2000) {
		return errAssemblyDebianEvidence
	}
	previous := -1
	for _, n := range value.SeenOrdinals {
		if n <= previous || n >= 174 {
			return errAssemblyDebianEvidence
		}
		previous = n
	}
	return nil
}

// Pure closed coverage check, NOT a publishable/native-owned candidate. A future
// real capture constructor must additionally establish complete original reads,
// current first3 time-use, process settlement, guards and same-owner current CAS.
func checkAssemblyDebianBinaryCoverage(selection assemblyDebianBinarySelectionV1, indexes assemblyDebianSignedIndexBindingsV1, root assemblyDebianBinariesV1, parts []assemblyDebianBinaryPartV1, bundles []assemblyDebianBinaryBundle) error {
	if checkAssemblyDebianBinarySelection(selection) != nil || len(indexes.Bindings) != 12 || len(indexes.Objects) != 8 || indexes.DependencyLockSHA256 != selection.DependencyLockSHA256 || indexes.AcquisitionManifestSHA256 != selection.AcquisitionManifestSHA256 || len(parts) != 11 || len(bundles) != 11 || len(root.Parts) != 11 || len(root.DecodedIndexes) != 4 {
		return errAssemblyDebianEvidence
	}
	indexRef, err := assemblyDebianRecordRef("signed-index-bindings", indexes)
	if err != nil {
		return err
	}
	if root.Schema != 1 || root.RecordType != "luci-debian-binary-bindings-v1" || root.ObservationState != "complete-observations-not-adopted" || root.SignedIndexes != indexRef || root.DependencyLockSHA256 != selection.DependencyLockSHA256 || root.AcquisitionManifestSHA256 != selection.AcquisitionManifestSHA256 || root.PackageCount != 174 || root.PackageBytes != assemblyDebianBinaryBytes || root.SourceArchiveBinding != "not-evaluated" || root.DependencyClosure != "not-evaluated" || root.ExpectedInstalledFiles != "not-derived" || root.ObservedInstalledOutput != "not-observed" || root.ServerEligibility != "not-evaluated-server-owned" || !checkAssemblyDebianObject(root.Producer, "binary-binding-receipt", 65536) || !validAssemblyCatalogRef(root.RetainedManifest) || root.RetainedManifest.Kind != "evidence-index" {
		return errAssemblyDebianEvidence
	}
	encoded, err := json.Marshal(root)
	if err != nil || len(encoded) > 65536 {
		return errAssemblyDebianEvidence
	}
	indexObjects := map[string]assemblyDebianObjectRef{}
	logical := 0
	compressed := int64(0)
	for _, binding := range indexes.Bindings {
		if binding.IndexPath == "main/binary-amd64/Packages.xz" {
			logical++
			o := binding.CompressedObject
			if !checkAssemblyDebianObject(o, "xz-index", 16<<20) {
				return errAssemblyDebianEvidence
			}
			if old, ok := indexObjects[o.SHA256]; ok {
				if old != o {
					return errAssemblyDebianEvidence
				}
			} else {
				indexObjects[o.SHA256] = o
				compressed += o.Bytes
			}
		}
	}
	if logical != 6 || len(indexObjects) != 4 || compressed != assemblyDebianBinaryIndexBytes {
		return errAssemblyDebianEvidence
	}
	decoded := map[string]assemblyDebianBinaryDecodedIndexV1{}
	expanded := int64(0)
	previous := ""
	for _, d := range root.DecodedIndexes {
		if d.Compressed.SHA256 <= previous || d.Compressed != indexObjects[d.Compressed.SHA256] || !assemblyCatalogHex(d.DecodedSHA256) || d.DecodedBytes <= 0 || d.DecodedBytes > 128<<20 || d.DecodedBytes > (256<<20)-expanded {
			return errAssemblyDebianEvidence
		}
		previous = d.Compressed.SHA256
		decoded[previous] = d
		expanded += d.DecodedBytes
	}
	c := root.Counters
	minimumReads := 0
	for _, p := range selection.Packages {
		minimumReads += 2 * (int((p.Bytes+65535)/65536) + 1)
	}
	for _, o := range indexObjects {
		minimumReads += 2 * (int((o.Bytes+65535)/65536) + 1)
	}
	if c.CompressedReadBytes < 2*(assemblyDebianBinaryBytes+assemblyDebianBinaryIndexBytes)+2*(174+4) || c.CompressedReadBytes > assemblyDebianBinaryReadLimit || c.ReadCalls < minimumReads || c.ReadCalls > 50000 || c.ExpandedBytes < expanded || c.ExpandedBytes > 512<<20 || c.ExpandedBytes-expanded > 174<<20 || c.Paragraphs < 174 || c.Paragraphs > 250000 {
		return errAssemblyDebianEvidence
	}
	totalEvidence := int64(0)
	operation := ""
	paragraphRanges := map[string][][2]int64{}
	for partOrdinal, part := range parts {
		count := 16
		if partOrdinal == 10 {
			count = 14
		}
		proof := bundles[partOrdinal]
		if partOrdinal == 0 {
			operation = part.OperationID
		}
		if !assemblyBinaryText(operation, 128) || part.OperationID != operation || part.Schema != 1 || part.RecordType != "luci-debian-binary-binding-part-v1" || part.Part != partOrdinal || part.FirstLockOrdinal != partOrdinal*16 || part.SignedIndexes != indexRef || part.DependencyLockSHA256 != selection.DependencyLockSHA256 || len(part.Bindings) != count || proof.first != part.FirstLockOrdinal || proof.count != count || !checkAssemblyDebianObject(proof.object, "binary-binding-bundle", assemblyDebianBinaryBundleLimit) || proof.object.Bytes > assemblyDebianBinaryEvidenceLimit-totalEvidence {
			return errAssemblyDebianEvidence
		}
		totalEvidence += proof.object.Bytes
		body, e := json.Marshal(part)
		if e != nil || len(body) > 65536 {
			return errAssemblyDebianEvidence
		}
		ref := root.Parts[partOrdinal]
		if ref.Part != partOrdinal || ref.FirstLockOrdinal != part.FirstLockOrdinal || ref.Count != count || ref.Object != (assemblyDebianObjectRef{"binary-binding-part", assemblyCatalogDigest(body), int64(len(body))}) {
			return errAssemblyDebianEvidence
		}
		used := 0
		for rowOrdinal, row := range part.Bindings {
			p := selection.Packages[part.FirstLockOrdinal+rowOrdinal]
			if row.LockOrdinal != p.Ordinal || row.Name != p.Name || row.Version != p.Version || row.Architecture != p.Architecture || row.Filename != p.Filename || row.DebObject != (assemblyDebianObjectRef{"deb-package", p.SHA256, p.Bytes}) || len(row.Matches) == 0 || len(row.Matches) > 4 {
				return errAssemblyDebianEvidence
			}
			frames := []assemblyDebianBinaryFrame{}
			for _, f := range proof.frames {
				if f.ordinal == p.Ordinal {
					frames = append(frames, f)
				}
			}
			if len(frames) != len(row.Matches)+2 {
				return errAssemblyDebianEvidence
			}
			used += len(frames)
			control, claimFrame := frames[len(frames)-2], frames[len(frames)-1]
			if control.kind != 2 || control.slot != 0 || control.span != row.Control || claimFrame.kind != 3 || claimFrame.slot != 0 || claimFrame.span != row.SelectedFieldClaims || claimFrame.claims == nil || !reflect.DeepEqual(claimFrame.claims.Control, assemblyBinarySelectedFields(control.fields)) || len(claimFrame.claims.Indexes) != len(row.Matches) {
				return errAssemblyDebianEvidence
			}
			if control.fields["package"] != p.Name || control.fields["version"] != p.Version || control.fields["architecture"] != p.Architecture {
				return errAssemblyDebianEvidence
			}
			seenIndex := map[string]bool{}
			for matchOrdinal, m := range row.Matches {
				frame := frames[matchOrdinal]
				d, ok := decoded[m.CompressedIndex.SHA256]
				if !ok || seenIndex[m.CompressedIndex.SHA256] || m.CompressedIndex != d.Compressed || m.DecodedIndexSHA256 != d.DecodedSHA256 || m.DecodedIndexBytes != d.DecodedBytes || frame.kind != 1 || frame.slot != matchOrdinal || frame.span != m.Paragraph || m.ParagraphOffset < 0 || m.Paragraph.Bytes > d.DecodedBytes || m.ParagraphOffset > d.DecodedBytes-m.Paragraph.Bytes {
					return errAssemblyDebianEvidence
				}
				seenIndex[m.CompressedIndex.SHA256] = true
				start, end := m.ParagraphOffset, m.ParagraphOffset+m.Paragraph.Bytes // subtraction-checked above
				for _, interval := range paragraphRanges[m.CompressedIndex.SHA256] {
					if start < interval[1] && interval[0] < end {
						return errAssemblyDebianEvidence
					}
				}
				paragraphRanges[m.CompressedIndex.SHA256] = append(paragraphRanges[m.CompressedIndex.SHA256], [2]int64{start, end})
				ordinals := []int{}
				for ordinal, binding := range indexes.Bindings {
					if binding.IndexPath == "main/binary-amd64/Packages.xz" && binding.CompressedObject == m.CompressedIndex && containsAssemblyBinaryCandidate(p.AllowedCandidateIDs, binding.CandidateID) {
						ordinals = append(ordinals, ordinal)
					}
				}
				claims := claimFrame.claims.Indexes[matchOrdinal]
				if len(ordinals) == 0 || !reflect.DeepEqual(m.IndexBindingOrdinals, ordinals) || !reflect.DeepEqual(claims.BindingOrdinals, ordinals) || !reflect.DeepEqual(claims.Fields, assemblyBinarySelectedFields(frame.fields)) {
					return errAssemblyDebianEvidence
				}
				for _, key := range assemblyBinaryFieldNames[:9] {
					a, aok := frame.fields[key]
					b, bok := control.fields[key]
					if aok != bok || a != b {
						return errAssemblyDebianEvidence
					}
				}
				if frame.fields["size"] != strconv.FormatInt(p.Bytes, 10) || frame.fields["sha256"] != p.SHA256 || frame.fields["filename"] != p.Filename {
					return errAssemblyDebianEvidence
				}
			}
		}
		if used != len(proof.frames) {
			return errAssemblyDebianEvidence
		}
	}
	return nil
}

func containsAssemblyBinaryCandidate(ids []string, id string) bool {
	for _, value := range ids {
		if value == id {
			return true
		}
	}
	return false
}
