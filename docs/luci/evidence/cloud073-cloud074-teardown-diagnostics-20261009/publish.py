from pathlib import Path
import hashlib,json,re,shutil
B=Path(__file__).parent;O=B/'publication/cloud073-cloud074-teardown-diagnostics-20261009';assert not O.exists();O.mkdir(parents=True)
def meta(p):
 b=p.read_bytes();return dict(bytes=len(b),sha256=hashlib.sha256(b).hexdigest())
originals=[]
for v in ['warm','cold']:
 for n in ['launch.mjs','scenario.mjs','outer-run.py']:
  p=Path('/workspace/.luci-tools/cloud072-late-failure-diagnostic')/v/n;assert p.read_bytes()==(B/'before'/v/n).read_bytes();originals.append(dict(source=str(p),original=meta(p),unchanged=True))
for row in json.loads((B/'final-source-bindings.json').read_text()):assert meta(B/row['variant']/row['name'])==row['final']
for name in ['before','before-c074','counterexample-untracked','warm','cold','support','evidence','disposal-evidence','race-evidence','intermediate21-evidence','transport-triage']:shutil.copytree(B/name,O/name)
for n in ['README.md','CLOUD-073.patch','CLOUD-073-074.patch','warm-launch.mjs.diff','cold-launch.mjs.diff','warm-c074-launch.mjs.diff','cold-c074-launch.mjs.diff','source-binding.json','c074-launcher-bindings.json','final-source-bindings.json','report.json','route.test.mjs','disposal-boundary.test.mjs','run-tests.py','run-disposal-tests.py','runs.json','disposal-runs.json','intermediate21-runs.json','prepare.py','prepare-c074-launchers.py','publish.py']:shutil.copyfile(B/n,O/n)
(O/'.gitattributes').write_text('# Preserve helper/test/receipt bytes across platforms.\n** -text\n')
(O/'original-immutability.json').write_text(json.dumps(dict(originals=originals,scope='Six C072 helper originals unchanged; no original failed runtime fixture read or modified by this source capsule.'),indent=2)+'\n')
patterns={'private-key':rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----','github-token':rb'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})','openai-token':rb'\bsk-(?:proj-)?[A-Za-z0-9_-]{30,}','bearer-value':rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}'}
findings=[]
for p in O.rglob('*'):
 if not p.is_file():continue
 for k,v in patterns.items():
  if re.search(v,p.read_bytes()):findings.append(dict(path=str(p.relative_to(O)),pattern=k))
assert findings==[]
assert not list(O.rglob('private-first-failure.json')) and not list(O.rglob('private-controller.stderr')) and not list(O.rglob('private-transport-errors.jsonl'))
(O/'privacy-scan.json').write_text(json.dumps(dict(boundedPatternFindings=[],actualPrivateFilesAbsent=True,scope='Controlled source/test/receipt capsule only; no real private error/auth/controller/transport contents read or exported. Common credential pattern scan, not exhaustive detection.'),indent=2)+'\n')
m={str(p.relative_to(O)):meta(p) for p in sorted(O.rglob('*')) if p.is_file()}
(O/'publication.json').write_text(json.dumps(dict(version=1,stableIDs=['CLOUD-073','CLOUD-074'],scope='External helper candidate; controlled before/final evidence, no installed candidate acceptance',files=m),indent=2)+'\n')
(O/'SHA256SUMS').write_text(''.join(v['sha256']+'  '+n+'\n' for n,v in m.items())+meta(O/'publication.json')['sha256']+'  publication.json\n')
assert all(meta(O/n)==v for n,v in m.items())
print(json.dumps(dict(capsule=str(O),files=len(m)+2,bytes=sum(p.stat().st_size for p in O.rglob('*') if p.is_file()),publicationSHA256=meta(O/'publication.json')['sha256'],checksumsSHA256=meta(O/'SHA256SUMS')['sha256']),indent=2))
