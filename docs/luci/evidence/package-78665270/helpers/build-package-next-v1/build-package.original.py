#!/usr/bin/env python3
"""Prepare exact clean combined-source package; explicit runtime flag required."""
from pathlib import Path
import argparse,subprocess,os,json,datetime,time,hashlib,signal,re
ROOT=Path('/workspace/luci');OLD='edd0cb121ff6847f681ab6bf41509a497f043d41'
def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def sha(p):
 with p.open('rb')as f:return hashlib.file_digest(f,'sha256').hexdigest()
def write(p,v):
 b=(json.dumps(v,indent=2)+'\n').encode();tmp=p.with_suffix(p.suffix+'.new');fd=os.open(tmp,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
 try:
  with os.fdopen(fd,'wb')as f:f.write(b);f.flush();os.fsync(f.fileno())
  os.replace(tmp,p)
  fd=os.open(p.parent,os.O_RDONLY|os.O_DIRECTORY)
  try:os.fsync(fd)
  finally:os.close(fd)
 except BaseException:raise

def clean_source(expected):
 assert re.fullmatch('[a-f0-9]{40}',expected)
 head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True,timeout=30).strip();assert head==expected,'Unexpected source'
 assert not subprocess.check_output(['git','status','--porcelain=v1','--untracked-files=all'],cwd=ROOT,text=True,timeout=30).strip(),'Dirty source'
 return head

def preserve_release(release,out,commit):
 assert release.is_dir()and not release.is_symlink(),'Expected prior generated release directory'
 old=json.loads((release/'release-manifest.json').read_text());assert old['sourceCommit']==OLD and old['sourceDirty']is False,'Prior release identity mismatch'
 names=[row['name']for row in old['artifacts']];assert names and len(names)==len(set(names)) and all(isinstance(n,str)and Path(n).name==n and n not in ['.','..','release-manifest.json','SHA256SUMS']for n in names)
 paths=[release/name for name in names]+[release/'release-manifest.json',release/'SHA256SUMS'];before={p.name:{'sha256':sha(p),'bytes':p.stat().st_size,'link':os.readlink(p)if p.is_symlink()else None}for p in paths}
 for row in old['artifacts']:assert before[row['name']]['sha256']==row['sha256']and before[row['name']]['bytes']==row['bytes']
 retired=release.with_name('0.5.0-edd0cb12-preserved-before-'+commit[:8]);assert not retired.exists()and not retired.is_symlink(),'Preservation path already occupied'
 receipt={'at':now(),'oldSource':OLD,'newSource':commit,'originalPath':str(release),'preservedPath':str(retired),'members':before,'status':'rename-pending','scope':'Generated directory rename only; target archive bytes, retained package/fixtures/ledgers unchanged'}
 write(out/'previous-release-preservation.json',receipt);release.rename(retired)
 for name,m in before.items():assert sha(retired/name)==m['sha256']and(retired/name).stat().st_size==m['bytes']
 receipt.update(status='preserved-verified',originalProvenanceUnchanged=True);write(out/'previous-release-preservation.json',receipt)
 return receipt

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--expected-source',required=True);parser.add_argument('--window-granted',action='store_true',required=True);args=parser.parse_args();os.umask(0o077)
 commit=clean_source(args.expected_source);assert commit!=OLD,'New corrected source required'
 assert subprocess.check_output(['git','for-each-ref','--contains='+commit,'--count=1','--format=%(objectname)','refs/remotes/origin/'],cwd=ROOT,text=True,timeout=30).strip(),'Source must be retrievable from origin'
 out=ROOT/('artifacts/quality-swarm/candidate-'+commit[:8]+'/package');assert not out.exists();out.mkdir(parents=True,mode=0o700)
 preserved=preserve_release(ROOT/'artifacts/releases/0.5.0',out,commit);clean_source(commit)
 cmd=['bash','scripts/build-release.sh','0.5.0','linux-amd64'];env=dict(os.environ,GOMAXPROCS='2',GOFLAGS='-mod=readonly -p=1')
 metadata={'sourceCommit':commit,'startedAt':now(),'command':cmd,'helperSHA256':sha(Path(__file__)),'environment':{'GOMAXPROCS':'2','GOFLAGS':'-mod=readonly -p=1'},'timeoutSeconds':600,'cleanBefore':True,'status':'running','preservedPreviousSource':OLD}
 write(out/'build-invocation.json',metadata);began=time.monotonic();timedout=False
 with(out/'build.log').open('w')as log:
  process=subprocess.Popen(cmd,cwd=ROOT,env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True);metadata.update(ownedPID=process.pid,ownedProcessGroup=os.getpgid(process.pid));write(out/'build-invocation.json',metadata)
  try:code=process.wait(timeout=600)
  except subprocess.TimeoutExpired:
   timedout=True;metadata['status']='timeout-cleanup';write(out/'build-failure-checkpoint.json',metadata);os.killpg(process.pid,signal.SIGTERM)
   try:code=process.wait(timeout=10)
   except subprocess.TimeoutExpired:os.killpg(process.pid,signal.SIGKILL);code=process.wait(timeout=10)
 clean=not subprocess.check_output(['git','status','--porcelain'],cwd=ROOT,text=True,timeout=30).strip();same=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True,timeout=30).strip()==commit
 result={**metadata,'status':'finished','exitCode':code,'timedOut':timedout,'elapsedSeconds':time.monotonic()-began,'finishedAt':now(),'cleanAfter':clean,'sameHead':same};write(out/'build-result.json',result);print(json.dumps(result),flush=True);assert code==0 and not timedout and clean and same
 subprocess.run(['python3','/workspace/.luci-tools/verify-local-package.py',commit,str(out)],cwd=ROOT,check=True,timeout=180)
if __name__=='__main__':main()
