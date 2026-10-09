#!/usr/bin/env python3
import importlib.util,tempfile,json,hashlib,time,sys
from pathlib import Path
P=Path(__file__).parent;source=Path(sys.argv[1]);out=Path(sys.argv[2]);spec=importlib.util.spec_from_file_location('reviewed',source);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);m.DUE=time.monotonic()+20
results=[]
def run(n,f):
 try:f();results.append({'case':n,'passed':True})
 except BaseException as e:results.append({'case':n,'passed':False,'error':str(e)[:200]})
def fsync_failure(nth):
 with tempfile.TemporaryDirectory(prefix='c060-stage-control-')as t:
  p=Path(t);original=p/'original';holding=p/'holding';target=p/'target';original.write_bytes(b'owned generated');target.write_bytes(b'exact retained');before=m.meta(original);old=m.fsyncdir;calls=0
  def maybe(p):
   nonlocal calls
   calls+=1
   if calls==nth:raise OSError('controlled directory sync failure')
  m.fsyncdir=maybe
  try:
   try:m.link_payload(original,holding,target)
   except OSError:pass
  finally:m.fsyncdir=old
  assert original.exists()and not original.is_symlink()and m.meta(original)==before and original.read_bytes()==b'owned generated'and not holding.exists(),'Original not restored after controlled fsync failure'
  assert target.read_bytes()==b'exact retained'
def visible_busy_then_denied():
 class Leaf:
  def __init__(self,n):self.n=n
  def read_text(self):return '203 (fixture) '+' '.join(['S']+['0']*18+['1521'])
  def read_bytes(self):return str(m.ROOT/m.NAMES[0]/'luci').encode()
 class Entry:
  name='203'
  def __truediv__(self,n):return Leaf(n)
 class Proc:
  def iterdir(self):return iter([Entry()])
 oldp=m.Path;oldlink=m.os.readlink
 def link(p):
  if p.n=='cwd':raise PermissionError('controlled unknown cwd')
  return str(m.ROOT/m.NAMES[0]/'luci')
 m.Path=lambda _:Proc();m.os.readlink=link
 try:
  rejected=False
  try:m.processes()
  except AssertionError:rejected=True
  assert rejected,'Already observed busy reference was discarded by later unreadable field'
 finally:m.Path=oldp;m.os.readlink=oldlink
run('fsync-after-rename-restores-current-payload',lambda:fsync_failure(1));run('fsync-after-link-restores-current-payload',lambda:fsync_failure(2));run('readable-busy-reference-wins-over-known-unknown-cwd',visible_busy_then_denied)
r={'sourceSHA256':hashlib.sha256(source.read_bytes()).hexdigest(),'cases':3,'passed':sum(x['passed']for x in results),'failed':sum(not x['passed']for x in results),'results':results,'scope':'Disposable tiny file objects and fake proc fields only; no package/path retirement or real process signals.'};out.write_text(json.dumps(r,indent=2)+'\n');print(json.dumps(r,indent=2));sys.exit(0 if r['failed']==0 else 1)
