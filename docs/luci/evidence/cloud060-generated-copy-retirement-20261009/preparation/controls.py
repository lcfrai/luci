#!/usr/bin/env python3
import importlib.util,tempfile,json,hashlib,time,os
from pathlib import Path
B=Path(__file__).parent
spec=importlib.util.spec_from_file_location('retirement',B/'retire.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
m.DUE=time.monotonic()+30
results=[]
def test(name,fn):
 try:fn();results.append({'name':name,'passed':True})
 except BaseException as e:results.append({'name':name,'passed':False,'error':repr(e)});raise

def pair(p):
 a=p/'copy';b=p/'reference';a.mkdir();b.mkdir()
 for d in(a,b):
  (d/'web').mkdir();(d/'web/assets').mkdir();(d/'luci').write_bytes(b'test binary');(d/'web/assets/test.js').write_bytes(b'test script');(d/'web/index.html').write_bytes(b'test html')
 files,dirs=m.tree_inventory(a);copy={'outputs':{n:{'metadata':v,**m.hashfile(a/n)}for n,v in files.items()},'directoryMetadata':dirs}
 return a,b,copy

def successful():
 with tempfile.TemporaryDirectory(prefix='c060-generated-control-')as t:
  a,b,c=pair(Path(t));h=a/'.holding';m.link_payload(a/'web',h,b/'web');assert(a/'web').is_symlink();m.remove_exact_payload(h,'web',c);assert not h.exists()and(a/'web/index.html').read_bytes()==b'test html'and(a/'luci').read_bytes()==b'test binary';assert(b/'web/assets/test.js').read_bytes()==b'test script'
def binary():
 with tempfile.TemporaryDirectory(prefix='c060-generated-control-')as t:
  a,b,c=pair(Path(t));h=a/'.holding';m.link_payload(a/'luci',h,b/'luci');m.remove_exact_payload(h,'luci',c);assert not h.exists()and(a/'luci').read_bytes()==b'test binary'and(a/'web/index.html').is_file()
def rollback():
 with tempfile.TemporaryDirectory(prefix='c060-generated-control-')as t:
  a,b,c=pair(Path(t));h=a/'.holding';old=m.os.symlink
  def refusal(*a,**kw):raise OSError('inert link refusal')
  m.os.symlink=refusal
  try:
   try:m.link_payload(a/'web',h,b/'web');assert False
   except OSError:pass
  finally:m.os.symlink=old
  assert(a/'web/index.html').read_bytes()==b'test html'and not(a/'web').is_symlink()and not h.exists()
def unexpected():
 with tempfile.TemporaryDirectory(prefix='c060-generated-control-')as t:
  a,b,c=pair(Path(t));h=a/'.holding';m.link_payload(a/'web',h,b/'web');(h/'unexpected').write_bytes(b'preserve')
  try:m.remove_exact_payload(h,'web',c);assert False
  except AssertionError:pass
  assert(h/'index.html').read_bytes()==b'test html'and(h/'unexpected').read_bytes()==b'preserve'
def changed_binary():
 with tempfile.TemporaryDirectory(prefix='c060-generated-control-')as t:
  a,b,c=pair(Path(t));h=a/'.holding';m.link_payload(a/'luci',h,b/'luci');h.write_bytes(b'changed')
  try:m.remove_exact_payload(h,'luci',c);assert False
  except AssertionError:pass
  assert h.read_bytes()==b'changed'and(b/'luci').read_bytes()==b'test binary'
def linked_tree_refusal():
 with tempfile.TemporaryDirectory(prefix='c060-generated-control-')as t:
  a,b,c=pair(Path(t));(a/'web/foreign').symlink_to(b/'luci')
  try:m.tree_inventory(a);assert False
  except AssertionError:pass
  assert(b/'luci').read_bytes()==b'test binary'
def timeout():
 old=m.DUE;m.DUE=time.monotonic()-1
 try:
  try:m.bound();assert False
  except TimeoutError:pass
 finally:m.DUE=old
for n,f in [('exact-web-retirement-preserves-reference-and-binary',successful),('exact-binary-retirement-preserves-reference-and-web',binary),('symlink-creation-failure-rolls-original-back',rollback),('unexpected-file-refuses-before-file-unlink',unexpected),('changed-binary-metadata-refuses-unlink',changed_binary),('symlink-child-refuses-inventory',linked_tree_refusal),('elapsed-budget-refuses',timeout)]:test(n,f)
out={'sourceSHA256':hashlib.sha256((B/'retire.py').read_bytes()).hexdigest(),'passed':True,'cases':len(results),'results':results,'scope':'Tiny disposable filesystem controls only; no candidate package, fixture, runtime, Docker, process signalling or real retirement executed.'}
(B/'controls-result.json').write_text(json.dumps(out,indent=2)+'\n');print(json.dumps(out,indent=2))
