from pathlib import Path
import hashlib,json,subprocess,os,time,signal
root=Path('/workspace/luci');out=Path(__file__).parent/'compact';out.mkdir();release=root/'artifacts/releases/0.5.0';prior=release.with_name('0.5.0-577f7a0f-full-docs');assert not prior.exists()
sha=lambda p:hashlib.file_digest(p.open('rb'),'sha256').hexdigest()
man=json.loads((release/'release-manifest.json').read_text());assert man['sourceCommit']=='577f7a0fdb039b7f875901a6a567d62fdfee88c8' and man['sourceDirty'] is False
for row in man['artifacts']:assert sha(release/row['name'])==row['sha256'] and (release/row['name']).stat().st_size==row['bytes']
release.rename(prior)
for row in man['artifacts']:assert sha(prior/row['name'])==row['sha256']
(out/'prior-preserved.json').write_text(json.dumps({'path':str(prior),'manifest':man,'allArchiveHashesUnchanged':True},indent=2))
for n in ['release-manifest.json','SHA256SUMS']:(out/('prior-'+n)).write_bytes((prior/n).read_bytes())
assert not subprocess.check_output(['git','status','--porcelain'],cwd=root,text=True)
cmd=['bash','scripts/build-release.sh','0.5.0','linux-amd64'];env=dict(os.environ,GOMAXPROCS='2',GOFLAGS='-mod=readonly -p=1');start=time.time()
(out/'command.json').write_text(json.dumps({'argv':cmd,'cwd':str(root),'source':man['sourceCommit'],'sourceDirty':False,'start':start,'environment':{k:env.get(k) for k in ['GOMAXPROCS','GOFLAGS','GOWORK']},'reason':'Cached remote tracking ref was stale after push; explicit fetch now associates exact source. Preserve initial full-doc package; repeat supported clean build to exercise compact guides.'},indent=2))
with (out/'build.stdout.log').open('w') as stdout,(out/'build.stderr.log').open('w') as stderr:
 p=subprocess.Popen(cmd,cwd=root,env=env,stdout=stdout,stderr=stderr,start_new_session=True)
 try:code=p.wait(timeout=600)
 except subprocess.TimeoutExpired:
  os.killpg(p.pid,signal.SIGTERM)
  try:p.wait(timeout=5)
  except subprocess.TimeoutExpired:os.killpg(p.pid,signal.SIGKILL);p.wait()
  code=124
result={'exit':code,'elapsedSeconds':time.time()-start,'headAfter':subprocess.check_output(['git','rev-parse','HEAD'],cwd=root,text=True).strip(),'statusAfter':subprocess.check_output(['git','status','--porcelain'],cwd=root,text=True)};(out/'result.json').write_text(json.dumps(result,indent=2));print(json.dumps(result));print((out/'build.stdout.log').read_text()[-600:]);print((out/'build.stderr.log').read_text()[-1200:]);raise SystemExit(code)
