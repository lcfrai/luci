from pathlib import Path
import hashlib,json,re,shutil
B=Path(__file__).parent;O=B/'publication/cloud072-late-failure-diagnostic-20261009';assert not O.exists();O.mkdir(parents=True)
def meta(p):
 b=p.read_bytes();return {'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
originals=[]
for v in ['warm','cold']:
 for n in ['launch.mjs','scenario.mjs','outer-run.py']:
  old=B/'before'/v/n;upstream=Path('/workspace/.luci-tools/cloud071-browser-restart-diagnostic')/v/n
  assert old.read_bytes()==upstream.read_bytes()
  if n!='launch.mjs':assert old.read_bytes()==(B/v/n).read_bytes()
  originals.append({'path':str(upstream),'sha256':meta(upstream)['sha256'],'bytes':meta(upstream)['bytes'],'unchanged':True})
for directory in ['before','warm','cold','evidence']:
 shutil.copytree(B/directory,O/directory)
for name in ['README.md','CLOUD-072.patch','warm-launch.mjs.diff','cold-launch.mjs.diff','source-binding.json','failure.test.mjs','run-tests.py','runs.json','report.json','prepare.py','publish.py']:
 shutil.copyfile(B/name,O/name)
(O/'.gitattributes').write_text('# Preserve captured helper/test/receipt bytes.\n** -text\n')
(O/'original-immutability.json').write_text(json.dumps({'sources':originals,'scope':'Six selected original helper files byte-identical to preserved C071 sources. Original actual cold fixture and its private files were not read or modified by C072.'},indent=2)+'\n')
patterns={'private-key':rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----','github-token':rb'(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{50,})','openai-token':rb'\bsk-(?:proj-)?[A-Za-z0-9_-]{30,}','bearer-value':rb'(?i)bearer\s+[A-Za-z0-9_.-]{24,}'}
findings=[]
for p in O.rglob('*'):
 if not p.is_file():continue
 for label,pattern in patterns.items():
  if re.search(pattern,p.read_bytes()):findings.append({'path':str(p.relative_to(O)),'pattern':label})
assert not findings
assert not list(O.rglob('private-first-failure.json'))
(O/'privacy-scan.json').write_text(json.dumps({'boundedPatternFindings':findings,'rawPrivateFailureFileAbsent':True,'scope':'Controlled helper/test source and TAP only. Synthetic private-writer fixtures removed by their test; no real private logs/errors/auth/request bodies read or included.'},indent=2)+'\n')
m={str(p.relative_to(O)):meta(p) for p in sorted(O.rglob('*')) if p.is_file()}
(O/'publication.json').write_text(json.dumps({'version':1,'stableID':'CLOUD-072','scope':'External helper correction with controlled original/final verification; no installed runtime validation','files':m},indent=2)+'\n')
(O/'SHA256SUMS').write_text(''.join(x['sha256']+'  '+n+'\n' for n,x in m.items())+meta(O/'publication.json')['sha256']+'  publication.json\n')
assert all(meta(O/n)==x for n,x in m.items())
print(json.dumps({'capsule':str(O),'files':len(m)+2,'bytes':sum(p.stat().st_size for p in O.rglob('*') if p.is_file()),'publicationSHA256':meta(O/'publication.json')['sha256'],'checksumsSHA256':meta(O/'SHA256SUMS')['sha256']},indent=2))
