from pathlib import Path
import subprocess,os,json,datetime,time,hashlib,signal,sys
r=Path('/workspace/luci')
now=lambda:datetime.datetime.now(datetime.timezone.utc).isoformat()
sha=lambda p:hashlib.file_digest(p.open('rb'),'sha256').hexdigest()
commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=r,text=True).strip()
assert len(commit)==40 and not subprocess.check_output(['git','status','--porcelain'],cwd=r,text=True).strip()
assert subprocess.check_output(['git','for-each-ref','--contains='+commit,'--count=1','--format=%(objectname)','refs/remotes/origin/'],cwd=r,text=True).strip()
out=r/('artifacts/quality-swarm/candidate-'+commit[:8]+'/package');assert not out.exists();out.mkdir(parents=True)
release=r/'artifacts/releases/0.5.0'
if release.exists():
 old=json.loads((release/'release-manifest.json').read_text());assert old['sourceCommit']=='67a5b7c75ca6354d43085f5bb81ec53ee4291f34' and old['sourceDirty'] is False
 paths=[release/row['name'] for row in old['artifacts']]+[release/'release-manifest.json',release/'SHA256SUMS']
 before={p.name:{'sha256':sha(p),'bytes':p.stat().st_size,'link':os.readlink(p) if p.is_symlink() else None} for p in paths}
 for row in old['artifacts']:assert before[row['name']]['sha256']==row['sha256'] and before[row['name']]['bytes']==row['bytes']
 retired=release.with_name('0.5.0-67a5b7c7-preserved-before-'+commit[:8]);assert not retired.exists();release.rename(retired)
 for name,value in before.items():assert sha(retired/name)==value['sha256']
 (out/'previous-release-preservation.json').write_text(json.dumps({'at':now(),'oldSource':old['sourceCommit'],'originalPath':str(release),'preservedPath':str(retired),'members':before,'originalProvenanceUnchanged':True,'scope':'Generated release directory rename only; old retained package, target archive bytes, original provenance and failed fixtures unchanged'},indent=2)+'\n')
cmd=['bash','scripts/build-release.sh','0.5.0','linux-amd64'];env=dict(os.environ,GOMAXPROCS='2',GOFLAGS='-mod=readonly -p=1')
metadata={'sourceCommit':commit,'startedAt':now(),'command':cmd,'environment':{'GOMAXPROCS':'2','GOFLAGS':'-mod=readonly -p=1'},'timeoutSeconds':600,'cleanBefore':True,'status':'running'}
(out/'build-invocation.json').write_text(json.dumps(metadata,indent=2)+'\n');began=time.monotonic();timedout=False
with (out/'build.log').open('w') as log:
 process=subprocess.Popen(cmd,cwd=r,env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
 metadata['ownedPID']=process.pid;metadata['ownedProcessGroup']=os.getpgid(process.pid);(out/'build-invocation.json').write_text(json.dumps(metadata,indent=2)+'\n')
 try:code=process.wait(timeout=600)
 except subprocess.TimeoutExpired:
  timedout=True;metadata['status']='timeout-cleanup';(out/'build-failure-checkpoint.json').write_text(json.dumps(metadata,indent=2)+'\n')
  os.killpg(process.pid,signal.SIGTERM)
  try:code=process.wait(timeout=10)
  except subprocess.TimeoutExpired:os.killpg(process.pid,signal.SIGKILL);code=process.wait(timeout=10)
clean=not subprocess.check_output(['git','status','--porcelain'],cwd=r,text=True).strip();same=subprocess.check_output(['git','rev-parse','HEAD'],cwd=r,text=True).strip()==commit
result={**metadata,'status':'finished','exitCode':code,'timedOut':timedout,'elapsedSeconds':time.monotonic()-began,'finishedAt':now(),'cleanAfter':clean,'sameHead':same};(out/'build-result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result),flush=True)
assert code==0 and not timedout and clean and same
subprocess.run(['python3','/workspace/.luci-tools/verify-local-package.py',commit,str(out)],cwd=r,check=True,timeout=180)
