"""Pure bounded GnuPG machine-output projection; no effects on import."""
import email.utils
import datetime
import hashlib
import re


class Refusal(ValueError):
    pass


def need(condition, code):
    if not condition:
        raise Refusal(code)


def fingerprint(value):
    need(bool(re.fullmatch('[0-9A-F]{40}', value)), 'fingerprint_shape')
    return value


def text(data, maximum):
    need(isinstance(data, bytes) and len(data) <= maximum, 'machine_output_bound')
    try:
        value = data.decode('utf-8', 'strict')
    except UnicodeError:
        raise Refusal('machine_output_encoding') from None
    need('\x00' not in value and len(value.splitlines()) <= 2048, 'machine_output_lines')
    return value


def keys(data, expected, now):
    """Derive identity/selected validity, without inferring unseen revocations."""
    records, primary, pending = {}, None, None
    for line in text(data, 65536).splitlines():
        fields = line.split(':')
        tag = fields[0]
        need(tag in {'pub', 'sub', 'fpr', 'uid', 'uat', 'grp', 'tru', 'rvk'}, 'key_record_type')
        if tag in {'pub', 'sub'}:
            need(pending is None and len(fields) >= 12, 'key_record_order')
            need(fields[1] not in {'r', 'e', 'i', 'd'}, 'unusable_key')
            need(fields[5].isdigit() and (not fields[6] or fields[6].isdigit()), 'key_dates')
            created, expires = int(fields[5]), int(fields[6] or '0')
            need(created <= now + 300 and (not expires or expires > now), 'key_time')
            pending = dict(kind=tag, created=created, expires=expires, algorithm=fields[3],
                           bits=fields[2], capabilities=fields[11], validity=fields[1])
        elif tag == 'fpr':
            need(pending is not None and len(fields) >= 10, 'fingerprint_order')
            fpr = fingerprint(fields[9])
            need(fpr not in records, 'duplicate_fingerprint')
            if pending['kind'] == 'pub':
                primary = fpr
                need(primary in expected, 'unexpected_primary')
            need(primary is not None, 'subkey_without_primary')
            pending.update(fingerprint=fpr, primary=primary)
            records[fpr], pending = pending, None
    need(pending is None, 'missing_fingerprint')
    need({f for f, r in records.items() if r['kind'] == 'pub'} == set(expected), 'primary_set')
    for root, subkey in expected.items():
        if subkey:
            need(subkey in records and records[subkey]['primary'] == root and
                 records[subkey]['kind'] == 'sub', 'selected_subkey_binding')
    return records


def signatures(data, exit_code, records, eligible, now):
    """Each NEWSIG has one classified disposition; no blanket nonzero success."""
    groups, current = [], None
    auxiliary = {'KEY_CONSIDERED', 'SIG_ID', 'TRUST_UNDEFINED', 'TRUST_NEVER',
                 'TRUST_MARGINAL', 'TRUST_FULLY', 'TRUST_ULTIMATE'}
    for line in text(data, 262144).splitlines():
        need(line.startswith('[GNUPG:] '), 'status_prefix')
        parts = line[9:].split()
        need(bool(parts), 'empty_status')
        tag, args = parts[0], parts[1:]
        if tag == 'NEWSIG':
            current = {'valid': [], 'good': [], 'error': [], 'missing': []}
            groups.append(current)
            need(len(groups) <= 16, 'signature_count')
            continue
        need(current is not None, 'status_before_signature')
        if tag in auxiliary:
            if tag == 'KEY_CONSIDERED':
                need(len(args) == 2 and bool(re.fullmatch('[0-9A-F]{40}', args[0])) and args[1].isdigit(), 'key_considered_shape')
            continue
        need(tag in {'VALIDSIG', 'GOODSIG', 'ERRSIG', 'NO_PUBKEY'}, 'unclassified_or_bad_signature')
        current[{'VALIDSIG': 'valid', 'GOODSIG': 'good', 'ERRSIG': 'error', 'NO_PUBKEY': 'missing'}[tag]].append(args)
    need(bool(groups), 'no_signatures')
    accepted, unknown, dispositions = 0, 0, []
    expected_ids = {f[-16:] for f in records}
    for group in groups:
        if group['valid']:
            need(len(group['valid']) == len(group['good']) == 1 and not group['error'] and not group['missing'], 'mixed_signature_status')
            v = group['valid'][0]
            need(len(v) == 10, 'validsig_shape')
            signer, primary = fingerprint(v[0]), fingerprint(v[9])
            need(signer in records and records[signer]['primary'] == primary and primary in eligible, 'ineligible_signer')
            need(v[2].isdigit() and v[3].isdigit(), 'signature_dates')
            created, expires = int(v[2]), int(v[3])
            key = records[signer]
            need(key['created'] <= created <= now + 300 and (not key['expires'] or created < key['expires']) and
                 (not expires or expires > now), 'signature_time')
            need(v[4] == '4' and v[5] == '0' and v[7] in {'8', '9', '10'} and v[8] == '01', 'signature_algorithm_or_class')
            need((v[6] in {'1', '3'} and key['algorithm'] in {'1', '3'} and key['bits'].isdigit() and int(key['bits']) >= 3072) or
                 (v[6] == key['algorithm'] == '22' and key['bits'] == '255'), 'public_key_algorithm')
            good = group['good'][0]
            need(len(good) >= 2 and good[0] == signer[-16:], 'goodsig_identity')
            accepted += 1
            dispositions.append(dict(state='eligible-valid-signature', signer=signer, primary=primary,
                                     created=created, expires=expires, hashAlgorithm=v[7]))
        else:
            need(not group['good'] and len(group['error']) == len(group['missing']) == 1, 'unclassified_signature')
            error, missing = group['error'][0], group['missing'][0]
            need(len(error) in {6, 7} and len(missing) == 1 and
                 bool(re.fullmatch('[0-9A-F]{16}', error[0])) and error[0] == missing[0] and
                 error[0] not in expected_ids and error[5] == '9', 'unknown_extra_pair')
            need(all(error[i].isdigit() for i in (1, 2, 4)) and bool(re.fullmatch('[0-9A-Fa-f]{2}', error[3])), 'unknown_extra_shape')
            if len(error) == 7 and error[6] != '-':
                need(fingerprint(error[6]) not in records and error[6][-16:] == error[0], 'unknown_extra_fingerprint')
            unknown += 1
            dispositions.append(dict(state='unverified-extra-signature', keyID=error[0], errorCode=9))
    need(accepted > 0, 'no_eligible_valid_signature')
    need(exit_code == (2 if unknown else 0), 'exit_status_mismatch')
    return dispositions


def historical_context(data, selected):
    """Derive the timestamp from exact pinned source bytes, not an injected date.

    Root/layer/config references are configured subjects, not authenticated base
    provenance. This read-only projection never grants historical eligibility.
    """
    need(selected.get('schema') == 1, 'historical_context_schema')
    source = selected['snapshotSource']
    need(source['role'] == 'base-rootfs-debian-sources-snapshot' and
         isinstance(source['bytes'], int) and 0 < source['bytes'] <= 4096 and
         bool(re.fullmatch('[a-f0-9]{64}', source['sha256'])), 'snapshot_source_binding')
    need(len(data) == source['bytes'] and hashlib.sha256(data).hexdigest() == source['sha256'],
         'snapshot_source_identity')
    sections, current = [], {}
    for line in text(data, 4096).splitlines() + ['']:
        if not line:
            if current:
                sections.append(current)
                current = {}
            continue
        need(': ' in line, 'snapshot_source_field')
        key, value = line.split(': ', 1)
        need(key not in current, 'snapshot_source_duplicate')
        current[key] = value
    archives, timestamps = {}, set()
    for section in sections:
        need(set(section) == {'Types', 'URIs', 'Suites', 'Components', 'Signed-By'} and
             section['Types'] == 'deb' and section['Components'] == 'main' and
             section['Signed-By'] == '/usr/share/keyrings/debian-archive-keyring.gpg',
             'snapshot_source_scope')
        match = re.fullmatch(r'http://snapshot\.debian\.org/archive/(debian|debian-security)/(\d{8}T\d{6}Z)', section['URIs'])
        need(match is not None, 'snapshot_source_origin')
        archive, timestamp = match.groups()
        timestamps.add(timestamp)
        for suite in section['Suites'].split():
            need(suite not in archives, 'snapshot_suite_duplicate')
            archives[suite] = archive
    need(archives == {'bookworm': 'debian', 'bookworm-updates': 'debian',
                      'bookworm-security': 'debian-security'} and len(timestamps) == 1,
         'snapshot_source_suites')
    timestamp = next(iter(timestamps))
    need(timestamp == selected['snapshotTimestamp'], 'snapshot_timestamp_binding')
    try:
        epoch = int(datetime.datetime.strptime(timestamp, '%Y%m%dT%H%M%SZ').replace(
            tzinfo=datetime.timezone.utc).timestamp())
    except ValueError:
        raise Refusal('snapshot_timestamp_shape') from None
    for field in ('trustPolicySha256', 'dependencyLockSha256'):
        need(bool(re.fullmatch('[a-f0-9]{64}', selected[field])), 'historical_policy_binding')
    root = selected['baseSubject']
    need(bool(re.fullmatch('sha256:[a-f0-9]{64}', root['bootstrapConfigDigest'])) and
         bool(re.fullmatch('[a-f0-9]{64}', root['debuerreotypeRootfsArchiveSha256'])) and
         root['compressedLayer']['role'] == 'oci-compressed-base-layer' and
         bool(re.fullmatch('[a-f0-9]{64}', root['compressedLayer']['sha256'])) and
         isinstance(root['compressedLayer']['bytes'], int) and 0 < root['compressedLayer']['bytes'] <= 2 << 30,
         'historical_base_subject')
    need(set(selected['releaseBindings']) == set(archives), 'historical_release_set')
    return dict(selected, snapshotTimestamp=timestamp, snapshotEpoch=epoch, archives=archives)


def release(data, candidates, now, historical=None, snapshot=None):
    """Use only gpgv-produced signed plaintext after signature classification."""
    fields, in_sha, hashes = {}, False, {}
    for line in text(data, 262144).splitlines():
        if line.startswith(' '):
            if in_sha:
                parts = line.split()
                need(len(parts) == 3 and bool(re.fullmatch('[a-f0-9]{64}', parts[0])) and parts[1].isdigit(), 'signed_hash_shape')
                need(parts[2] not in hashes, 'duplicate_signed_hash')
                hashes[parts[2]] = (int(parts[1]), parts[0])
            continue
        need(':' in line, 'signed_field_shape')
        key, value = line.split(':', 1)
        need(key not in fields, 'duplicate_signed_field')
        fields[key] = value.strip()
        in_sha = key == 'SHA256'
    def epoch(value):
        try:
            parsed = email.utils.parsedate_to_datetime(value)
            need(parsed.tzinfo is not None, 'release_timezone')
            return int(parsed.timestamp())
        except (TypeError, ValueError, OverflowError):
            raise Refusal('release_date') from None
    date = epoch(fields.get('Date', ''))
    until = epoch(fields['Valid-Until']) if 'Valid-Until' in fields else None
    need(date <= now + 300 and (until is None or until > date), 'release_time_order')
    results, bound = [], None
    for candidate in candidates:
        era = candidate['era']
        need(isinstance(era, str) and era in {'current', 'historical-base-anchor'}, 'candidate_era')
        # Bind exact advertised fields as well as hash tuples, not a suite guess.
        for field in candidate['fields']:
            key, value = field.split(':', 1)
            need(fields.get(key) == value.strip(), 'candidate_signed_field_mismatch')
        for index in candidate['indexCandidates']:
            need(hashes.get(index['path']) == (index['bytes'], index['sha256']), 'candidate_signed_index_mismatch')
        is_historical = era == 'historical-base-anchor'
        name = candidate['id'].split('/')[-1]
        need(candidate['id'] == era + '/' + name and name in {'bookworm', 'bookworm-updates', 'bookworm-security'} and
             fields.get('Codename') == name, 'candidate_era_identity')
        anchor = None
        if is_historical:
            need(historical is not None and isinstance(snapshot, bytes), 'candidate_historical_context')
            if bound is None:
                bound = historical_context(snapshot, historical)
            need(candidate.get('historicalAnchorID') == bound['id'] and
                 name in bound['archives'], 'candidate_historical_context')
            binding = bound['releaseBindings'][name]
            need(candidate['releaseUrl'] == binding['releaseUrl'] and candidate['sha256'] == binding['inputSha256'] and
                 candidate['allowedRootIDs'] == binding['allowedRootIDs'], 'candidate_historical_release_binding')
            prefix = ('https://snapshot.debian.org/archive/' + bound['archives'][name] + '/' +
                      bound['snapshotTimestamp'] + '/dists/' + name + '/')
            for index in candidate['indexCandidates']:
                parent = index['path'].rsplit('/', 1)[0]
                need(index['url'] == prefix + index['path'] and
                     index['byHashUrl'] == prefix + parent + '/by-hash/SHA256/' + index['sha256'],
                     'candidate_historical_index_origin')
            need(date <= bound['snapshotEpoch'] + 300, 'historical_release_after_snapshot')
            anchor = {key: bound[key] for key in ('id', 'snapshotTimestamp', 'trustPolicySha256', 'dependencyLockSha256')}
            anchor['snapshotSourceRef'] = {key: bound['snapshotSource'][key] for key in ('role', 'bytes', 'sha256')}
            anchor.update(bootstrapConfigDigest=bound['baseSubject']['bootstrapConfigDigest'],
                          compressedLayerRef=bound['baseSubject']['compressedLayer'],
                          debuerreotypeRootfsArchiveSha256=bound['baseSubject']['debuerreotypeRootfsArchiveSha256'])
            freshness = 'historical-only-not-current'
        elif candidate.get('historicalAnchorID') is not None:
            raise Refusal('current_candidate_historical_anchor')
        elif name == 'bookworm':
            freshness = 'exact-observed-point-release-no-latest-protection'
        elif now - date > 172800 or until is not None and until <= now or name == 'bookworm-security' and until is None:
            freshness = 'current-policy-refused'
        else:
            freshness = 'current-window-with-expiry' if until is not None else 'current-window-expiry-absent'
        results.append(dict(candidateID=candidate['id'], releaseDate=date, validUntil=until,
                            signatureState='eligible-signature-verified', freshness=freshness,
                            signedIndexes=candidate['indexCandidates'], era=era, observedAt=now,
                            historicalTimestamp=bound['snapshotTimestamp'] if is_historical else None,
                            historicalAnchor=anchor,
                            upstreamExpiryState='absent' if until is None else 'expired-at-observation' if until <= now else 'within-observed-valid-until',
                            historicalExpiryState=None if not is_historical else 'absent' if until is None else 'expired-at-anchor' if until <= bound['snapshotEpoch'] else 'within-valid-until-at-anchor',
                            keyRevocationKnowledge='unseen-revocations-not-established',
                            serverEligibility='not-evaluated-server-owned'))
    return results
