import pathlib,json,hashlib,subprocess,os,time,signal,sys
stage=pathlib.Path(__file__).resolve().parent;root=pathlib.Path('/workspace/luci');mode=sys.argv[1];assert mode in ('before','after');attempt=sys.argv[2] if len(sys.argv)>2 else '1';assert attempt.isdigit();out=stage/(mode+'-attempt'+attempt);out.mkdir(exist_ok=False)
sha=lambda b:hashlib.sha256(b).hexdigest()
inputs=json.loads((stage.parent/'source-bindings.json').read_text())['files'];names=[x['path'] for x in inputs]+['desktop/src/components/teams/TeamWorkspaceAuthority.test.tsx']
def capture():return [{'path':n,'sha256':sha((root/n).read_bytes())} for n in names]
assert (root/'desktop/src/components/teams/TeamWorkspaceProvider.tsx').read_bytes()==(stage/'before.tsx').read_bytes()
before=capture();(out/'source-before.json').write_text(json.dumps(before,indent=2)+'\n')
helper=[]
for n in ['before.tsx','after.tsx','authority-test-before.tsx','authority-test-after.tsx','episode.test.tsx','config.mts','run.py','combined-followup.patch']:
 b=(stage/n).read_bytes();helper.append({'path':n,'bytes':len(b),'sha256':sha(b)});p=out/'frozen'/n;p.parent.mkdir(parents=True,exist_ok=True);p.write_bytes(b)
(out/'input-bindings.json').write_text(json.dumps(helper,indent=2)+'\n')
cmd=['node','node_modules/vitest/vitest.mjs','run','--config',str(stage/'config.mts'),'--maxWorkers=1','--reporter=json','--outputFile='+str(out/'report.json')];env=os.environ.copy();env['CLOUD056_EPISODE_FIXED']='1' if mode=='after' else '0';start=time.time();timed=False
with (out/'stdout.log').open('wb') as a,(out/'stderr.log').open('wb') as b:
 child=subprocess.Popen(cmd,cwd=root/'desktop',stdout=a,stderr=b,env=env,start_new_session=True)
 try:code=child.wait(timeout=120)
 except subprocess.TimeoutExpired:
  timed=True;os.killpg(child.pid,signal.SIGTERM)
  try:code=child.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(child.pid,signal.SIGKILL);code=child.wait(timeout=5)
after=capture();(out/'source-after.json').write_text(json.dumps(after,indent=2)+'\n');assert before==after
r={'mode':mode,'command':cmd,'exitCode':code,'timedOut':timed,'elapsedSeconds':time.time()-start,'repositoryInputCount':len(names),'sourceStable':True,'node':subprocess.check_output(['node','--version'],text=True).strip(),'npm':subprocess.check_output(['npm','--version'],text=True).strip(),'ownedRootExited':True};(out/'result.json').write_text(json.dumps(r,indent=2)+'\n');print(json.dumps(r))
