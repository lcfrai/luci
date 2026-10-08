import os,json,hashlib,subprocess,time,pathlib,datetime,shutil
root=pathlib.Path('/workspace/.luci-tools/cloud033-cleanup-message'); repo=pathlib.Path('/workspace/luci'); tree=root/'frontend-tree'; out=root/'frontend-evidence'; out.mkdir(exist_ok=True)
if tree.exists(): raise SystemExit('Refusing to overwrite frontend evidence tree')
tree.mkdir(); shutil.copytree(repo/'desktop/src',tree/'src')
for source in (repo/'desktop').iterdir():
 if source.is_file() and (source.suffix in ('.json','.ts','.js','.mjs','.cjs') or source.name.startswith('.')): shutil.copy2(source,tree/source.name)
(tree/'node_modules').symlink_to(repo/'desktop/node_modules',target_is_directory=True)
component='desktop/src/components/journey/ProjectChecks.tsx'; test='desktop/src/components/journey/ProjectChecks.test.tsx'
shutil.copy2(root/'after'/test,tree/'src/components/journey/ProjectChecks.test.tsx')
def stamp():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def bind():
 rows=[]
 for path in sorted(tree.rglob('*')):
  if not path.is_file() or path.is_symlink() or 'node_modules' in path.parts or '.vite' in path.parts: continue
  data=path.read_bytes(); rows.append({'path':'desktop/'+str(path.relative_to(tree)),'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data)})
 return {'capturedAt':stamp(),'scope':'Complete actual copied desktop src and root configuration tree (node_modules external, package-lock bound); exact component/test overlays are copied inside this independent tree.','files':rows}
(out/'versions.json').write_text(json.dumps({'node':subprocess.check_output(['node','--version'],text=True).strip(),'vitest':subprocess.check_output(['node',str(tree/'node_modules/vitest/vitest.mjs'),'--version'],cwd=tree,text=True).strip()},indent=2)+'\n')
for name,fixed,pattern in [('unfixed',False,'legacy|manufacturing a cleanup display conclusion'),('fixed',True,'')]:
 if fixed: shutil.copy2(root/'after'/component,tree/'src/components/journey/ProjectChecks.tsx')
 before=bind(); (out/(name+'-source-before.json')).write_text(json.dumps(before,indent=2)+'\n')
 cmd=['node',str(tree/'node_modules/vitest/vitest.mjs'),'run','src/components/journey/ProjectChecks.test.tsx','--maxWorkers=1','--reporter=json','--outputFile='+str(out/(name+'-vitest.json'))]
 if pattern:cmd+=['-t',pattern]
 receipt={'startedAt':stamp(),'command':cmd,'cwd':str(tree),'sourceMode':'fixed' if fixed else 'original component + unchanged new regression'}
 start=time.monotonic()
 with (out/(name+'.stdout')).open('w') as stdout,(out/(name+'.stderr')).open('w') as stderr:
  proc=subprocess.run(cmd,cwd=tree,stdout=stdout,stderr=stderr,timeout=240)
 receipt.update(exitCode=proc.returncode,elapsedSeconds=time.monotonic()-start,finishedAt=stamp())
 after=bind(); (out/(name+'-source-after.json')).write_text(json.dumps(after,indent=2)+'\n'); receipt['inputsUnchanged']=before['files']==after['files']
 report=json.loads((out/(name+'-vitest.json')).read_text()) if (out/(name+'-vitest.json')).exists() else {}
 receipt['counts']={k:report.get(k) for k in ['numTotalTests','numPassedTests','numFailedTests','numPendingTests']}
 (out/(name+'-result.json')).write_text(json.dumps(receipt,indent=2)+'\n')
 print(json.dumps({'name':name,'exitCode':proc.returncode,'counts':receipt['counts'],'elapsedSeconds':receipt['elapsedSeconds'],'inputsUnchanged':receipt['inputsUnchanged']}),flush=True)
 if fixed and proc.returncode!=0: break
