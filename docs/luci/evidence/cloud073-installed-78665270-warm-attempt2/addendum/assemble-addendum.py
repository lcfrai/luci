from pathlib import Path
import hashlib,json,re,os
B=Path(__file__).resolve().parent;O=B/'addendum';assert not O.exists();os.umask(0o077)
S=Path('/workspace/.luci-tools/cloud073-installed-78665270');sha=lambda b:hashlib.sha256(b).hexdigest();enc=lambda o:(json.dumps(o,indent=2)+'\n').encode()
inputs=[('evidence/warm-attempt2/private-error-status-projection.json','private-error-status-projection.json','ea963a36a3e2121a34516ad942ec7029f826ac3455ceb081233b59b5fa571ce4'),('evidence/warm-attempt2/retained-allocation-projection.json','retained-allocation-projection.json','ca9302b95985567100b0c54cde55ef3fc382c49dfa96bffda3ccffed3968f935'),('project-warm2-private-error-status.py','sources/project-warm2-private-error-status.py',None),('project-warm2-retained-allocation.py','sources/project-warm2-retained-allocation.py',None)]
O.mkdir();rows=[]
for src,dst,pin in inputs:
 p=S/src;assert p.is_file() and not p.is_symlink();raw=p.read_bytes();assert len(raw)<65536
 if pin:assert sha(raw)==pin
 q=O/dst;q.parent.mkdir(parents=True,exist_ok=True);q.write_bytes(raw);rows.append({'source':str(p),'publishedPath':dst,'bytes':len(raw),'sha256':sha(raw),'representation':'exact-safe-projection' if p.suffix=='.json' else 'exact-extractor-source'})
status=json.loads((O/'private-error-status-projection.json').read_bytes());allocation=json.loads((O/'retained-allocation-projection.json').read_bytes())
assert len(status['rows'])==14 and all(x['httpStatus']==409 and x['errorBytes']==63 and not x['rawTextRetained'] for x in status['rows'])
assert allocation['container']['status']=='created' and allocation['container']['running'] is False and allocation['lease']['state']=='backend-dispatching' and allocation['privateInputsUnchanged']
assert allocation['registry']['status']=='starting' and not allocation['registry']['endpointPresent'] and not allocation['registry']['sandboxRefPresent']
(O/'README.md').write_text('''# Warm attempt2 — additive read-only classifications

This supplement preserves the frozen [failed-run capsule](../README.md); its publication SHA256 is `724c345a4f94075e22d2cd73073ef19281226d16ce0d56ada73a45d2001ac41d`. No base receipt, source or outcome is rewritten.

The authorized diagnostic reader produced14 safe rows: seven CDP and seven Playwright observations, each an anchored Chromium unexpected-handshake status409 classification. Every source error is63bytes with SHA256 `84f76179cded37834529a365d836b2bdcdce89f74bf27c4a4d5c2b1426895f45`. This is **error-text classification only**. No raw URL/query/token/error, captured HTTP response body, server reason or successful Restart acknowledgement is provided. It does not establish a cause for the still-pending POST.

The later exact retained-allocation projection observes the selected container **created, not running**, the registry **starting** without endpoint/reference, and its lease **backend-dispatching**. Immutable container ID, name, image, managed/instance/session/sandbox labels and project checks match. One Restart remains pending. Endpoint/token values, environment, mount sources, commands and private registry/ledger contents are excluded; only allowlisted metadata and hashes are exported. Selected private inputs remained unchanged. This verifies selected ownership, not daemon-internal quiescence, remote acknowledgement or successful cleanup.

Exact safe projections and their reader source are included. Those readers performed the original authorized read-only inspection; publication copied their outputs and source without executing them or reading raw private errors/fixture state. The original sources remain bound in `input-bindings.json`. No runtime, mutation, repair, retry or acceptance upgrade was performed. `sha256sum -c SHA256SUMS` verifies this supplement independently of the frozen base.
''')
(O/'input-bindings.json').write_bytes(enc({'version':1,'inputs':rows,'basePublicationSHA256':'724c345a4f94075e22d2cd73073ef19281226d16ce0d56ada73a45d2001ac41d','scope':'Safe outputs and exact readers only; publisher did not execute readers or read their private inputs.'}))
(O/'.gitattributes').write_bytes(b'** -text\n')
(O/'assemble-addendum.py').write_bytes(Path(__file__).read_bytes())
patterns=[rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----',rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}',rb'https?://[^/@\s:]+:[^/@\s]+@']
for p in O.rglob('*'):
 if p.is_file():assert not any(re.search(t,p.read_bytes()) for t in patterns),str(p)
for row in rows:assert sha(Path(row['source']).read_bytes())==row['sha256']
files={str(p.relative_to(O)):{'bytes':p.stat().st_size,'sha256':sha(p.read_bytes())} for p in sorted(O.rglob('*')) if p.is_file()}
(O/'publication.json').write_bytes(enc({'version':1,'files':files,'originalsUnchanged':True,'scope':'Additive status-text and retained-owned-allocation projections; original warm failure remains unchanged.'}))
(O/'SHA256SUMS').write_text(''.join(v['sha256']+'  '+k+'\n' for k,v in files.items())+sha((O/'publication.json').read_bytes())+'  publication.json\n')
for line in (O/'SHA256SUMS').read_text().splitlines():
 digest,name=line.split('  ',1);assert sha((O/name).read_bytes())==digest
report={'path':str(O),'files':len(files)+2,'publicationSHA256':sha((O/'publication.json').read_bytes()),'checksumsSHA256':sha((O/'SHA256SUMS').read_bytes())};(B/'addendum-result.json').write_bytes(enc(report));print(json.dumps(report,indent=2))
