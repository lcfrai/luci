#!/usr/bin/env python3
from pathlib import Path
import importlib.util,tempfile,hashlib,json,os,sys
P=Path(__file__).parent;results=[]
def load():
 s=importlib.util.spec_from_file_location('relocation',P/'relocate.py');m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m

def case(kind):
 m=load()
 with tempfile.TemporaryDirectory(prefix='old-archive-placement-control-')as t:
  r=Path(t)/'root';r.mkdir();dst=Path(t)/'tmp';dst.mkdir();m.ROOT=r;m.SOURCES=(r/'runtime-archive-7e1b1f608cfa/verified-luci-sandbox-dev.tar.gz',r/'runtime-archive-1f845cfd2b45/old-luci-sandbox-dev.tar.gz');m.TARGET=dst/'retained.tar.gz';m.PROTECTED=(r/'bootstrap1',r/'bootstrap2');data=b'controlled immutable archive bytes';m.SIZE=len(data);m.EXPECTED=hashlib.sha256(data).hexdigest();m.FLOOR=100;m.GUARD=64
  for p in m.SOURCES:p.parent.mkdir(parents=True,exist_ok=True)
  m.SOURCES[0].write_bytes(data);m.SOURCES[0].chmod(0o600);os.link(m.SOURCES[0],m.SOURCES[1])
  for p in m.PROTECTED:p.write_bytes(b'protected bootstrap')
  rp=[r/'runtime-archive-7e1b1f608cfa/receipt.json',r/'runtime-archive-1f845cfd2b45/receipt.json']
  rp[0].write_text(json.dumps({'archiveVerification':{'archiveSHA256':m.EXPECTED,'archiveBytes':m.SIZE,'candidateImage':'sha256:42597a985b20f2aee31a629a320980f02180bde307b6ffdd3ec60b257a82af5b'}}));rp[1].write_text(json.dumps({'preservedBefore':{'archiveSHA256':m.EXPECTED,'archiveBytes':m.SIZE}}))
  receipts=[p.read_bytes()for p in rp];protected=[m.met(p)for p in m.PROTECTED]
  if kind=='bad-hash':m.EXPECTED='0'*64
  if kind=='distinct-inode':m.SOURCES[1].unlink();m.SOURCES[1].write_bytes(data);m.SOURCES[1].chmod(0o600)
  if kind=='low-space':m.free=lambda p:m.SIZE+m.FLOOR+m.GUARD-1
  if kind in ('first-link-fsync','second-link-fsync'):
   chosen=m.SOURCES[0 if kind=='first-link-fsync'else 1].parent;original=m.fsyncdir;calls=0
   def sync(p):
    nonlocal calls
    if p==chosen:
     calls+=1
     if calls==2:raise OSError('controlled post-replace sync refusal')
    original(p)
   m.fsyncdir=sync
  oldargv=sys.argv;out=r/'historical425-relocation-operation-control';sys.argv=['relocate.py','--window-granted','--output',str(out)];failed=False
  try:
   try:m.main()
   except (AssertionError,OSError):failed=True
  finally:sys.argv=oldargv
  assert [p.read_bytes()for p in rp]==receipts and[m.met(p)for p in m.PROTECTED]==protected
  if kind=='success':
   assert not failed and all(p.is_symlink()and p.read_bytes()==data for p in m.SOURCES)and m.TARGET.read_bytes()==data and m.met(m.TARGET)['mode']==0o600;assert any(p.name.endswith('-final.json')for p in out.iterdir())
  elif kind in('first-link-fsync','second-link-fsync'):
   assert failed and m.TARGET.read_bytes()==data and all(p.read_bytes()==data for p in m.SOURCES);assert any(p.name.endswith('-failure.json')for p in out.iterdir())
  else:assert failed and not m.TARGET.exists()and all(not p.is_symlink()and p.read_bytes()==data for p in m.SOURCES)
for kind in['success','bad-hash','distinct-inode','low-space','first-link-fsync','second-link-fsync']:
 case(kind);results.append({'name':kind,'passed':True})
r={'sourceSHA256':hashlib.sha256((P/'relocate.py').read_bytes()).hexdigest(),'passed':True,'cases':len(results),'results':results,'scope':'Exact helper with tiny disposable roots/bytes and inert low-space or fsync failures; no actual archive read/hash/copy, runtime, Docker or production path mutation.'};(P/'controls-result.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps(r,indent=2))
