#!/usr/bin/env python3
"""Inventory and copy only frozen preparation evidence; never read installed output."""
from pathlib import Path
import datetime
import hashlib
import json
import re
import shutil

HERE = Path(__file__).resolve().parent
ANALYSIS = HERE.parent
CAPSULE = ANALYSIS / 'capsule'
V2 = Path('/workspace/.luci-tools/cloud032-summary-delivery-v2-20261009')
V1 = Path('/workspace/.luci-tools/cloud032-summary-delivery-diagnostic-20261009')

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def manifest_members(root, relative):
    path = root / relative
    data = json.loads(path.read_text())
    members = data['members']
    rows = [{'path': key, **value} for key, value in members.items()] if isinstance(members, dict) else members
    selected = {relative}
    for row in rows:
        part = Path(row['path'])
        assert not part.is_absolute() and '..' not in part.parts, (relative, 'unsafe member')
        member = path.parent / part
        assert member.is_file() and not member.is_symlink(), (relative, row['path'])
        assert member.stat().st_size == row['bytes'] and digest(member) == row['sha256'], (relative, row['path'], 'binding changed')
        selected.add(member.relative_to(root).as_posix())
    return selected, len(rows)

def ordinary_members(root):
    paths = set()
    for path in root.rglob('*'):
        assert not path.is_symlink(), ('symlink', root.name, path.relative_to(root).as_posix())
        assert path.is_file() or path.is_dir(), ('special file', root.name, path.relative_to(root).as_posix())
        if path.is_file():
            paths.add(path.relative_to(root).as_posix())
    return paths

closure = ANALYSIS / 'independent-closure.json'
closure_before = digest(closure)
v2_members, _ = manifest_members(V2, 'preparation-manifest.json')
v2_members.add('SHA256SUMS')
v1_members, _ = manifest_members(V1, 'preparation-manifest.json')
for proof in ('transport-proof', 'prebody-proof'):
    selected, _ = manifest_members(V1, proof + '/publication-manifest.json')
    v1_members.update(selected)
v1_members.update(('BLOCKER.md', 'status.json', 'summary-delivery.mjs'))
assert len(v2_members) == 176 and len(v1_members) == 69
assert v2_members == ordinary_members(V2)
assert v1_members == ordinary_members(V1)

manifest_checks = []
for root in (V2, V1):
    for path in sorted(root.rglob('*manifest*.json')):
        _, count = manifest_members(root, path.relative_to(root).as_posix())
        manifest_checks.append({'source': root.name, 'manifest': path.relative_to(root).as_posix(), 'verifiedMemberBindings': count})
checksum_count = 0
for line in (V2 / 'SHA256SUMS').read_text().splitlines():
    expected, relative = line.split(maxsplit=1)
    relative = relative.lstrip('*')
    assert digest(V2 / relative) == expected, ('checksum mismatch', relative)
    checksum_count += 1
assert checksum_count == 175

# These are indicators, not a substitute for the bounded content review recorded below.
indicators = {
    'literalCredentialAssignment': re.compile(r'''(?i)(?:accessToken|refreshToken|attachToken|apiKey|password|secret)\s*[:=]\s*["'][^"']{8,}["']'''),
    'literalBearerCredential': re.compile(r'(?i)Bearer\s+[A-Za-z0-9_./+=-]{12,}'),
    'privateKeyMaterial': re.compile(r'-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----'),
}
indicator_hits = []
inventory = []
for root, selected, destination in ((V2, v2_members, 'preparation-v2'), (V1, v1_members, 'prior-v1')):
    for relative in sorted(selected):
        path = root / relative
        data = path.read_bytes()
        text = data.decode('utf-8')
        for category, pattern in indicators.items():
            for match in pattern.finditer(text):
                indicator_hits.append({'source': root.name, 'path': relative, 'line': text.count('\n', 0, match.start()) + 1, 'category': category})
        inventory.append({'source': root.name, 'sourceRelativePath': relative, 'capsulePath': destination + '/' + relative,
                          'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()})
assert not indicator_hits, ('content review required', indicator_hits)

# Exclusive destination creation prevents replacement of another owner's files.
for destination in ('preparation-v2', 'prior-v1'):
    assert not (CAPSULE / destination).exists(), ('destination already exists', destination)
for destination in ('preparation-v2', 'prior-v1'):
    (CAPSULE / destination).mkdir()
for row in inventory:
    root = V2 if row['source'] == V2.name else V1
    source = root / row['sourceRelativePath']
    target = CAPSULE / row['capsulePath']
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, target)
    assert target.stat().st_size == row['bytes'] and digest(target) == row['sha256']
    assert source.stat().st_size == row['bytes'] and digest(source) == row['sha256']
assert digest(closure) == closure_before

counts = {}
for name in ('preparation-v2', 'prior-v1'):
    rows = [row for row in inventory if row['capsulePath'].startswith(name + '/')]
    counts[name] = {'members': len(rows), 'bytes': sum(row['bytes'] for row in rows)}
receipt = {
    'scope': 'Frozen external preparation, helper, controlled proof, and validation evidence only; no installed result read or copied.',
    'preparedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'sourceRoots': {'preparation-v2': str(V2), 'prior-v1': str(V1)},
    'capsuleDirectories': ['preparation-v2', 'prior-v1'],
    'counts': counts,
    'totalMembers': len(inventory), 'totalBytes': sum(row['bytes'] for row in inventory),
    'sourceAndDestinationHashesVerified': True,
    'sourceBytesUnchanged': True,
    'independentClosureSHA256Before': closure_before,
    'independentClosureSHA256After': digest(closure),
    'nestedManifestChecks': manifest_checks,
    'v2ChecksumBindingsVerified': checksum_count,
    'symlinksOrSpecialFiles': [],
    'literalCredentialIndicatorHits': indicator_hits,
    'privacyReview': {
        'method': 'Bounded text review of manifest/source bindings, small validation logs, proof result structure, and absolute-path/credential indicators across every selected UTF-8 file.',
        'findings': [
            'No actual installed fixture path, installed session token, credential literal, private key, or installed raw result was found in the selected frozen files.',
            'Absolute paths retained in helper source and bindings identify repository/preparation directories, distribution packages, or source placeholders; they are not copied private fixture directories.',
            'Proof origins, request IDs, bodies, and headers are controlled tiny loopback HTTP evidence. They do not originate from an installed application fixture.',
            'Source-seal package provenance and release-preservation files contain package/archive identity metadata. Expected image digest bindings are retained; Docker image-inspect metadata is excluded.',
            'Frozen helper source contains generic fixture creation and receipt code; code definitions do not establish that a private installed artifact is included.',
        ],
        'selectedFilesRequiringRedaction': [],
        'excludedArtifactClasses': [
            'Private installed driver result and raw stdout/stderr',
            'Private installed fixture trees, session journals, and environment material',
            'Installed browser profiles, caches, node_modules, and runtime state',
            'Private Docker image metadata',
        ],
        'excludedArtifactsHandling': 'Parent owns original-artifacts-metadata.json and the sanitized result projection; this inventory does not open or copy excluded artifacts.',
        'limits': 'This is a bounded evidence-publication review, not a claim that arbitrary future runtime output from these helpers is safe to publish.',
    },
    'evidenceInterpretation': [
        'Prior v1 transport proof, failed pre-body proof, blocker, and unrun draft remain byte-exact and retrievable.',
        'V2 controlled proof is separate from prior v1 checks; counts must not be added as cumulative unique checks.',
        'V2 mechanism concerns response availability and backpressure, not proof of frontend commit causality or preservation of upstream work.',
        'No tests, runtime, browser, installed experiment, repository mutation, or source rewrite was performed by this publication preparation.',
    ],
    'members': inventory,
}
(HERE / 'inventory-privacy-receipt.json').write_text(json.dumps(receipt, indent=2) + '\n')
(HERE / 'capsule-allowlist.txt').write_text(''.join(row['capsulePath'] + '\n' for row in inventory))
(HERE / 'capsule-SHA256SUMS').write_text(''.join(row['sha256'] + '  ' + row['capsulePath'] + '\n' for row in inventory))
print(json.dumps({'passed': True, 'counts': counts, 'totalMembers': len(inventory), 'totalBytes': receipt['totalBytes'], 'manifestBindingsVerified': sum(x['verifiedMemberBindings'] for x in manifest_checks), 'checksumBindingsVerified': checksum_count, 'sourceBytesUnchanged': True, 'closureUnchanged': True}, sort_keys=True))
