#!/usr/bin/env python3
"""One bounded source run; captures effective repository inputs, not a binary attestation."""
import datetime, hashlib, json, os, pathlib, signal, subprocess, sys, time

ROOT=pathlib.Path('/workspace/luci')
BASE=pathlib.Path('/workspace/.luci-tools/cloud086-retention')
mode=sys.argv[1]
out=BASE/'evidence'/mode
out.mkdir(parents=True,exist_ok=False)
overlay={}
if mode=='original':
    original=BASE/'original'
    original.mkdir(exist_ok=True)
    (original/'empty_test.go').write_text('package product\n')
    (original/'compatibility.go').write_text('''package product
import "github.com/lcfrai/luci/control-plane/internal/project"
// Inert compatibility for newer shared caller sites. The original coordination
// store has no archives; these cannot supply receipts or alter the old64 bound.
func(s *Server) archivedCoordinationRequestLocked(project.Project,string,string)(*storedOrchestratorTurn,error){return nil,nil}
func(s *Server) archivedCoordinationTurnLocked(project.Project,string)(*storedOrchestratorTurn,error){return nil,nil}
func(s *Server) archivedCoordinationForeignRequestLocked(project.Project,string,string)(bool,error){return false,nil}
''')
    for name in ('coordination.go','coordination_store.go'):
        overlay[str(ROOT/'control-plane/internal/product'/name)]=str(BASE/'before'/name)
    overlay[str(ROOT/'control-plane/internal/product/coordination_archive.go')]=str(original/'compatibility.go')
    overlay[str(ROOT/'control-plane/internal/product/coordination_archive_test.go')]=str(original/'empty_test.go')
    (out/'overlay.json').write_text(json.dumps({'Replace':overlay},indent=2)+'\n')

def sources():
    files=subprocess.check_output(['rg','--files','-g','*.go','-g','go.mod','-g','go.sum','control-plane'],cwd=ROOT,text=True).splitlines()
    files += [n for n in ('go.work','go.work.sum') if (ROOT/n).is_file()]
    rows={}
    for name in sorted(set(files)):
        actual=pathlib.Path(overlay.get(str(ROOT/name),str(ROOT/name)))
        data=actual.read_bytes()
        rows[name]={'sha256':hashlib.sha256(data).hexdigest(),'bytes':len(data),'effectiveSource':str(actual)}
    return rows

def free(path):
    s=os.statvfs(path);return s.f_bavail*s.f_frsize

before=sources()
(out/'inputs-before.json').write_text(json.dumps(before,indent=2)+'\n')
short=mode.startswith('preview-first')
outer=90 if short else 300
cmd=['go','test','-p','1','-count=1','-timeout=20s' if short else '-timeout=240s','-json']
if mode=='original':
    cmd+=['-overlay='+str(out/'overlay.json'),'-run','^TestCoordinationAutomaticRetentionAllowsOrdinaryProposalsPastActiveLimit$','./internal/product']
elif mode.startswith('preview-first'):
    cmd+=['-race','-run','^TestCoordinatorPreviewHTTPAdvancesExactCheckedSourceWithoutPublishing$/^preview$','./internal/product']
elif mode.startswith('preview-rest'):
    cmd+=['-race','-run','^(TestCoordinator.*Preview|TestCoordinatorDelivery|TestApplicationConfiguration|TestProjectRunLocalReleaseJoinNeedsPrivateExactCandidateAndFreshOwnedHealth$|TestLocalReleaseBuildsCommittedSafeContextThenVerifiesHealthAndOwnership$|TestLocalReleaseObservationRejectsStalePortAndHealthWithoutRewritingHistory$|TestPreviewReceiptRetainsUnknownBuildWithoutContainerAndNeverReplays$|TestPreviewReceiptCASPreventsStaleReadyFromReplacingExactStop$|TestProductRegisteredReadsCannotMutateProjectsOrStartResources|TestTeamCoordinator)','-skip','^TestCoordinatorPreviewHTTPAdvancesExactCheckedSourceWithoutPublishing$/^preview$','./internal/product','./internal/management']
elif mode.startswith('refinement'):
    cmd+=['-run','^TestCoordinationArchivedOriginContinuesThroughCurrentTypedPreparation$','./internal/product']
elif mode.startswith('race'):
    cmd+=['-race','-run','^(TestCoordinationArchive|TestCoordinationRetention|TestCoordinationConcurrent|TestCoordinator.*Preview|TestCoordinatorDelivery|TestProjectRunLocalReleaseJoinNeedsPrivateExactCandidateAndFreshOwnedHealth$|TestLocalReleaseBuildsCommittedSafeContextThenVerifiesHealthAndOwnership$|TestLocalReleaseObservationRejectsStalePortAndHealthWithoutRewritingHistory$|TestPreviewReceiptRetainsUnknownBuildWithoutContainerAndNeverReplays$|TestPreviewReceiptCASPreventsStaleReadyFromReplacingExactStop$|TestProductRegisteredReadsCannotMutateProjectsOrStartResources|TestTeamCoordinator)','./internal/product','./internal/management']
else:
    cmd+=['-run','^(TestCoordination|TestOrchestratorProblemKeepsConflictAndStorageFailureDistinct|TestProductRegisteredReadsCannotMutateProjectsOrStartResources)','./internal/product']
env=os.environ.copy()
env.update({'GOMAXPROCS':'2','GOCACHE':'/dev/shm/luci-go-cache-20261011-recovery','GOMODCACHE':'/tmp/luci-retained-tools/go-mod','TMPDIR':'/workspace/.t','GOTMPDIR':'/workspace/.t','GOFLAGS':'-mod=readonly'})
inv={'command':cmd,'cwd':str(ROOT/'control-plane'),'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'status':subprocess.check_output(['git','status','--short'],cwd=ROOT,text=True).splitlines(),'environment':{k:env[k] for k in ('GOMAXPROCS','GOCACHE','GOMODCACHE','TMPDIR','GOTMPDIR','GOFLAGS')},'inputScope':'Repository control-plane Go/module files plus root go.work/go.work.sum; excludes external modules, Go SDK, and exact executed test binary.','sourceCount':len(before),'outerTimeoutSeconds':outer,'rootFloorBytes':512*1024**2,'tmpFloorBytes':256*1024**2,'cgroupRemainingFloorBytes':4*1024**3,'cacheLimitBytes':2*1024**3,'resourceAdjustment':'Root explicitly authorized memory-backed Go cache after root disk exhaustion; lower root floor applies only to controlled test binaries/logs. Product admission/resource guards are unchanged.'}
(out/'invocation.json').write_text(json.dumps(inv,indent=2)+'\n')
def resources():
    cache=pathlib.Path(env['GOCACHE'])
    cache_bytes=int(subprocess.check_output(['du','-sb',str(cache)],text=True).split()[0]) if cache.exists() else 0
    cgroup=pathlib.Path('/sys/fs/cgroup')
    limit=(cgroup/'memory.max').read_text().strip()
    used=int((cgroup/'memory.current').read_text())
    if limit=='max':raise RuntimeError('explicit cgroup memory bound unavailable')
    return {'rootFreeBytes':free('/workspace'),'tmpFreeBytes':free('/tmp'),'cgroupRemainingBytes':int(limit)-used,'cacheBytes':cache_bytes}
def refuse(observed):
    if observed['rootFreeBytes']<inv['rootFloorBytes']:return 'root-floor'
    if observed['tmpFreeBytes']<inv['tmpFloorBytes']:return 'tmp-floor'
    if observed['cgroupRemainingBytes']<inv['cgroupRemainingFloorBytes']:return 'cgroup-memory-floor'
    if observed['cacheBytes']>inv['cacheLimitBytes']:return 'cache-size-ceiling'
    return None
observations=[]
def sample():
    try:
        observed=resources();observed['monotonicSeconds']=time.monotonic()-start
        observations.append(observed)
        return refuse(observed)
    except Exception as err:
        observations.append({'monotonicSeconds':time.monotonic()-start,'observationErrorClass':type(err).__name__})
        return 'resource-observation-unavailable'
start=time.monotonic();reason=None;code=None
initial=sample()
if initial:reason='resource-refusal:'+initial
else:
    with (out/'stdout.jsonl').open('w') as stdout,(out/'stderr.txt').open('w') as stderr:
        proc=subprocess.Popen(cmd,cwd=ROOT/'control-plane',env=env,stdout=stdout,stderr=stderr,start_new_session=True)
        while proc.poll() is None:
            if time.monotonic()-start>outer:reason='outer-timeout'
            elif problem:=sample():reason='resource-during-run:'+problem
            if reason:
                try:os.killpg(proc.pid,signal.SIGTERM)
                except ProcessLookupError:pass
                try:proc.wait(timeout=3)
                except subprocess.TimeoutExpired:os.killpg(proc.pid,signal.SIGKILL);proc.wait()
                break
            time.sleep(2)
        code=proc.returncode
after=sources()
(out/'inputs-after.json').write_text(json.dumps(after,indent=2)+'\n')
receipt={'exitCode':code,'stopReason':reason,'elapsedSeconds':time.monotonic()-start,'effectiveRepositoryInputsStable':before==after,'sourceCountBefore':len(before),'sourceCountAfter':len(after),'freeBytesAfter':{'root':free('/workspace'),'tmp':free('/tmp')},'completedAt':datetime.datetime.now(datetime.timezone.utc).isoformat()}
(out/'resource-observations.json').write_text(json.dumps(observations,indent=2)+'\n')
(out/'result.json').write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps(receipt),flush=True)
