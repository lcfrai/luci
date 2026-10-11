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
cmd=['go','test','-p','1','-count=1','-timeout=240s','-json']
if mode=='original':
    cmd+=['-overlay='+str(out/'overlay.json'),'-run','^TestCoordinationAutomaticRetentionAllowsOrdinaryProposalsPastActiveLimit$','./internal/product']
elif mode.startswith('race'):
    cmd+=['-race','-run','^(TestCoordinationArchive|TestCoordinationRetention|TestCoordinationConcurrent|TestProductRegisteredReadsCannotMutateProjectsOrStartResources|TestTeamCoordinator)','./internal/product','./internal/management']
else:
    cmd+=['-run','^(TestCoordination|TestOrchestratorProblemKeepsConflictAndStorageFailureDistinct|TestProductRegisteredReadsCannotMutateProjectsOrStartResources)','./internal/product']
env=os.environ.copy()
env.update({'GOMAXPROCS':'2','GOCACHE':'/tmp/luci-go-build-cache','GOMODCACHE':'/tmp/luci-retained-tools/go-mod','TMPDIR':'/workspace/.t','GOTMPDIR':'/workspace/.t','GOFLAGS':'-mod=readonly'})
inv={'command':cmd,'cwd':str(ROOT/'control-plane'),'startedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'head':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'status':subprocess.check_output(['git','status','--short'],cwd=ROOT,text=True).splitlines(),'environment':{k:env[k] for k in ('GOMAXPROCS','GOCACHE','GOMODCACHE','TMPDIR','GOTMPDIR','GOFLAGS')},'inputScope':'Repository control-plane Go/module files plus root go.work/go.work.sum; excludes external modules, Go SDK, and exact executed test binary.','sourceCount':len(before),'outerTimeoutSeconds':300,'rootFloorBytes':int(1.25*1024**3),'tmpFloorBytes':256*1024**2}
(out/'invocation.json').write_text(json.dumps(inv,indent=2)+'\n')
floor=lambda:free('/workspace')>=inv['rootFloorBytes'] and free('/tmp')>=inv['tmpFloorBytes']
start=time.monotonic();reason=None;code=None
if not floor():reason='storage-floor-refusal'
else:
    with (out/'stdout.jsonl').open('w') as stdout,(out/'stderr.txt').open('w') as stderr:
        proc=subprocess.Popen(cmd,cwd=ROOT/'control-plane',env=env,stdout=stdout,stderr=stderr,start_new_session=True)
        while proc.poll() is None:
            if time.monotonic()-start>300:reason='outer-timeout'
            elif not floor():reason='storage-floor-during-run'
            if reason:
                os.killpg(proc.pid,signal.SIGTERM)
                try:proc.wait(timeout=3)
                except subprocess.TimeoutExpired:os.killpg(proc.pid,signal.SIGKILL);proc.wait()
                break
            time.sleep(0.5)
        code=proc.returncode
after=sources()
(out/'inputs-after.json').write_text(json.dumps(after,indent=2)+'\n')
receipt={'exitCode':code,'stopReason':reason,'elapsedSeconds':time.monotonic()-start,'effectiveRepositoryInputsStable':before==after,'sourceCountBefore':len(before),'sourceCountAfter':len(after),'freeBytesAfter':{'root':free('/workspace'),'tmp':free('/tmp')},'completedAt':datetime.datetime.now(datetime.timezone.utc).isoformat()}
(out/'result.json').write_text(json.dumps(receipt,indent=2)+'\n')
print(json.dumps(receipt),flush=True)
