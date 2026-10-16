package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func assemblyDebianSourceFixture(t *testing.T) ([]byte, []byte) {
	t.Helper()
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("testdata", "document-assembly-debian", name))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	return read("official-policy-13bd77f4.json"), read("producer-config-64583d9f.json")
}

// Synthetic summaries only. None of these refs represents actual native/key
// verification, raw-object retention, positive owned cleanup or accepted closure.
func assemblyDebianSummaryFixture(t *testing.T) (assemblyDebianRootPolicyV1, assemblyDebianKeyObservationsV1, assemblyDebianReleaseSignaturesV1, assemblyDebianProducerPins, int64) {
	t.Helper()
	source, config := assemblyDebianSourceFixture(t)
	policy, err := normalizeAssemblyDebianPolicy(source, config, "debian-pinned-fixture-1")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.October, 16, 10, 0, 0, 0, time.UTC).Unix()
	ref := func(role string) assemblyDebianObjectRef {
		return assemblyDebianObjectRef{role, assemblyCatalogDigest([]byte("synthetic-" + role)), 1}
	}
	pins := assemblyDebianProducerPins{SourceReceipt: ref("producer-source-receipt"), Config: policy.ProducerConfig, ProjectionSHA256: assemblyDebianProjectionSHA, GPGSHA256: "a5140c85353e8399da8d6bd7e3741524cd76a4e69be88af12042b1c9ef022984", GPGVSHA256: "f4d13204d77fdf63c02b0e6742230f83a833128c28f7b715709c2c63a96c427b"}
	producer := assemblyDebianProducerV1{Profile: "luci-retained-debian-signature-input-evidence-v2", SourceReceipt: pins.SourceReceipt, Config: pins.Config, TypedOutput: ref("producer-output"), InnerReceipt: ref("inner-receipt"), OuterReceipt: ref("outer-receipt"), ProjectionSHA256: pins.ProjectionSHA256, GPGSHA256: pins.GPGSHA256, GPGVSHA256: pins.GPGVSHA256}
	policyRef, _ := assemblyDebianRecordRef("official-root-policy", policy)
	keys := assemblyDebianKeyObservationsV1{Schema: 1, RecordType: "luci-debian-key-observations-v1", RootPolicy: policyRef, Producer: producer, ObservedAt: now - 10, KeyListing: ref("key-listing"), UnseenRevocations: "not-established"}
	for _, root := range policy.Roots {
		keys.Snapshots = append(keys.Snapshots, assemblyDebianKeySnapshotV1{root.ID, root.PublicKey, ref("keyring")})
		key := assemblyDebianKeyV1{Fingerprint: root.PrimaryFingerprint, PrimaryFingerprint: root.PrimaryFingerprint, Kind: "pub", Created: now - 365*86400, Algorithm: 1, Bits: 4096, Capabilities: "scSC", SnapshotValidity: "-"}
		keys.Keys = append(keys.Keys, key)
		if root.SigningSubkeyFingerprint != nil {
			key.Fingerprint = *root.SigningSubkeyFingerprint
			key.Kind = "sub"
			keys.Keys = append(keys.Keys, key)
		}
	}
	sort.Slice(keys.Keys, func(i, j int) bool { return keys.Keys[i].Fingerprint < keys.Keys[j].Fingerprint })
	keyRef, _ := assemblyDebianRecordRef("key-observations", keys)
	signatures := assemblyDebianReleaseSignaturesV1{Schema: 1, RecordType: "luci-debian-release-signatures-v1", RootPolicy: policyRef, KeyObservations: keyRef, Producer: producer}
	var parsed struct {
		Releases []struct {
			SHA        string `json:"sha256"`
			Bytes      int64  `json:"bytes"`
			Candidates []struct {
				ID string `json:"id"`
			} `json:"candidates"`
		} `json:"releases"`
	}
	if json.Unmarshal(config, &parsed) != nil {
		t.Fatal("producer config fixture")
	}
	selections := map[string]assemblyDebianReleaseSelectionV1{}
	rootByID := map[string]assemblyDebianRootV1{}
	for _, selection := range policy.Releases {
		selections[selection.CandidateID] = selection
	}
	for _, root := range policy.Roots {
		rootByID[root.ID] = root
	}
	for i, input := range parsed.Releases {
		object := assemblyDebianObjectRef{"signed-release", input.SHA, input.Bytes}
		pins.SignedObjects = append(pins.SignedObjects, object)
		release := assemblyDebianReleaseVerificationV1{SignedRelease: object, SignedPlaintext: ref("signed-plaintext"), MachineStatus: ref("signature-status"), ChildIndex: 4 + i, ExitCode: 0, ObservedStart: now - 5, ObservedEnd: now + 1}
		for _, candidate := range input.Candidates {
			selection := selections[candidate.ID]
			logical := assemblyDebianLogicalReleaseV1{CandidateID: candidate.ID, ReleaseDate: selection.ExpectedReleaseDate, ValidUntil: selection.ExpectedValidUntil, ProducerFreshness: assemblyDebianProducerFreshness(selection, now, policy), Era: "current", ObservedAt: now, SignatureState: "eligible-signature-verified", UpstreamExpiryState: assemblyDebianUpstreamExpiry(selection.ExpectedValidUntil, now), KeyRevocationKnowledge: "unseen-revocations-not-established", ServerEligibility: "not-evaluated-server-owned", SignedIndexes: selection.SignedIndexes}
			if selection.Role == "historical-reproducible-anchor" {
				logical.Era = "historical-base-anchor"
				logical.ProducerHistoricalTimestamp = selection.HistoricalTimestamp
				anchor := policy.HistoricalAnchor
				logical.HistoricalAnchor = &anchor
				state := "absent"
				if logical.ValidUntil != nil {
					snapshot, _ := time.Parse("20060102T150405Z", assemblyDebianSnapshotTimestamp)
					if *logical.ValidUntil <= snapshot.Unix() {
						state = "expired-at-anchor"
					} else {
						state = "within-valid-until-at-anchor"
					}
				}
				logical.HistoricalExpiryState = &state
			}
			release.Logical = append(release.Logical, logical)
		}
		selection := selections[input.Candidates[0].ID]
		root := rootByID[selection.AllowedRootIDs[0]]
		signer := root.PrimaryFingerprint
		if root.SigningSubkeyFingerprint != nil {
			signer = *root.SigningSubkeyFingerprint
		}
		release.Dispositions = []assemblyDebianSignatureDispositionV1{{Kind: "eligible-valid-signature", Valid: &assemblyDebianValidSignatureV1{Signer: signer, Primary: root.PrimaryFingerprint, Created: selection.ExpectedReleaseDate, HashAlgorithm: 8}}}
		signatures.Releases = append(signatures.Releases, release)
	}
	return policy, keys, signatures, pins, now
}

func cloneAssemblyDebian[T any](t *testing.T, value T) T {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if json.Unmarshal(data, &out) != nil {
		t.Fatal("fixture clone")
	}
	return out
}

func TestAssemblyDebianPolicyNormalizerPinsSourcesAndKeepsAbsentExpiry(t *testing.T) {
	source, config := assemblyDebianSourceFixture(t)
	policy, err := normalizeAssemblyDebianPolicy(source, config, "policy-1")
	if err != nil || len(policy.Roots) != 3 || len(policy.Releases) != 6 || policy.KeyObservationMaxAgeSeconds != 86400 {
		t.Fatal(policy, err)
	}
	for _, selection := range policy.Releases {
		if selection.CandidateID == "current/bookworm" && (selection.ExpectedValidUntil != nil || selection.Role != "exact-point-anchor") {
			t.Fatal("missing expiry/latest protection invented")
		}
		if selection.CandidateID == "current/bookworm-updates" && selection.ExpectedValidUntil != nil {
			t.Fatal("missing upstream expiry invented")
		}
	}
	if policy.HistoricalAnchor.CompressedLayerRef.SHA256 == strings.TrimPrefix(policy.HistoricalAnchor.BootstrapConfigDigest, "sha256:") || policy.HistoricalAnchor.CompressedLayerRef.SHA256 == policy.HistoricalAnchor.DebuerreotypeRootfsArchiveSHA256 {
		t.Fatal("base identities conflated")
	}
	for _, which := range []string{"policy", "config", "revision"} {
		t.Run(which, func(t *testing.T) {
			a, b := append([]byte{}, source...), append([]byte{}, config...)
			revision := "policy-1"
			switch which {
			case "policy":
				a[len(a)-1] ^= 1
			case "config":
				b[len(b)-1] ^= 1
			case "revision":
				revision = "bad\nrevision"
			}
			if _, err := normalizeAssemblyDebianPolicy(a, b, revision); err == nil {
				t.Fatal("unpinned source normalized")
			}
		})
	}
}

func TestAssemblyDebianKeySummaryBoundIdentityRevocationAndTime(t *testing.T) {
	policy, keys, _, pins, now := assemblyDebianSummaryFixture(t)
	if err := checkAssemblyDebianKeys(keys, policy, pins, now); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*assemblyDebianKeyObservationsV1){
		"stale":                      func(v *assemblyDebianKeyObservationsV1) { v.ObservedAt = now - 86401 },
		"future":                     func(v *assemblyDebianKeyObservationsV1) { v.ObservedAt = now + 301 },
		"revoked":                    func(v *assemblyDebianKeyObservationsV1) { v.Keys[0].SnapshotValidity = "r" },
		"expired":                    func(v *assemblyDebianKeyObservationsV1) { v.Keys[0].Expires = &now },
		"unknown-global-revocations": func(v *assemblyDebianKeyObservationsV1) { v.UnseenRevocations = "all-currently-unrevoked" },
		"duplicate":                  func(v *assemblyDebianKeyObservationsV1) { v.Keys = append(v.Keys, v.Keys[0]) },
		"unbounded": func(v *assemblyDebianKeyObservationsV1) {
			for len(v.Keys) <= 32 {
				v.Keys = append(v.Keys, v.Keys[0])
			}
		},
		"foreign-policy": func(v *assemblyDebianKeyObservationsV1) { v.RootPolicy.Digest = strings.Repeat("9", 64) },
		"foreign-config": func(v *assemblyDebianKeyObservationsV1) { v.Producer.Config.SHA256 = strings.Repeat("9", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			changed := cloneAssemblyDebian(t, keys)
			mutate(&changed)
			if checkAssemblyDebianKeys(changed, policy, pins, now) == nil {
				t.Fatal("invalid key summary accepted")
			}
		})
	}
	keys.ObservedAt = now - 86400
	if checkAssemblyDebianKeys(keys, policy, pins, now) != nil {
		t.Fatal("exact finite observation boundary refused")
	}
}

func TestAssemblyDebianSignatureCoreBindsExactObjectsRolesAndExtras(t *testing.T) {
	policy, keys, signatures, pins, now := assemblyDebianSummaryFixture(t)
	if err := checkAssemblyDebianSignatures(signatures, policy, keys, pins, now); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*assemblyDebianReleaseSignaturesV1){
		"missing-object": func(v *assemblyDebianReleaseSignaturesV1) { v.Releases = v.Releases[:4] },
		"foreign-object": func(v *assemblyDebianReleaseSignaturesV1) {
			v.Releases[0].SignedRelease.SHA256 = strings.Repeat("9", 64)
		},
		"foreign-child":      func(v *assemblyDebianReleaseSignaturesV1) { v.Releases[0].ChildIndex = 8 },
		"no-valid-signature": func(v *assemblyDebianReleaseSignaturesV1) { v.Releases[0].Dispositions = nil },
		"unclassified":       func(v *assemblyDebianReleaseSignaturesV1) { v.Releases[0].Dispositions[0].Kind = "verified" },
		"weak-hash":          func(v *assemblyDebianReleaseSignaturesV1) { v.Releases[0].Dispositions[0].Valid.HashAlgorithm = 2 },
		"wrong-signer": func(v *assemblyDebianReleaseSignaturesV1) {
			v.Releases[0].Dispositions[0].Valid.Signer = v.Releases[0].Dispositions[0].Valid.Primary
		},
		"wrong-index":             func(v *assemblyDebianReleaseSignaturesV1) { v.Releases[0].Logical[0].SignedIndexes[0].Bytes++ },
		"producer-authority":      func(v *assemblyDebianReleaseSignaturesV1) { v.Releases[0].Logical[0].ServerEligibility = "eligible" },
		"nonzero-blanket-success": func(v *assemblyDebianReleaseSignaturesV1) { v.Releases[0].ExitCode = 2 },
	} {
		t.Run(name, func(t *testing.T) {
			changed := cloneAssemblyDebian(t, signatures)
			mutate(&changed)
			if checkAssemblyDebianSignatures(changed, policy, keys, pins, now) == nil {
				t.Fatal("invalid signature summary accepted")
			}
		})
	}
	extra := assemblyDebianSignatureDispositionV1{Kind: "unverified-extra-signature", UnverifiedExtra: &assemblyDebianUnverifiedExtraV1{KeyID: "0123456789ABCDEF", ErrorCode: 9}}
	signatures.Releases[0].Dispositions = append(signatures.Releases[0].Dispositions, extra)
	signatures.Releases[0].ExitCode = 2
	if checkAssemblyDebianSignatures(signatures, policy, keys, pins, now) != nil {
		t.Fatal("exact valid+unknown-extra disposition refused")
	}
	signatures.Releases[0].Dispositions[1].UnverifiedExtra.ErrorCode = 1
	if checkAssemblyDebianSignatures(signatures, policy, keys, pins, now) == nil {
		t.Fatal("other error became unverified extra")
	}
}

func TestAssemblyDebianHistoricalBindingAndExpiryDoNotGrantCurrentEligibility(t *testing.T) {
	policy, keys, signatures, pins, now := assemblyDebianSummaryFixture(t)
	for _, selection := range policy.Releases {
		if selection.Role == "historical-reproducible-anchor" {
			if checkAssemblyDebianReleaseTimeUse(selection, policy, now) != nil {
				t.Fatal("exact historical expiry policy refused")
			}
			if selection.ExpectedValidUntil != nil && *selection.ExpectedValidUntil > now {
				t.Fatal("fixture historical expiry expected before observation")
			}
		} else if selection.Role == "current-updates" || selection.Role == "current-security" {
			if checkAssemblyDebianReleaseTimeUse(selection, policy, now) != nil || checkAssemblyDebianReleaseTimeUse(selection, policy, selection.ExpectedReleaseDate+172801) == nil {
				t.Fatal("current freshness bound bypassed")
			}
		}
	}
	for name, mutate := range map[string]func(*assemblyDebianLogicalReleaseV1){
		"v3-era":         func(v *assemblyDebianLogicalReleaseV1) { v.Era = "historical" },
		"v3-null-anchor": func(v *assemblyDebianLogicalReleaseV1) { v.HistoricalAnchor = nil; v.ProducerHistoricalTimestamp = nil },
		"wrong-snapshot": func(v *assemblyDebianLogicalReleaseV1) {
			v.HistoricalAnchor.SnapshotSourceRef.SHA256 = strings.Repeat("9", 64)
		},
		"config-as-layer": func(v *assemblyDebianLogicalReleaseV1) {
			v.HistoricalAnchor.CompressedLayerRef.SHA256 = strings.TrimPrefix(v.HistoricalAnchor.BootstrapConfigDigest, "sha256:")
		},
		"history-as-current": func(v *assemblyDebianLogicalReleaseV1) {
			v.Era = "current"
			v.ProducerFreshness = "current-window-with-expiry"
		},
		"expiry-at-observation-as-anchor": func(v *assemblyDebianLogicalReleaseV1) {
			state := "expired-at-anchor"
			v.HistoricalExpiryState = &state
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := cloneAssemblyDebian(t, signatures)
			mutate(&changed.Releases[2].Logical[0])
			if checkAssemblyDebianSignatures(changed, policy, keys, pins, now) == nil {
				t.Fatal("historical mismatch accepted")
			}
		})
	}
	keys.Keys[0].SnapshotValidity = "r"
	if checkAssemblyDebianSignatures(signatures, policy, keys, pins, now) == nil {
		t.Fatal("historical release waived current key revocation")
	}
}

func TestAssemblyDebianCanonicalSummaryReadsRejectUnknownTrailingAndOversize(t *testing.T) {
	policy, keys, signatures, pins, now := assemblyDebianSummaryFixture(t)
	keyData, _ := json.Marshal(keys)
	signatureData, _ := json.Marshal(signatures)
	if _, err := decodeAssemblyDebianKeySummary(keyData, policy, pins, now); err != nil {
		t.Fatal(err)
	}
	if _, err := decodeAssemblyDebianSignatureSummary(signatureData, policy, keys, pins, now); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{append(append([]byte{}, signatureData...), []byte(" {}")...), []byte(strings.Repeat("x", assemblyCatalogRecordLimit+1)), append([]byte(`{"verified":true,`), signatureData[1:]...), append([]byte(`{"schema":1,`), signatureData[1:]...)} {
		if _, err := decodeAssemblyDebianSignatureSummary(data, policy, keys, pins, now); err == nil {
			t.Fatal("noncanonical summary accepted")
		}
	}
	// A successful synthetic summary is still not native/owned evidence. No
	// capture constructor, adopted record writer or full closure exists here.
}
