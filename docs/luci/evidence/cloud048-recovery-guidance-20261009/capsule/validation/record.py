import datetime,hashlib,json,os,pathlib,signal,subprocess,sys,time
base=pathlib.Path(__file__).resolve().parent
root=pathlib.Path('/workspace/luci')
label=sys.argv[1]; timeout=int(sys.argv[2]); command=sys.argv[3:]
out=base/label;out.mkdir()
paths=['desktop/src/components/journey/ProjectSwarm.tsx','desktop/src/components/journey/SwarmRunQueue.tsx','desktop/src/components/journey/ProjectSwarmQueue.test.tsx','desktop/src/components/journey/ProjectSwarm.test.tsx','desktop/src/components/journey/ProjectSwarmRefresh.test.tsx','desktop/src/components/journey/SwarmRunQueue.test.tsx','desktop/package.json','desktop/package-lock.json','desktop/vitest.config.ts','desktop/tsconfig.json']
def hashes():
 return {p:hashlib.sha256((root/p).read_bytes()).hexdigest() for p in paths if (root/p).is_file()}
record={'command':command,'cwd':'desktop','envOverrides':{'GOMAXPROCS':'2'},'timeoutSeconds':timeout,'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'inputsBefore':hashes()}
(out/'command.json').write_text(json.dumps(record,indent=2)+'\n')
env=os.environ.copy();env['GOMAXPROCS']='2'; start=time.monotonic()
with (out/'stdout.log').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
 p=subprocess.Popen(command,cwd=root/'desktop',env=env,stdout=stdout,stderr=stderr,start_new_session=True)
 try:code=p.wait(timeout=timeout)
 except subprocess.TimeoutExpired:
  os.killpg(p.pid,signal.SIGTERM)
  try:p.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL);p.wait()
  code=124;record['timedOut']=True
record.update(exitCode=code,elapsedSeconds=round(time.monotonic()-start,3),endedAt=datetime.datetime.now(datetime.timezone.utc).isoformat(),inputsAfter=hashes())
(out/'result.json').write_text(json.dumps(record,indent=2)+'\n')
print(json.dumps({'label':label,'exitCode':code,'elapsedSeconds':record['elapsedSeconds'],'unchangedInputs':record['inputsBefore']==record['inputsAfter']}))
sys.exit(code)
