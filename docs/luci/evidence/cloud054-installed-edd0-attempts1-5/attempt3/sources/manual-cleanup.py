from pathlib import Path
import datetime, hashlib, json, os, stat, subprocess, tarfile

base = Path('/workspace/.luci-tools/cloud054-terminal-close/actual-edd0/evidence/attempt3')
out = base / 'manual-recovery'
out.mkdir(mode=0o700)
fixture = Path((base/'driver/private-fixture-path').read_text().strip())
assert fixture.parent == base.parents[1]/'fixtures' and fixture.resolve() == fixture
cid = '8e068ee56a4de3d8f6a225e3300501ec8896e1e1326febbd4ce14303d5330b48'
sid = 's-1e5fec80'
image = 'sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98'
env = {k:os.environ[k] for k in ('PATH','LANG','LC_ALL','TZ') if k in os.environ}
env['HOME'] = str(base/'launcher-home')
def sha(p):
    with p.open('rb') as f: return hashlib.file_digest(f,'sha256').hexdigest()
def snap(root):
    rows = {}
    for p in root.rglob('*'):
        s=p.lstat()
        assert stat.S_ISREG(s.st_mode) or stat.S_ISDIR(s.st_mode), 'Unexpected fixture entry'
        if stat.S_ISREG(s.st_mode): rows[str(p.relative_to(root))]={'sha256':sha(p),'bytes':s.st_size}
    return rows
def docker(*args, timeout=20):
    return subprocess.run(['docker','--host','unix:///var/run/docker.sock','--config',str(base/'launcher-home'),*args],env=env,text=True,capture_output=True,timeout=timeout)
def get(*args):
    r=docker(*args); assert r.returncode == 0, r.stderr; return r.stdout.strip()
def write(name, value):
    with (out/name).open('x') as f: json.dump(value,f,indent=2); f.write('\n')
    os.chmod(out/name,0o600)
driver=json.loads((base/'driver/result.json').read_text())
wrapper=json.loads((base/'wrapper-result.json').read_text())
assert not driver['passed'] and driver['failure']['message']=='Unowned mount'
assert driver['pendingMutations']==[]
assert [r['status'] for r in driver['apiActions'] if r['method']=='POST' and r['path']=='/v1/sessions']==[201]
for row in wrapper['identities']:
    expected=row['expected']; p=Path('/proc')/str(expected['pid'])/'stat'
    if p.exists(): assert p.read_text().rsplit(') ',1)[1].split()[19]!=expected['startTicks'], 'Original controller/launcher still present'
assert get('info','--format','{{.ID}}')==driver['daemon']['id']
assert get('ps','-aq','--no-trunc').split()==[cid]
obj=json.loads(get('inspect',cid))[0]
instance=(fixture/'state/instance/instance-id').read_text().strip()
assert instance=='0a63094e-ca17-4b39-b40c-7de6e01c502e'
assert obj['Id']==cid and obj['Image']==image and obj['Name']=='/luci-'+sid
labels=obj['Config']['Labels']
assert labels['luci.instance']==instance and labels['luci.managed']=='true' and labels['luci.sandbox-id']==sid and labels['luci.session-id']==sid
hc=obj['HostConfig']; assert hc['Memory']==1073741824 and hc['NanoCpus']==1000000000 and hc['PidsLimit']==256 and not hc['Privileged']
mounts={m['Destination']:m for m in obj['Mounts']}
expected={'/home/dev/work':('projects/c054-terminal-fixture',True),'/home/dev/work/.git/worktrees':('projects/c054-terminal-fixture/.git/worktrees',False),'/home/dev/.luci-shell':('state/conversations/h/Bc7FrFANkWW3kD1zk4sSXqCOjmFyKSQ3YVXUY4YGmNY',True)}
assert set(mounts)==set(expected)
for dest,(rel,rw) in expected.items():
    m=mounts[dest]; p=fixture/rel
    assert m['Source']==str(p) and p.resolve()==p and m['RW']==rw and m['Type']=='bind'
sessions=json.loads((fixture/'state/sessions/sessions.json').read_text())['sessions']
session=next(s for s in sessions if s['id']==sid)
assert session['sandboxRef']==cid and session['sandboxId']==sid and session['projectId']=='p-ef4d52c2'
ledger_path=fixture/'state/control-plane/execution/leases.json'
ledger=json.loads(ledger_path.read_text()); lease=ledger['leases'][0]
assert len(ledger['leases'])==1 and lease['id']=='manual-0ee1faf7' and lease['sessionId']==sid and lease['sandboxId']==sid
assert lease['state']=='backend-settled' and lease['resources']=={'CPUMillis':1000,'MemoryMiB':1024}
before=snap(fixture); assert sum(x['bytes'] for x in before.values())<16*1024*1024
originals={name:sha(base/name) for name in ['driver/result.json','wrapper-result.json']}
archive=out/'private-original-fixture.tar.gz'
with tarfile.open(archive,'x:gz',dereference=False) as tar: tar.add(fixture,arcname='fixture',recursive=True)
os.chmod(archive,0o600)
assert before==snap(fixture)
write('before.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'container':{'id':cid,'image':image,'state':obj['State'],'instance':instance,'session':sid,'mounts':[{'sourceRelative':str(Path(m['Source']).relative_to(fixture)),'destination':m['Destination'],'rw':m['RW'],'type':m['Type']} for m in obj['Mounts']]},'ledgerSHA256':sha(ledger_path),'lease':lease,'files':before,'originalReports':originals,'privateArchiveSHA256':sha(archive),'privateArchiveBytes':archive.stat().st_size,'scope':'Independent operator cleanup after failed helper; acknowledged POST201, exact controller identities absent; not product recovery or capacity release.'})
# Exact immutable ID only. No forced removal or name-based fallback.
stop=docker('stop','--time','10',cid,timeout=30)
write('stop.json',{'returncode':stop.returncode,'stdout':stop.stdout,'stderr':stop.stderr})
assert stop.returncode==0
stopped=json.loads(get('inspect',cid))[0]
assert stopped['Id']==cid and not stopped['State']['Running'] and stopped['Config']['Labels']==labels
write('stopped.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'id':cid,'state':stopped['State']})
remove=docker('rm',cid,timeout=30)
write('remove.json',{'returncode':remove.returncode,'stdout':remove.stdout,'stderr':remove.stderr})
assert remove.returncode==0
absent=docker('inspect',cid)
assert absent.returncode!=0 and 'No such object' in absent.stderr
after=snap(fixture)
write('after.json',{'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'inspect':{'returncode':absent.returncode,'stdout':absent.stdout,'stderr':absent.stderr},'containers':get('ps','-aq','--no-trunc').split(),'volumes':get('volume','ls','-q').split(),'files':after,'fixtureFilesUnchanged':after==before,'originalReportsUnchanged':all(sha(base/n)==h for n,h in originals.items()),'ledgerSHA256':sha(ledger_path),'retainedLease':json.loads(ledger_path.read_text())['leases'],'scope':'Manual exact stopped-container removal only. Original backend-settled reservation retained; no product Stop, lease release, terminal test pass or automatic cleanup certification.'})
print(json.dumps({'receiptDirectory':str(out),'fixtureFiles':len(before),'unchanged':after==before,'ledgerSHA256':sha(ledger_path),'retainedChargeMiB':1024,'scriptSHA256':sha(Path(__file__))}))
