from pathlib import Path
import ast,json,hashlib,subprocess,datetime,difflib
HERE=Path(__file__).resolve().parent;OLD=Path('/workspace/.luci-tools/cloud032-idle-integrated-candidate-20261009');REPO=Path('/workspace/luci');HEAD='67a5b7c75ca6354d43085f5bb81ec53ee4291f34'
h=lambda b:hashlib.sha256(b).hexdigest()
old=(OLD/'actual/sustained.mjs').read_bytes();new=(HERE/'actual/sustained.mjs').read_bytes();assert old==new
originalOld=(OLD/'original/sustained.mjs').read_text();originalNew=(HERE/'original/sustained.mjs').read_text()
cut=" setStage('sustained-work');const loaded=phaseStart('loaded');"
assert originalOld[:originalOld.index(cut)]==originalNew[:originalNew.index(cut)]
ending="\n });\n}catch(error){"
assert originalOld[originalOld.index(ending,originalOld.index(cut)):]==originalNew[originalNew.index(ending,originalNew.index(cut)):]
source=new.decode();spans={}
def section(start,end):
 assert source.count(start)==1,start
 begin=source.index(start);stop=source.index(end,begin)+len(end)
 return source[begin:stop].encode()
for name,(start,end) in {
 'measurement-and-quiet-functions':('function phaseStart(name)','async function prepare(owner)'),
 'saved-context-and-draft-setup':('async function saveContext(owner','function exactQueueControlEntry(owner'),
 'startup-and-queue-setup':(" engineID=docker('info'"," setStage('idle-baseline');"),
 'quiet-primary-and-separate-trace-workload':(" setStage('idle-baseline');"," assert.deepEqual(effects(),{create:0,start:0,destroy:0});"),
 'bounded-cleanup-wiring':('// BEGIN SUSTAINED CLEANUP WIRING','// END SUSTAINED CLEANUP WIRING'),
}.items():
 b=section(start,end);assert b in old;spans[name]={'bytes':len(b),'sha256':h(b),'byteIdenticalTo990Idle':True}
rawFinally=source[source.index('finally {\n beginCleanup();persistFailureCheckpoint();'):].encode();assert rawFinally in old;spans['final-cleanup-and-receipt']={'bytes':len(rawFinally),'sha256':h(rawFinally),'byteIdenticalTo990Idle':True}
assert 'BEGIN SUSTAINED PHONE OVERVIEW' in originalNew and 'BEGIN SUSTAINED PHONE OVERVIEW' not in source
members={};capture=json.loads((HERE/'template-capture.json').read_text())
for name,row in capture['sources'].items():
 raw=(REPO/name).read_bytes();assert h(raw)==row['sha256'] and len(raw)==row['bytes'];assert raw==subprocess.check_output(['git','show',HEAD+':'+name],cwd=REPO)
for name in json.loads((HERE/'actual/external-source-binding.json').read_text())['localSources']:
 raw=(HERE/'actual'/name).read_bytes();members[name]={'sha256':h(raw),'bytes':len(raw),'identicalTo990Idle':raw==(OLD/'actual'/name).read_bytes()};assert members[name]['identicalTo990Idle']
for name in ['prepare-idle.py','launch.py']:assert (HERE/name).read_bytes()==(OLD/name).read_bytes()
syntax=[]
for p in [HERE/'launch.py',HERE/'prepare-idle.py',HERE/'actual/run-with-observer.py']:
 ast.parse(p.read_text(),filename=str(p));syntax.append({'command':['python AST parse',str(p)],'exitCode':0,'sha256':h(p.read_bytes())})
for p in sorted((HERE/'actual').glob('*.mjs')):
 result=subprocess.run(['node','--check',str(p)],capture_output=True,text=True,timeout=20);syntax.append({'command':['node','--check',str(p)],'exitCode':result.returncode,'stdout':result.stdout,'stderr':result.stderr,'sha256':h(p.read_bytes())});assert result.returncode==0
assert subprocess.check_output(['git','rev-parse','HEAD'],cwd=REPO,text=True).strip()==HEAD
assert subprocess.check_output(['git','status','--porcelain'],cwd=REPO,text=True)==''
report={'preparedOnly':True,'runtimeExecuted':False,'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'head':HEAD,'repositoryCleanBeforeAndAfter':True,'scope':'Read-only Git/source binding, textual extraction, Node syntax and Python AST checks only. No app, browser, Docker, compiler/build, tests, provider or acceptance workload.','entireGeneratedDriverIdenticalTo990Idle':{'bytes':len(new),'sha256':h(new)},'C049PhoneChangeRemovedBeforeIdleExecution':True,'originalTemplateDelta':'Only the full sustained phone tail differs; captured original prefix through idle and suffix cleanup are identical.','selectedLocalMembers':members,'spans':spans,'syntaxChecks':syntax,'budgets':{'preparationSeconds':60,'quietSeconds':60,'primarySwitches':100,'primaryInputs':200,'primaryResets':100,'separateTraceRounds':8,'separateTraceActions':32,'workflowSeconds':210,'cleanupSeconds':120,'outerWaitSeconds':450,'outerTermSeconds':30,'outerKillWaitSeconds':10,'switchP95MaxMs':300,'inputP95MaxMs':100,'p99StrictlyBelowMs':1000,'hardAllowedCreate':0,'hardAllowedStart':0,'rootFloorBytes':6*1024**3,'runningReserveBytes':1024**3},'packageScope':'Exact provenance supplied to root; actual launcher must verify package binary/index/full archive/member/image identities before any app launch. Preparation does not claim that preflight or acceptance passed.'}
(HERE/'preparation-verification.json').write_text(json.dumps(report,indent=2)+'\n')
(HERE/'current-template-vs-990.patch').write_text(''.join(difflib.unified_diff(originalOld.splitlines(keepends=True),originalNew.splitlines(keepends=True),fromfile='990-template/sustained.mjs',tofile='67a5-template/sustained.mjs')))
print(json.dumps({'preparedOnly':True,'runtimeExecuted':False,'driverSHA256':h(new),'selectedSources':len(capture['sources']),'localIdenticalMembers':len(members),'identicalSpans':len(spans),'syntaxChecks':len(syntax)}))
