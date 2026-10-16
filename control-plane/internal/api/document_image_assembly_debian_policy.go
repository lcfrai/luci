package api

import (
	"encoding/json"
	"errors"
	"net/mail"
	"sort"
	"strings"
	"time"
)

const assemblyDebianPolicySourceSHA = "13bd77f40356ef37ae01f3baf05172a035423815b33e9a292a7736ec699150a5"
const assemblyDebianLockSHA = "91f2739b1ccfb8c5277e41ebbcaa45ba7ba16f1018abdf0122fc39c1ec0d0423"
const assemblyDebianManifestSHA = "f839f2d1edaf0fbc1f039d20fcc4abd83b04accba2d0846e369174a0f2bad8b2"
const assemblyDebianSnapshotTimestamp = "20260918T000000Z"
const assemblyDebianConfigSHA = "64583d9f02277a0f311d1f6776850471b1720f18ea22f24bb93381c6c00b5205"
const assemblyDebianProjectionSHA = "8401b441e3ecccd386aace046d71bb7a0941f74d105addc6a22965f7bdae0f3b"

var errAssemblyDebianEvidence = errors.New("private Debian evidence is missing, stale or inconsistent")

// Normalize only the exact already selected official source policy. Hash-bound
// source metadata is normative input, never an observed key/signature receipt.
// The revision is supplied by the existing installer owner, not HTTP or a model.
// No files, keys, process, clock, current catalog, or accepted index are changed.
func normalizeAssemblyDebianPolicy(source, producerConfig []byte, revision string) (assemblyDebianRootPolicyV1, error) {
	var out assemblyDebianRootPolicyV1
	if len(source) != 108143 || assemblyCatalogDigest(source) != assemblyDebianPolicySourceSHA || len(producerConfig) != 41817 || assemblyCatalogDigest(producerConfig) != assemblyDebianConfigSHA || !validAssemblyCatalogPolicy(assemblyCatalogPolicy{revision, assemblyDebianPolicySourceSHA}) {
		return out, errAssemblyDebianEvidence
	}
	// The source is exact digest-pinned13bd, which contains extra explanatory
	// metadata. This small projection deliberately never interprets status flags.
	var input struct {
		Version          int    `json:"version"`
		LockSHA          string `json:"lockSha256"`
		ManifestSHA      string `json:"manifestSha256"`
		RootOriginPolicy struct {
			Roots []struct {
				ID      string  `json:"id"`
				Primary string  `json:"primaryFingerprint"`
				Subkey  *string `json:"signingSubkeyFingerprint"`
				Origin  string  `json:"origin"`
				Bytes   int64   `json:"bytes"`
				SHA     string  `json:"sha256"`
				Expiry  *string `json:"announcedExpiry"`
			} `json:"roots"`
		} `json:"rootOriginPolicy"`
		Releases []struct {
			ID      string   `json:"id"`
			Era     string   `json:"era"`
			Bytes   int64    `json:"bytes"`
			SHA     string   `json:"sha256"`
			Fields  []string `json:"fields"`
			RootIDs []string `json:"allowedRootIDs"`
			Indexes []struct {
				Path  string `json:"path"`
				Bytes int64  `json:"bytes"`
				SHA   string `json:"sha256"`
			} `json:"indexCandidates"`
		} `json:"releaseCandidates"`
		Historical struct {
			Timestamp string `json:"timestamp"`
		} `json:"historicalPolicy"`
	}
	if json.Unmarshal(source, &input) != nil || input.Version != 3 || input.LockSHA != assemblyDebianLockSHA || input.ManifestSHA != assemblyDebianManifestSHA || len(input.RootOriginPolicy.Roots) != 3 || len(input.Releases) != 6 || input.Historical.Timestamp != assemblyDebianSnapshotTimestamp {
		return out, errAssemblyDebianEvidence
	}
	out = assemblyDebianRootPolicyV1{Schema: 1, RecordType: "luci-debian-official-root-policy-v1", SourcePolicy: assemblyDebianObjectRef{Role: "policy-source", SHA256: assemblyDebianPolicySourceSHA, Bytes: int64(len(source))}, PolicyRevision: revision, DependencyLockSHA256: input.LockSHA, AcquisitionManifestSHA256: input.ManifestSHA, FutureSkewSeconds: 300, CurrentReleaseMaxAgeSeconds: 172800, RevocationKnowledge: "pinned-snapshot-only", KeyObservationMaxAgeSeconds: 86400}
	// These are fixed normative bindings from exact v4 source metadata, not
	// observed snapshot/crypto/base-layer receipt facts.
	out.ProducerConfig = assemblyDebianObjectRef{"producer-config", assemblyDebianConfigSHA, int64(len(producerConfig))}
	out.HistoricalAnchor = expectedAssemblyDebianHistoricalAnchor()
	for _, root := range input.RootOriginPolicy.Roots {
		out.Roots = append(out.Roots, assemblyDebianRootV1{ID: root.ID, PrimaryFingerprint: root.Primary, SigningSubkeyFingerprint: root.Subkey, PublicKey: assemblyDebianObjectRef{Role: "public-key", SHA256: root.SHA, Bytes: root.Bytes}, OfficialOrigin: root.Origin, AnnouncedExpiryDate: root.Expiry})
	}
	for _, release := range input.Releases {
		selection := assemblyDebianReleaseSelectionV1{CandidateID: release.ID, SignedRelease: assemblyDebianObjectRef{Role: "signed-release", SHA256: release.SHA, Bytes: release.Bytes}, AllowedRootIDs: append([]string(nil), release.RootIDs...)}
		sort.Strings(selection.AllowedRootIDs)
		switch release.ID {
		case "current/bookworm":
			selection.Role = "exact-point-anchor"
		case "current/bookworm-updates":
			selection.Role = "current-updates"
		case "current/bookworm-security":
			selection.Role = "current-security"
		case "historical-base-anchor/bookworm", "historical-base-anchor/bookworm-updates", "historical-base-anchor/bookworm-security":
			selection.Role = "historical-reproducible-anchor"
			timestamp := input.Historical.Timestamp
			selection.HistoricalTimestamp = &timestamp
		default:
			return assemblyDebianRootPolicyV1{}, errAssemblyDebianEvidence
		}
		if release.Era != "current" && release.Era != "historical-base-anchor" || (release.Era == "historical-base-anchor") != (selection.HistoricalTimestamp != nil) {
			return assemblyDebianRootPolicyV1{}, errAssemblyDebianEvidence
		}
		fields := map[string]string{}
		for _, field := range release.Fields {
			key, value, ok := strings.Cut(field, ":")
			if !ok || fields[key] != "" {
				return assemblyDebianRootPolicyV1{}, errAssemblyDebianEvidence
			}
			fields[key] = strings.TrimSpace(value)
		}
		selection.Origin, selection.Label, selection.Suite, selection.Codename = fields["Origin"], fields["Label"], fields["Suite"], fields["Codename"]
		date, err := mail.ParseDate(fields["Date"])
		if err != nil {
			return assemblyDebianRootPolicyV1{}, errAssemblyDebianEvidence
		}
		selection.ExpectedReleaseDate = date.Unix()
		if fields["Valid-Until"] != "" {
			until, err := mail.ParseDate(fields["Valid-Until"])
			if err != nil || !until.After(date) {
				return assemblyDebianRootPolicyV1{}, errAssemblyDebianEvidence
			}
			epoch := until.Unix()
			selection.ExpectedValidUntil = &epoch
		}
		for _, index := range release.Indexes {
			selection.SignedIndexes = append(selection.SignedIndexes, assemblyDebianSignedIndexClaimV1{index.Path, index.Bytes, index.SHA})
		}
		sort.Slice(selection.SignedIndexes, func(i, j int) bool { return selection.SignedIndexes[i].Path < selection.SignedIndexes[j].Path })
		out.Releases = append(out.Releases, selection)
	}
	sort.Slice(out.Roots, func(i, j int) bool { return out.Roots[i].ID < out.Roots[j].ID })
	sort.Slice(out.Releases, func(i, j int) bool { return out.Releases[i].CandidateID < out.Releases[j].CandidateID })
	if err := checkAssemblyDebianPolicyShape(out); err != nil {
		return assemblyDebianRootPolicyV1{}, err
	}
	return out, nil
}

func expectedAssemblyDebianHistoricalAnchor() assemblyDebianHistoricalAnchorV1 {
	return assemblyDebianHistoricalAnchorV1{ID: "bookworm-base-rootfs-20260918", SnapshotTimestamp: assemblyDebianSnapshotTimestamp, SnapshotSourceRef: assemblyDebianObjectRef{"base-rootfs-debian-sources-snapshot", "615c3e2f894afdad54d0ff676117ae6be513b4bf500b05660cecbcb5238f7feb", 372}, TrustPolicySHA256: assemblyDebianPolicySourceSHA, DependencyLockSHA256: assemblyDebianLockSHA, BootstrapConfigDigest: "sha256:db9f02c6bde9fa90cc8074c92754b2b046947392f1a857726e77f041febb7b82", CompressedLayerRef: assemblyDebianObjectRef{"oci-compressed-base-layer", "774043ccc8ccd0d0833a9ee0792142ab7ad93df971e59dd248fbf82db16d0150", 28238443}, DebuerreotypeRootfsArchiveSHA256: "ca543f89919cb79fc16c8e96b957e75c64bed1dcecc64f320c7388aefc9676a8"}
}

func assemblyDebianFingerprint(value string, length int) bool {
	return len(value) == length && strings.Trim(value, "0123456789ABCDEF") == ""
}

func checkAssemblyDebianObject(ref assemblyDebianObjectRef, role string, limit int64) bool {
	return ref.Role == role && assemblyCatalogHex(ref.SHA256) && ref.Bytes > 0 && ref.Bytes <= limit
}

func assemblyDebianRecordRef(kind string, value any) (assemblyCatalogRef, error) {
	data, err := json.Marshal(value)
	if err != nil || len(data) > assemblyCatalogRecordLimit {
		return assemblyCatalogRef{}, errAssemblyDebianEvidence
	}
	ref := assemblyCatalogRef{1, kind, assemblyCatalogDigest(data), int64(len(data))}
	if !validAssemblyCatalogRef(ref) {
		return assemblyCatalogRef{}, errAssemblyDebianEvidence
	}
	return ref, nil
}

// Shape checks validate untrusted metadata only. Actual policy selection must use
// the digest-bound normalizer above; a caller-constructed shape cannot adopt it.
func checkAssemblyDebianPolicyShape(policy assemblyDebianRootPolicyV1) error {
	if policy.Schema != 1 || policy.RecordType != "luci-debian-official-root-policy-v1" || policy.SourcePolicy != (assemblyDebianObjectRef{"policy-source", assemblyDebianPolicySourceSHA, 108143}) || policy.ProducerConfig != (assemblyDebianObjectRef{"producer-config", assemblyDebianConfigSHA, 41817}) || policy.HistoricalAnchor != expectedAssemblyDebianHistoricalAnchor() || policy.DependencyLockSHA256 != assemblyDebianLockSHA || policy.AcquisitionManifestSHA256 != assemblyDebianManifestSHA || !validAssemblyCatalogPolicy(assemblyCatalogPolicy{policy.PolicyRevision, assemblyDebianPolicySourceSHA}) || policy.FutureSkewSeconds != 300 || policy.CurrentReleaseMaxAgeSeconds != 172800 || policy.KeyObservationMaxAgeSeconds != 86400 || policy.RevocationKnowledge != "pinned-snapshot-only" || len(policy.Roots) != 3 || len(policy.Releases) != 6 {
		return errAssemblyDebianEvidence
	}
	known := map[string]bool{}
	previous := ""
	for _, root := range policy.Roots {
		if root.ID <= previous || !assemblyDebianFingerprint(root.PrimaryFingerprint, 40) || root.SigningSubkeyFingerprint != nil && !assemblyDebianFingerprint(*root.SigningSubkeyFingerprint, 40) || !checkAssemblyDebianObject(root.PublicKey, "public-key", 65536) || len(root.OfficialOrigin) > 256 {
			return errAssemblyDebianEvidence
		}
		known[root.ID], previous = true, root.ID
	}
	previous = ""
	objects := map[string]bool{}
	for _, selection := range policy.Releases {
		if selection.CandidateID <= previous || !checkAssemblyDebianObject(selection.SignedRelease, "signed-release", 262144) || len(selection.AllowedRootIDs) < 1 || len(selection.AllowedRootIDs) > 2 || !assemblyDebianEpoch(selection.ExpectedReleaseDate) || selection.ExpectedValidUntil != nil && (!assemblyDebianEpoch(*selection.ExpectedValidUntil) || *selection.ExpectedValidUntil <= selection.ExpectedReleaseDate) || len(selection.SignedIndexes) != 2 {
			return errAssemblyDebianEvidence
		}
		last := ""
		for _, id := range selection.AllowedRootIDs {
			if !known[id] || id <= last {
				return errAssemblyDebianEvidence
			}
			last = id
		}
		for i, index := range selection.SignedIndexes {
			path := "main/binary-amd64/Packages.xz"
			if i == 1 {
				path = "main/source/Sources.xz"
			}
			if index.Path != path || index.Bytes <= 0 || index.Bytes > 16<<20 || !assemblyCatalogHex(index.SHA256) {
				return errAssemblyDebianEvidence
			}
		}
		if selection.HistoricalTimestamp != nil {
			if selection.Role != "historical-reproducible-anchor" || *selection.HistoricalTimestamp != assemblyDebianSnapshotTimestamp {
				return errAssemblyDebianEvidence
			}
			timestamp, err := time.Parse("20060102T150405Z", *selection.HistoricalTimestamp)
			if err != nil || selection.ExpectedReleaseDate > timestamp.Unix()+policy.FutureSkewSeconds {
				return errAssemblyDebianEvidence
			}
		} else if selection.Role != "current-updates" && selection.Role != "current-security" && selection.Role != "exact-point-anchor" {
			return errAssemblyDebianEvidence
		}
		objects[selection.SignedRelease.SHA256], previous = true, selection.CandidateID
	}
	if len(objects) != 5 {
		return errAssemblyDebianEvidence
	}
	_, err := assemblyDebianRecordRef("official-root-policy", policy)
	return err
}
