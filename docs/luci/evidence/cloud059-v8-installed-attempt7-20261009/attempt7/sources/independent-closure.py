from pathlib import Path
import argparse, datetime, hashlib, json, os, stat, subprocess

p=argparse.ArgumentParser();p.add_argument('--attempt',type=int,required=True);a=p.parse_args()
assert 1 <= a.attempt <= 20
root=Path('/workspace/.luci-tools/cloud054-terminal-close/actual-edd0')
base=root/'evidence'/f'attempt{a.attempt}'
out=base/'independent-closure.json';assert not out.exists()
def sha(path):
    with path.open('rb') as f:return hashlib.file_digest(f,'sha256').hexdigest()
def snapshot(directory):
    files={};links=[]
    for path in directory.rglob('*'):
        info=path.lstat()
        if stat.S_ISLNK(info.st_mode):
            links.append({'relative':str(path.relative_to(directory)),'targetSHA256':hashlib.sha256(os.readlink(path).encode()).hexdigest()})
        elif stat.S_ISREG(info.st_mode):
            files[str(path.relative_to(directory))]={'sha256':sha(path),'bytes':info.st_size}
        else:assert stat.S_ISDIR(info.st_mode)
    return files,links
original,original_links=snapshot(base)
wrapper=json.loads((base/'wrapper-result.json').read_text())
driver=json.loads((base/'driver/result.json').read_text())
fixture=Path((base/'driver/private-fixture-path').read_text().strip())
assert fixture.parent==root/'fixtures' and fixture.name.startswith('luci-c054-') and fixture.resolve()==fixture
files,links=snapshot(fixture)
identities=[]
for row in wrapper['identities']:
    expected=row['expected'];path=Path('/proc')/str(expected['pid'])/'stat'
    try:
        fields=path.read_text().rsplit(') ',1)[1].split()
        now={'status':'observed' if fields[19]==expected['startTicks'] else 'original-absent-pid-reused','startTicks':fields[19],'state':fields[0],'parentPID':int(fields[1])}
    except FileNotFoundError:now={'status':'absent'}
    except PermissionError:now={'status':'unavailable'}
    identities.append({'pid':expected['pid'],'expectedStartTicks':expected['startTicks'],'family':expected['family'],'now':now})
env={key:os.environ[key] for key in ['PATH','LANG','LC_ALL','TZ'] if key in os.environ}
env['HOME']=str(base/'launcher-home')
def docker(*args):
    return subprocess.check_output(['docker','--host','unix:///var/run/docker.sock','--config',str(base/'launcher-home'),*args],env=env,text=True,timeout=10).strip()
containers=docker('ps','-aq','--no-trunc').split();volumes=docker('volume','ls','-q').split()
engine=docker('info','--format','{{.ID}}');assert engine==driver['daemon']['id']
ledger=fixture/'state/control-plane/execution/leases.json';value=json.loads(ledger.read_text())
counts={key:len(value.get(key,[])) for key in ['leases','checks','fences','checkFences']}
assert snapshot(base)==(original,original_links) and snapshot(fixture)==(files,links)
result={'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sourceCommit':driver['sourceCommit'],
    'passedWorkflow':driver['passed'],'originalFailedStage':driver.get('failedStage'),
    'originalResultSHA256':sha(base/'driver/result.json'),'originalWrapperSHA256':sha(base/'wrapper-result.json'),
    'capturedProblems':[row['problem'] for row in driver['apiActions'] if 'problem' in row],
    'identities':identities,'containers':containers,'volumes':volumes,'sameEngine':True,
    'ledger':{'sha256':sha(ledger),'bytes':ledger.stat().st_size,'counts':counts},
    'fixtureFilesCount':len(files),'fixtureManifestSHA256':hashlib.sha256(json.dumps(files,sort_keys=True).encode()).hexdigest(),
    'fixtureLinks':links,'originalFilesUnchanged':True,'fixtureFilesUnchanged':True,
    'scriptSHA256':sha(Path(__file__)),
    'scope':'Read-only exact recorded process identities, selected daemon inventories and private ledger counts. No signals, cleanup mutation, whole-machine census, historical charge release, numeric admission reconstruction or Docker wire counts. Private files remain unchanged; original transient/zombie and unreadable observations remain in their receipts.'}
with out.open('x') as f:json.dump(result,f,indent=2);f.write('\n')
os.chmod(out,0o600)
print(json.dumps({'receipt':str(out),'sha256':sha(out),'identities':identities,'ledgerCounts':counts,'containers':containers,'volumes':volumes,'fixtureFiles':len(files)}))
