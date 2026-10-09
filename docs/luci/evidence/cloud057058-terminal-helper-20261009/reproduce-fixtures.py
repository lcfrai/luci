from pathlib import Path
import argparse,hashlib,json,os,subprocess,sys,shutil
p=Path(__file__).resolve().parent
a=argparse.ArgumentParser();a.add_argument('--output',required=True,type=Path);a.add_argument('--node',default='node');args=a.parse_args()
assert args.output.is_absolute();args.output.mkdir(mode=0o700,parents=False,exist_ok=False)
node=shutil.which(args.node);assert node,'Node executable unavailable'
base={k:v for k,v in os.environ.items() if k in ('PATH','SystemRoot','WINDIR','TEMP','TMP','LANG','LC_ALL')}
rows=[]
cases=[('problem-original','installed-v3','problem-diagnostic.test.mjs',{'C057_ORIGINAL':'1'},1),('problem-draft','installed-v3','problem-diagnostic.test.mjs',{'C057_DRAFT':'1'},1),('problem-fixed','installed-v3','problem-diagnostic.test.mjs',{},0),('mount-original','installed-v4','mount-contract.test.mjs',{'C058_ORIGINAL':'1'},1),('mount-fixed','installed-v4','mount-contract.test.mjs',{},0)]
for name,folder,test,flags,expected in cases:
 command=[node,'--max-old-space-size=64','--test','--test-concurrency=1',test]
 result=subprocess.run(command,cwd=p/folder,env=dict(base,**flags),capture_output=True,timeout=60)
 for suffix,b in [('stdout',result.stdout),('stderr',result.stderr)]:
  assert len(b)<=262144;f=args.output/(name+'.'+suffix);f.write_bytes(b)
 rows.append({'name':name,'command':command,'cwd':folder,'envFlags':flags,'expectedExit':expected,'actualExit':result.returncode,'stdoutSHA256':hashlib.sha256(result.stdout).hexdigest(),'stderrSHA256':hashlib.sha256(result.stderr).hexdigest()})
 (args.output/'reproduction.json').write_text(json.dumps({'scope':'New inert VM fixture execution only','rows':rows},indent=2)+'\n')
 if result.returncode!=expected:sys.exit(1)
print(json.dumps({'allExpectedExits':True,'invocations':len(rows),'output':str(args.output)}))
