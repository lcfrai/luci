import datetime,hashlib,json,os,pathlib,subprocess,sys,time
base=pathlib.Path(__file__).parent; name=sys.argv[1]; old=name.startswith('before'); out=base/'evidence'/name;out.mkdir(parents=True,exist_ok=False)
root=pathlib.Path('/workspace/luci');src=base/'backend/control-plane/internal/api';target=root/'control-plane/internal/api'
files=['sessions.go','session_execution_generation.go','session_execution_generation_test.go']
replacements={str(target/n):str(base/'sessions.before.go' if old and n=='sessions.go' else src/n) for n in files}
(out/'overlay.json').write_text(json.dumps({'Replace':replacements},indent=2)+'\n')
def bind(p):
 b=p.read_bytes();return {'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()}
pre={p:bind(pathlib.Path(p)) for p in replacements.values()}
command=['/workspace/.luci-tools/go1.26.3/bin/go','test','-race','-p','2','-count=1','-timeout=180s','-json','-overlay='+str(out/'overlay.json'),'./internal/api','-run','^(TestSessionExecutionGeneration.*|TestSessionJSONLeaksNeitherTheTokenNorTheEndpoint|TestSessionJSONCarriesExactlyTheContractsTwentyOneKeys)$']
env=dict(os.environ,GOMAXPROCS='4',GOWORK=str(root/'go.work'));started=datetime.datetime.now(datetime.timezone.utc).isoformat();t=time.monotonic()
with (out/'stdout.jsonl').open('wb') as stdout,(out/'stderr.log').open('wb') as stderr:
 p=subprocess.run(command,cwd=root/'control-plane',env=env,stdout=stdout,stderr=stderr,timeout=210)
post={p:bind(pathlib.Path(p)) for p in replacements.values()};result={'command':command,'startedAt':started,'elapsedSeconds':time.monotonic()-t,'exitCode':p.returncode,'sourceStable':pre==post,'inputsBefore':pre,'inputsAfter':post,'logs':{n:bind(out/n) for n in ['stdout.jsonl','stderr.log']},'scope':'Selected API DTO projection and secret omission tests through external overlay; no app/runtime/provider'}
(out/'result.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
