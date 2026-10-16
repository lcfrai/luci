package api

import (
	"reflect"
	"strings"
	"time"
)

// Pins come from the selected actual producer profile, never a public request.
// Core checks below validate untrusted summaries, NOT raw objects, native
// execution, cleanup or observed facts. The finite owned-capture constructor is
// a separate reviewed seam and is not implemented or called by these functions.
type assemblyDebianProducerPins struct {
	Product *assemblyDebianProductProfilePins
	SourceReceipt    assemblyDebianObjectRef
	Config           assemblyDebianObjectRef
	ProjectionSHA256 string
	GPGSHA256        string
	GPGVSHA256       string
	SignedObjects    []assemblyDebianObjectRef // fixed five-child verification order
}

func assemblyDebianOptionalEpochEqual(a, b *int64) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func assemblyDebianUpstreamExpiry(until *int64, at int64) string {
	if until == nil {
		return "absent"
	}
	if *until <= at {
		return "expired-at-observation"
	}
	return "within-observed-valid-until"
}

func assemblyDebianProducerFreshness(selection assemblyDebianReleaseSelectionV1, at int64, policy assemblyDebianRootPolicyV1) string {
	if selection.Role == "historical-reproducible-anchor" {
		return "historical-only-not-current"
	}
	if selection.Role == "exact-point-anchor" {
		return "exact-observed-point-release-no-latest-protection"
	}
	if at-selection.ExpectedReleaseDate > policy.CurrentReleaseMaxAgeSeconds || selection.ExpectedValidUntil != nil && *selection.ExpectedValidUntil <= at || selection.Role == "current-security" && selection.ExpectedValidUntil == nil {
		return "current-policy-refused"
	}
	if selection.ExpectedValidUntil == nil {
		return "current-window-expiry-absent"
	}
	return "current-window-with-expiry"
}

func checkAssemblyDebianLogical(value assemblyDebianLogicalReleaseV1, selection assemblyDebianReleaseSelectionV1, policy assemblyDebianRootPolicyV1, start, end int64) error {
	if value.CandidateID != selection.CandidateID || value.ReleaseDate != selection.ExpectedReleaseDate || !assemblyDebianOptionalEpochEqual(value.ValidUntil, selection.ExpectedValidUntil) || !reflect.DeepEqual(value.SignedIndexes, selection.SignedIndexes) || !assemblyDebianEpoch(value.ObservedAt) || value.ObservedAt < start || value.ObservedAt > end || value.SignatureState != "eligible-signature-verified" || value.KeyRevocationKnowledge != "unseen-revocations-not-established" || value.ServerEligibility != "not-evaluated-server-owned" || value.ProducerFreshness != assemblyDebianProducerFreshness(selection, value.ObservedAt, policy) || value.UpstreamExpiryState != assemblyDebianUpstreamExpiry(value.ValidUntil, value.ObservedAt) {
		return errAssemblyDebianEvidence
	}
	if selection.Role == "historical-reproducible-anchor" {
		if value.Era != "historical-base-anchor" || value.ProducerHistoricalTimestamp == nil || *value.ProducerHistoricalTimestamp != assemblyDebianSnapshotTimestamp || value.HistoricalAnchor == nil || *value.HistoricalAnchor != policy.HistoricalAnchor || value.HistoricalExpiryState == nil {
			return errAssemblyDebianEvidence
		}
		timestamp, err := time.Parse("20060102T150405Z", assemblyDebianSnapshotTimestamp)
		if err != nil || value.ReleaseDate > timestamp.Unix()+policy.FutureSkewSeconds {
			return errAssemblyDebianEvidence
		}
		state := "absent"
		if value.ValidUntil != nil {
			if *value.ValidUntil <= timestamp.Unix() {
				state = "expired-at-anchor"
			} else {
				state = "within-valid-until-at-anchor"
			}
		}
		if *value.HistoricalExpiryState != state {
			return errAssemblyDebianEvidence
		}
	} else if value.Era != "current" || value.ProducerHistoricalTimestamp != nil || value.HistoricalAnchor != nil || value.HistoricalExpiryState != nil {
		return errAssemblyDebianEvidence
	}
	return nil
}

// Source/policy-specific time use, deliberately distinct from cryptographic
// observation or overall eligibility. The caller still needs actual producer,
// raw-object guards, current installer/CAS and all remaining proof domains.
func checkAssemblyDebianReleaseTimeUse(selection assemblyDebianReleaseSelectionV1, policy assemblyDebianRootPolicyV1, now int64) error {
	if !assemblyDebianEpoch(now) || selection.ExpectedReleaseDate > now+policy.FutureSkewSeconds {
		return errAssemblyDebianEvidence
	}
	switch selection.Role {
	case "historical-reproducible-anchor":
		if selection.HistoricalTimestamp == nil || *selection.HistoricalTimestamp != assemblyDebianSnapshotTimestamp || policy.HistoricalAnchor != expectedAssemblyDebianHistoricalAnchor() {
			return errAssemblyDebianEvidence
		}
		timestamp, err := time.Parse("20060102T150405Z", *selection.HistoricalTimestamp)
		if err != nil || selection.ExpectedReleaseDate > timestamp.Unix()+policy.FutureSkewSeconds {
			return errAssemblyDebianEvidence
		}
		// Only signed RELEASE expiry may be historical; keys/signatures are
		// independently checked for present policy validity, never waived here.
		return nil
	case "exact-point-anchor":
		return nil // Explicitly never latest/update evidence.
	case "current-updates", "current-security":
		if assemblyDebianProducerFreshness(selection, now, policy) == "current-policy-refused" {
			return errAssemblyDebianEvidence
		}
		return nil
	default:
		return errAssemblyDebianEvidence
	}
}

func checkAssemblyDebianSignatures(value assemblyDebianReleaseSignaturesV1, policy assemblyDebianRootPolicyV1, keys assemblyDebianKeyObservationsV1, pins assemblyDebianProducerPins, now int64) error {
	if checkAssemblyDebianKeys(keys, policy, pins, now) != nil || value.Schema != 1 || value.RecordType != "luci-debian-release-signatures-v1" || checkAssemblyDebianProducer(value.Producer, pins) != nil || !reflect.DeepEqual(value.Producer,keys.Producer) || len(value.Releases) != 5 || len(pins.SignedObjects) != 5 {
		return errAssemblyDebianEvidence
	}
	policyRef, _ := assemblyDebianRecordRef("official-root-policy", policy)
	keyRef, err := assemblyDebianRecordRef("key-observations", keys)
	if err != nil || value.RootPolicy != policyRef || value.KeyObservations != keyRef {
		return errAssemblyDebianEvidence
	}
	knownKeys, knownIDs, roots := map[string]assemblyDebianKeyV1{}, map[string]bool{}, map[string]assemblyDebianRootV1{}
	for _, key := range keys.Keys {
		knownKeys[key.Fingerprint], knownIDs[key.Fingerprint[24:]] = key, true
	}
	for _, root := range policy.Roots {
		roots[root.PrimaryFingerprint] = root
	}
	selections := map[string]assemblyDebianReleaseSelectionV1{}
	for _, selection := range policy.Releases {
		selections[selection.CandidateID] = selection
	}
	seenObjects, seenLogical := map[string]bool{}, map[string]bool{}
	for i, release := range value.Releases {
		if release.SignedRelease != pins.SignedObjects[i] || !checkAssemblyDebianObject(release.SignedRelease, "signed-release", 262144) || !checkAssemblyDebianObject(release.SignedPlaintext, "signed-plaintext", 262144) || !checkAssemblyDebianObject(release.MachineStatus, "signature-status", 262144) || release.ChildIndex != 4+i || !assemblyDebianEpoch(release.ObservedStart) || !assemblyDebianEpoch(release.ObservedEnd) || release.ObservedEnd < release.ObservedStart || release.ObservedEnd-release.ObservedStart > 62 || release.ObservedEnd > now+policy.FutureSkewSeconds || release.ObservedStart < keys.ObservedAt-1 || len(release.Dispositions) < 1 || len(release.Dispositions) > 16 || len(release.Logical) < 1 || len(release.Logical) > 2 || seenObjects[release.SignedRelease.SHA256] {
			return errAssemblyDebianEvidence
		}
		seenObjects[release.SignedRelease.SHA256] = true
		allowed := map[string]bool{}
		for _, logical := range release.Logical {
			selection, exists := selections[logical.CandidateID]
			if !exists || selection.SignedRelease != release.SignedRelease || seenLogical[logical.CandidateID] || checkAssemblyDebianLogical(logical, selection, policy, release.ObservedStart, release.ObservedEnd) != nil {
				return errAssemblyDebianEvidence
			}
			for _, id := range selection.AllowedRootIDs {
				allowed[id] = true
			}
			seenLogical[logical.CandidateID] = true
		}
		valid, extras := 0, 0
		for _, disposition := range release.Dispositions {
			switch disposition.Kind {
			case "eligible-valid-signature":
				v := disposition.Valid
				if v == nil || disposition.UnverifiedExtra != nil || !assemblyDebianFingerprint(v.Signer, 40) || !assemblyDebianFingerprint(v.Primary, 40) || !assemblyDebianEpoch(v.Created) || v.Created > release.Logical[0].ObservedAt+policy.FutureSkewSeconds || v.Expires != nil && (!assemblyDebianEpoch(*v.Expires) || *v.Expires <= v.Created || *v.Expires <= now || *v.Expires <= release.ObservedEnd) || v.HashAlgorithm != 8 && v.HashAlgorithm != 9 && v.HashAlgorithm != 10 {
					return errAssemblyDebianEvidence
				}
				key, exists := knownKeys[v.Signer]
				root, approved := roots[v.Primary]
				if !exists || !approved || !allowed[root.ID] || key.PrimaryFingerprint != v.Primary || key.Created > v.Created || key.Expires != nil && *key.Expires <= v.Created || !strings.Contains(strings.ToLower(key.Capabilities), "s") {
					return errAssemblyDebianEvidence
				}
				selected := root.PrimaryFingerprint
				if root.SigningSubkeyFingerprint != nil {
					selected = *root.SigningSubkeyFingerprint
				}
				if v.Signer != selected || !((key.Algorithm == 1 || key.Algorithm == 3) && key.Bits >= 3072 || key.Algorithm == 22 && key.Bits == 255) {
					return errAssemblyDebianEvidence
				}
				valid++
			case "unverified-extra-signature":
				v := disposition.UnverifiedExtra
				if v == nil || disposition.Valid != nil || !assemblyDebianFingerprint(v.KeyID, 16) || v.ErrorCode != 9 || knownIDs[v.KeyID] {
					return errAssemblyDebianEvidence
				}
				extras++
			default:
				return errAssemblyDebianEvidence
			}
		}
		if valid == 0 || extras == 0 && release.ExitCode != 0 || extras > 0 && release.ExitCode != 2 {
			return errAssemblyDebianEvidence
		}
	}
	if len(seenObjects) != 5 || len(seenLogical) != 6 {
		return errAssemblyDebianEvidence
	}
	_, err = assemblyDebianRecordRef("release-signatures", value)
	return err
}

// Canonical read helpers still validate summaries only. A successful decode is
// never a substitute for the missing owned raw-object/receipt capture step.
func decodeAssemblyDebianKeySummary(data []byte, policy assemblyDebianRootPolicyV1, pins assemblyDebianProducerPins, now int64) (assemblyDebianKeyObservationsV1, error) {
	var value assemblyDebianKeyObservationsV1
	if decodeAssemblyCatalog(data, assemblyCatalogRecordLimit, &value) != nil || checkAssemblyDebianKeys(value, policy, pins, now) != nil {
		return assemblyDebianKeyObservationsV1{}, errAssemblyDebianEvidence
	}
	return value, nil
}

func decodeAssemblyDebianSignatureSummary(data []byte, policy assemblyDebianRootPolicyV1, keys assemblyDebianKeyObservationsV1, pins assemblyDebianProducerPins, now int64) (assemblyDebianReleaseSignaturesV1, error) {
	var value assemblyDebianReleaseSignaturesV1
	if decodeAssemblyCatalog(data, assemblyCatalogRecordLimit, &value) != nil || checkAssemblyDebianSignatures(value, policy, keys, pins, now) != nil {
		return assemblyDebianReleaseSignaturesV1{}, errAssemblyDebianEvidence
	}
	return value, nil
}

func checkAssemblyDebianProducer(value assemblyDebianProducerV1, pins assemblyDebianProducerPins) error {
	if value.ProductOwner!=nil||pins.Product!=nil{return checkAssemblyDebianProductProducer(value,pins)}
	if value.Profile != "luci-retained-debian-signature-input-evidence-v2" || value.SourceReceipt != pins.SourceReceipt || value.Config != pins.Config || value.Config != (assemblyDebianObjectRef{"producer-config", assemblyDebianConfigSHA, 41817}) || value.ProjectionSHA256 != assemblyDebianProjectionSHA || value.ProjectionSHA256 != pins.ProjectionSHA256 || value.GPGSHA256 != pins.GPGSHA256 || value.GPGVSHA256 != pins.GPGVSHA256 || !checkAssemblyDebianObject(value.SourceReceipt, "producer-source-receipt", 65536) || !checkAssemblyDebianObject(value.Config, "producer-config", 128<<10) || !checkAssemblyDebianObject(value.TypedOutput, "producer-output", 256<<10) || !checkAssemblyDebianObject(value.InnerReceipt, "inner-receipt", 256<<10) || !checkAssemblyDebianObject(value.OuterReceipt, "outer-receipt", 256<<10) || !assemblyCatalogHex(value.ProjectionSHA256) || !assemblyCatalogHex(value.GPGSHA256) || !assemblyCatalogHex(value.GPGVSHA256) {
		return errAssemblyDebianEvidence
	}
	return nil
}
func validAssemblyDebianProductPins(p assemblyDebianProductProfilePins)bool{
	for _,ref:=range []assemblyDebianObjectRef{p.Profile,p.SourceReceipt,p.Config,p.Entry,p.Projection,p.Python,p.GPG,p.GPGV}{if strings.Trim(ref.SHA256,"0")==""{return false}}
	return checkAssemblyDebianObject(p.Profile,"product-profile",64<<10)&&checkAssemblyDebianObject(p.SourceReceipt,"producer-source-receipt",64<<10)&&p.SourceReceipt.SHA256!=assemblyDebianSourceReceiptSHA&&checkAssemblyDebianObject(p.Config,"producer-config",128<<10)&&p.Config.SHA256!=assemblyDebianConfigSHA&&checkAssemblyDebianObject(p.Entry,"projection-entry",64<<10)&&p.Projection==(assemblyDebianObjectRef{"projection-source",assemblyDebianProjectionSHA,15522})&&checkAssemblyDebianObject(p.Python,"cp-python",16<<20)&&checkAssemblyDebianObject(p.GPG,"gpg",16<<20)&&checkAssemblyDebianObject(p.GPGV,"gpgv",16<<20)
}
func checkAssemblyDebianProductProducer(v assemblyDebianProducerV1,pins assemblyDebianProducerPins)error{
	p:=pins.Product;o:=v.ProductOwner
	if p==nil||o==nil||!validAssemblyDebianProductPins(*p)||v.Profile!=assemblyDebianProductRawSchema||v.SourceReceipt!=p.SourceReceipt||v.SourceReceipt!=pins.SourceReceipt||v.Config!=p.Config||v.Config!=pins.Config||v.ProjectionSHA256!=p.Projection.SHA256||v.ProjectionSHA256!=pins.ProjectionSHA256||v.GPGSHA256!=p.GPG.SHA256||v.GPGSHA256!=pins.GPGSHA256||v.GPGVSHA256!=p.GPGV.SHA256||v.GPGVSHA256!=pins.GPGVSHA256||!checkAssemblyDebianObject(v.TypedOutput,"producer-output",256<<10)||v.InnerReceipt!=(assemblyDebianObjectRef{})||v.OuterReceipt!=(assemblyDebianObjectRef{})||o.Protocol!=assemblyDebianProductProtocol||o.Profile!=p.Profile||o.Entry!=p.Entry||o.Python!=p.Python||!checkAssemblyDebianObject(o.Receipt,"product-native-owner-receipt",64<<10){return errAssemblyDebianEvidence};return nil
}

func checkAssemblyDebianProducerPolicyFamily(policy assemblyDebianRootPolicyV1,pins assemblyDebianProducerPins)bool{
	if pins.Product==nil{return policy.ProducerProtocol==""&&policy.ProducerProfile==nil}
	return validAssemblyDebianProductPins(*pins.Product)&&policy.ProducerProtocol==assemblyDebianProductProtocol&&policy.ProducerProfile!=nil&&*policy.ProducerProfile==pins.Product.Profile&&policy.ProducerConfig==pins.Product.Config
}

func assemblyDebianEpoch(value int64) bool { return value > 0 && value <= 4102444800 }

func checkAssemblyDebianKeys(value assemblyDebianKeyObservationsV1, policy assemblyDebianRootPolicyV1, pins assemblyDebianProducerPins, now int64) error {
	if !checkAssemblyDebianProducerPolicyFamily(policy,pins)||checkAssemblyDebianPolicyShape(policy) != nil || !assemblyDebianEpoch(now) || value.Schema != 1 || value.RecordType != "luci-debian-key-observations-v1" || !assemblyDebianEpoch(value.ObservedAt) || value.ObservedAt > now+policy.FutureSkewSeconds || value.ObservedAt < now-policy.KeyObservationMaxAgeSeconds || value.UnseenRevocations != "not-established" || checkAssemblyDebianProducer(value.Producer, pins) != nil || !checkAssemblyDebianObject(value.KeyListing, "key-listing", 65536) || len(value.Snapshots) != 3 || len(value.Keys)<3 || len(value.Keys) > 32 {
		return errAssemblyDebianEvidence
	}
	policyRef, err := assemblyDebianRecordRef("official-root-policy", policy)
	if err != nil || policyRef != value.RootPolicy {
		return errAssemblyDebianEvidence
	}
	primaries := map[string]assemblyDebianRootV1{}
	for i, root := range policy.Roots {
		primaries[root.PrimaryFingerprint] = root
		snapshot := value.Snapshots[i]
		if snapshot.RootID != root.ID || snapshot.PublicKey != root.PublicKey || !checkAssemblyDebianObject(snapshot.DerivedKeyring, "keyring", 32768) {
			return errAssemblyDebianEvidence
		}
	}
	keys := map[string]assemblyDebianKeyV1{}
	previous, primaryCount := "", 0
	for _, key := range value.Keys {
		if !assemblyDebianFingerprint(key.Fingerprint, 40) || !assemblyDebianFingerprint(key.PrimaryFingerprint, 40) || key.Fingerprint <= previous || !assemblyDebianEpoch(key.Created) || key.Created > value.ObservedAt+policy.FutureSkewSeconds || key.Created > now+policy.FutureSkewSeconds || key.Expires != nil && (!assemblyDebianEpoch(*key.Expires) || *key.Expires <= key.Created || *key.Expires <= now || *key.Expires <= value.ObservedAt) || key.Algorithm < 1 || key.Algorithm > 100 || key.Bits < 1 || key.Bits > 16384 || len(key.Capabilities) == 0 || len(key.Capabilities) > 32 || strings.Trim(key.Capabilities, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ?") != "" || strings.Contains(key.Capabilities, "D") {
			return errAssemblyDebianEvidence
		}
		switch key.SnapshotValidity {
		case "-", "?", "q", "n", "m", "f", "u", "w", "s", "o":
		default:
			return errAssemblyDebianEvidence
		}
		if _, exists := primaries[key.PrimaryFingerprint]; !exists {
			return errAssemblyDebianEvidence
		}
		switch key.Kind {
		case "pub":
			if key.Fingerprint != key.PrimaryFingerprint {
				return errAssemblyDebianEvidence
			}
			primaryCount++
		case "sub":
			if key.Fingerprint == key.PrimaryFingerprint {
				return errAssemblyDebianEvidence
			}
		default:
			return errAssemblyDebianEvidence
		}
		keys[key.Fingerprint], previous = key, key.Fingerprint
	}
	if primaryCount != 3 {
		return errAssemblyDebianEvidence
	}
	for primary, root := range primaries {
		pub, exists := keys[primary]
		if !exists || pub.Kind != "pub" {
			return errAssemblyDebianEvidence
		}
		selected := primary
		if root.SigningSubkeyFingerprint != nil {
			selected = *root.SigningSubkeyFingerprint
		}
		key, exists := keys[selected]
		if !exists || key.PrimaryFingerprint != primary || selected != primary && key.Kind != "sub" || !strings.Contains(strings.ToLower(key.Capabilities), "s") {
			return errAssemblyDebianEvidence
		}
	}
	_, err = assemblyDebianRecordRef("key-observations", value)
	return err
}
