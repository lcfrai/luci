from pathlib import Path
import hashlib,json,re,shutil
B=Path(__file__).parent;O=B/'publication/cloud073-teardown-read-cancellation-v2-20261009';assert not O.exists();O.mkdir(parents=True)
def meta(p):
 b=p.read_bytes();return dict(bytes=len(b),sha256=hashlib.sha256(b).hexdigest())
original=[]
for v in ['warm','cold']:
 for n in ['launch.mjs','scenario.mjs','outer-run.py']:
  p=Path('/workspace/.luci-tools/cloud073-teardown-route')/v/n;assert p.read_bytes()==(B/'before'/v/n).read_bytes();original.append(dict(source=str(p),original=meta(p),unchanged=True))
for n in ['before','warm','cold','support','evidence']:shutil.copytree(B/n,O/n)
for n in ['README.md','CLOUD-073-v2.patch','warm-launch.mjs.diff','cold-launch.mjs.diff','source-binding.json','route.test.mjs','run-tests.py','runs.json','report.json','prepare.py','publish.py']:shutil.copyfile(B/n,O/n)
(O/'.gitattributes').write_text('# Preserve helper/test/receipt bytes.\n** -text\n');(O/'original-immutability.json').write_text(json.dumps(dict(originals=original,scope='Six prior helper files unchanged. Actual private fixtures were not modified or read by publisher.'),indent=2)+'\n')
patterns={'private-key':rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----','github-token':rb'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})','openai-token':rb'\bsk-(?:proj-)?[A-Za-z0-9_-]{30,}','bearer-value':rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}'};findings=[]
for p in O.rglob('*'):
 if p.is_file():
  for k,v in patterns.items():
   if re.search(v,p.read_bytes()):findings.append(dict(path=str(p.relative_to(O)),pattern=k))
assert not findings and not list(O.rglob('private-first-failure.json'))
(O/'privacy-scan.json').write_text(json.dumps(dict(boundedPatternFindings=[],privateRawFilesAbsent=True,scope='Controlled code/TAP and existing public failure metadata only; no private raw logs/auth/first-error contents read or exported.'),indent=2)+'\n')
m={str(p.relative_to(O)):meta(p) for p in sorted(O.rglob('*')) if p.is_file()};(O/'publication.json').write_text(json.dumps(dict(version=1,stableID='CLOUD-073',scope='Prospective symmetric read cancellation; no installed acceptance',files=m),indent=2)+'\n');(O/'SHA256SUMS').write_text(''.join(v['sha256']+'  '+n+'\n' for n,v in m.items())+meta(O/'publication.json')['sha256']+'  publication.json\n');assert all(meta(O/n)==v for n,v in m.items());print(json.dumps(dict(capsule=str(O),files=len(m)+2,bytes=sum(p.stat().st_size for p in O.rglob('*') if p.is_file()),publicationSHA256=meta(O/'publication.json')['sha256'],checksumsSHA256=meta(O/'SHA256SUMS')['sha256']),indent=2))
