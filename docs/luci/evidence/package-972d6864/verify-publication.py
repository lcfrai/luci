#!/usr/bin/env python3
from pathlib import Path
import hashlib,json
P=Path(__file__).resolve().parent
sha=lambda raw:hashlib.sha256(raw).hexdigest()
read=lambda rel:json.loads((P/rel).read_text())
listed={line[66:]:line[:64] for line in (P/'SHA256SUMS').read_text().splitlines()}
actual={p.relative_to(P).as_posix():sha(p.read_bytes()) for p in P.rglob('*') if p.is_file() and p!=P/'SHA256SUMS'}
assert listed==actual,'Incomplete inventory or byte mismatch'
for row in read('original-bindings.json')['originals']:
 raw=(P/row['publishedPath']).read_bytes()
 assert len(raw)==row['bytes'] and sha(raw)==row['sha256'],row['publishedPath']
result=read('receipts/build-result.json');integrity=read('receipts/package-integrity.json');provenance=read('receipts/package-provenance.json');release=read('release/release-manifest.json')
commit='972d6864a30b742b12545c8e12f4ca3ae2faa884'
assert all(r['sourceCommit']==commit for r in [result,integrity,provenance,release])
assert result['exitCode']==0 and result['cleanBefore'] and result['cleanAfter'] and result['sameHead'] and not result['timedOut']
assert result['helperSHA256']==sha((P/'helpers/build-package.py').read_bytes())
assert (integrity['verifiedExtractedFiles'],integrity['licenseComponents'],integrity['missingLicenses'],integrity['allMembersMatched'])==(965,347,0,True)
assert release['sourceDirty'] is False and release['nativeRequested'] is False
subjects={r['name'].split('/')[-1]:r for r in integrity['subjects']}
for item in release['artifacts']:
 assert item['bytes']==subjects[item['name']]['bytes'] and item['sha256']==subjects[item['name']]['sha256']
raw=(P/'release/release-manifest.json').read_bytes()
assert len(raw)==subjects['release-manifest.json']['bytes'] and sha(raw)==subjects['release-manifest.json']['sha256']
assert provenance['archiveSHA256']==subjects['luci-linux-amd64.tar.gz']['sha256']
for row in read('source-bindings.json')['inputs']:
 raw=(P/'selected-source'/row['path']).read_bytes()
 assert len(raw)==row['bytes'] and sha(raw)==row['sha256']
assert read('original-immutability.json')['allOriginalsUnchanged'] is True
print(json.dumps({'filesVerified':len(actual),'bytes':sum((P/r).stat().st_size for r in actual),'sourceCommit':commit,'originalBuildExit':0,'originalVerifiedMembers':965,'largePayloadsRead':False,'nativeExecuted':False},indent=2))
