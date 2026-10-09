from pathlib import Path
import os,subprocess,json,hashlib,gzip,time,datetime,signal
repo=Path('/workspace/luci');old=Path('/workspace/.luci-tools/cloud048050-combined-validation');out=Path('/workspace/.luci-tools/cloud051-final-typecheck-attempt2');out.mkdir(exist_ok=False)
sha=lambda b:hashlib.sha256(b).hexdigest()
def snapshot(name):
 rows=json.loads(gzip.decompress((old/(name+'-after-typecheck.json.gz')).read_bytes()))
 for row in rows:
  path=repo/row['path'];row['bytes']=path.stat().st_size;row['sha256']=sha(path.read_bytes())
  if path.is_symlink():row['symlinkTarget']=os.readlink(path)
 return rows
before={kind:snapshot(kind) for kind in ['repository','dependencies']}
original={r['path']:r for r in json.loads(gzip.decompress((old/'repository-after-typecheck.json.gz').read_bytes()))}
diff=[r['path'] for r in before['repository'] if original[r['path']]!=r]
assert sorted(diff)==['desktop/src/components/AppShell.test.tsx','desktop/src/components/common/DraftNavigationDialog.tsx'],diff
for kind,rows in before.items():(out/(kind+'-before.json.gz')).write_bytes(gzip.compress((json.dumps(rows,indent=2)+'\n').encode(),mtime=0))
for path in diff:
 dst=out/'changed-source'/path;dst.parent.mkdir(parents=True,exist_ok=True);dst.write_bytes((repo/path).read_bytes())
command=['npm','run','typecheck'];started=datetime.datetime.now(datetime.timezone.utc).isoformat();start=time.monotonic();timed_out=False
with (out/'typecheck.stdout.log').open('wb') as stdout,(out/'typecheck.stderr.log').open('wb') as stderr:
 process=subprocess.Popen(command,cwd=repo/'desktop',stdout=stdout,stderr=stderr,start_new_session=True)
 try:code=process.wait(timeout=300)
 except subprocess.TimeoutExpired:
  timed_out=True;os.killpg(process.pid,signal.SIGTERM)
  try:code=process.wait(timeout=10)
  except subprocess.TimeoutExpired:os.killpg(process.pid,signal.SIGKILL);code=process.wait(timeout=10)
after={kind:snapshot(kind) for kind in before}
for kind,rows in after.items():(out/(kind+'-after.json.gz')).write_bytes(gzip.compress((json.dumps(rows,indent=2)+'\n').encode(),mtime=0))
result={'command':command,'startedAt':started,'elapsedSeconds':time.monotonic()-start,'exitCode':code,'timedOut':timed_out,'inputsStable':before==after,'counts':{k:len(v) for k,v in before.items()},'changedSinceFullFrontend':diff,'fullFrontendReceiptSHA256':sha((old/'result.json').read_bytes()),'scope':'Final combined TypeScript with CLOUD051; earlier full2787 assertion run predates these two changed files. Focused CLOUD051 tests are recorded separately; no new full-suite or runtime claim.'}
(out/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result),flush=True)
raise SystemExit(0 if code==0 and not timed_out and before==after else 1)
