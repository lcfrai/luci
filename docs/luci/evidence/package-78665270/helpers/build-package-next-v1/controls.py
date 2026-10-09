#!/usr/bin/env python3
import importlib.util,hashlib,json,tempfile,sys
from pathlib import Path
B=Path(__file__).parent;OLD='5f4a966f6d70264c92ee2176dde912e1b43fc3c7';NEW='6'*40

def load(path):
 s=importlib.util.spec_from_file_location(path.stem,path);m=importlib.util.module_from_spec(s);s.loader.exec_module(m);return m
m=load(B/'build-package.py');original=load(B/'build-package.original.py');cases=[]
def hashes(root):return {str(p.relative_to(root)):hashlib.sha256(p.read_bytes()).hexdigest()for p in root.rglob('*')if p.is_file()}
def fixture(p):
 r=p/'releases/0.5.0';r.mkdir(parents=True);out=p/'receipts';out.mkdir();(p/'source').mkdir();(p/'source/unchanged.go').write_bytes(b'owned source sentinel')
 payload=b'fake package archive';(r/'luci.tar.gz').write_bytes(payload)
 manifest={'sourceCommit':OLD,'sourceDirty':False,'artifacts':[{'name':'luci.tar.gz','bytes':len(payload),'sha256':hashlib.sha256(payload).hexdigest()}]};(r/'release-manifest.json').write_text(json.dumps(manifest)+'\n');(r/'SHA256SUMS').write_text(manifest['artifacts'][0]['sha256']+'  luci.tar.gz\n');return r,out

def record(n,f):
 try:f();cases.append({'name':n,'passed':True})
 except BaseException as e:cases.append({'name':n,'passed':False,'error':str(e)});raise

def transition():
 with tempfile.TemporaryDirectory(prefix='package-transition-control-')as d:
  p=Path(d);r,o=fixture(p);before=hashes(r);source=hashes(p/'source');receipt=m.preserve_release(r,o,NEW,OLD);dest=Path(receipt['preservedPath']);assert dest.name=='0.5.0-5f4a966f-preserved-before-66666666'and not r.exists()and hashes(dest)==before and hashes(p/'source')==source;assert receipt['oldSource']==OLD and receipt['newSource']==NEW and receipt['status']=='preserved-verified'

def refuse(kind):
 with tempfile.TemporaryDirectory(prefix='package-refusal-control-')as d:
  p=Path(d);r,o=fixture(p);expected=OLD;new=NEW
  if kind=='wrong-prior':expected='7'*40
  if kind=='dirty-prior':
   f=r/'release-manifest.json';v=json.loads(f.read_text());v['sourceDirty']=True;f.write_text(json.dumps(v))
  if kind=='hash-mismatch':(r/'luci.tar.gz').write_bytes(b'changed')
  if kind=='destination-collision':(r.with_name('0.5.0-5f4a966f-preserved-before-66666666')).mkdir()
  if kind=='invalid-prior':expected='5f4a966f'
  if kind=='same-source':new=OLD
  if kind=='linked-release':
   target=r.with_name('actual');r.rename(target);r.symlink_to(target,target_is_directory=True)
  before=hashes(p);rejected=False
  try:m.preserve_release(r,o,new,expected)
  except AssertionError:rejected=True
  assert rejected and hashes(p)==before and not list(o.iterdir())

def dirty_source():
 with tempfile.TemporaryDirectory(prefix='package-dirty-control-')as d:
  p=Path(d);r,o=fixture(p);before=hashes(p);root=m.ROOT;args=sys.argv;call=m.subprocess.check_output;launch=m.subprocess.Popen;m.ROOT=p
  def fake(command,**kw):
   if command[:2]==['git','rev-parse']:return NEW+'\n'
   if command[:2]==['git','status']:return ' M tracked.go\n'
   raise AssertionError('Unexpected subprocess request')
  def forbidden(*a,**kw):raise AssertionError('No build launch permitted')
  m.subprocess.check_output=fake;m.subprocess.Popen=forbidden;sys.argv=['build-package.py','--expected-source',NEW,'--expected-previous-source',OLD,'--window-granted'];rejected=False
  try:
   try:m.main()
   except AssertionError as e:rejected=str(e)=='Dirty source'
  finally:m.ROOT=root;m.subprocess.check_output=call;m.subprocess.Popen=launch;sys.argv=args
  assert rejected and hashes(p)==before and not(p/'artifacts').exists()

def original_5f_refusal():
 with tempfile.TemporaryDirectory(prefix='package-original-control-')as d:
  p=Path(d);r,o=fixture(p);before=hashes(p);rejected=False
  try:original.preserve_release(r,o,NEW)
  except AssertionError as e:rejected=str(e)=='Prior release identity mismatch'
  assert rejected and hashes(p)==before
record('original-fixed-edd0-wrapper-refuses-current5f-before-rename',original_5f_refusal)
record('exact5f-to-new-preserves-all-manifest-artifacts-and-source',transition)
for kind in ['wrong-prior','dirty-prior','hash-mismatch','destination-collision','invalid-prior','same-source','linked-release']:record(kind,lambda k=kind:refuse(k))
record('dirty-current-source-refuses-before-output-or-build',dirty_source)
r={'sourceSHA256':hashlib.sha256((B/'build-package.py').read_bytes()).hexdigest(),'originalSHA256':hashlib.sha256((B/'build-package.original.py').read_bytes()).hexdigest(),'passed':True,'cases':len(cases),'results':cases,'scope':'Disposable fake package/source directories and fake subprocess responses only. No actual package build, release rename, Git mutation, runtime or Docker call.'};(B/'controls-result.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps(r,indent=2))
