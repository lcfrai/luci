from pathlib import Path
import hashlib,json,subprocess,datetime,os
HERE=Path(__file__).resolve().parent
ROOT=Path('/workspace/luci/artifacts/quality-swarm/settled-evaluate-edd0cb12-attempt1');LIVE=Path('/workspace/.luci-tools/cloud032-settled-evaluate-control-v1')
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
r=json.loads((ROOT/'driver/result.json').read_text());inv=json.loads((ROOT/'invocation.json').read_text());j=[json.loads(x) for x in (ROOT/'driver/diagnostic-journal.jsonl').read_text().splitlines()]
assert sha(ROOT/'driver/result.json')=='b1b04e879105e641702f9ae37652847e912f9515f31ce70c2062b7980223cd18'
files={p.relative_to(ROOT).as_posix():sha(p) for p in ROOT.rglob('*') if p.is_file()}
def inspect(pid,start,kind):
 o={'kind':kind,'pid':pid,'recordedStartTicks':int(start)}
 try:
  s=Path('/proc',str(pid),'stat').read_text().rsplit(') ',1)[1].split();o.update(observedState=s[0],observedStartTicks=int(s[19]),parentPid=int(s[1]),processGroup=int(s[2]),sameIdentity=int(s[19])==int(start));o['classification']='pid-reused' if not o['sameIdentity'] else 'literal-zombie' if s[0]=='Z' else 'still-present'
 except FileNotFoundError:o['classification']='absent'
 except (OSError,ValueError,IndexError) as e:o.update(classification='unreadable-or-unknown',errorType=type(e).__name__)
 return o
identities=[inspect(x['pid'],x['startTicks'],x['event']) for x in j if x['event'] in ['driver-start','controller-start']]
identities += [inspect(x['pid'],x['start'],'browser-setup') for x in r['browserProcessIdentitiesAtSetup']]
def docker(*args):return subprocess.check_output(['docker','--host','unix:///var/run/docker.sock',*args],text=True,timeout=15).strip()
containers=docker('ps','-aq').splitlines();volumes=docker('volume','ls','-q').splitlines();engine=docker('info','--format','{{.ID}}');image=docker('image','inspect',inv['command'][inv['command'].index('--image')+1],'--format','{{.Id}}')
protected_paths={'launcher':LIVE/'launch.py','binding':LIVE/'actual/external-source-binding.json','capture':LIVE/'template-capture.json','provenance':Path(inv['command'][inv['command'].index('--provenance')+1]),'image-provenance':Path(inv['command'][inv['command'].index('--image-provenance')+1])}
protected_paths.update({k:LIVE/'actual'/k.removeprefix('source/') for k in inv['protectedInputSHA256'] if k.startswith('source/')})
protected={k:sha(p)==inv['protectedInputSHA256'][k] for k,p in protected_paths.items()}
archive=json.loads(protected_paths['provenance'].read_text());archive_match=sha(Path(archive['archive']))==archive['archiveSHA256']
files_after={p.relative_to(ROOT).as_posix():sha(p) for p in ROOT.rglob('*') if p.is_file()}
out={'schema':1,'observedAt':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Independent read-only exact recorded identities and explicit daemon observation; no mutation, retry, relaunch or whole-machine census.','rawResultSHA256':sha(ROOT/'driver/result.json'),'identities':identities,'containers':len(containers),'volumes':len(volumes),'engineIdentityMatches':hashlib.sha256(engine.encode()).hexdigest()==inv['engineIdentitySHA256'],'imageIdentityMatches':image==inv['command'][inv['command'].index('--expected-image-id')+1],'protectedInputsAllMatch':all(protected.values()),'protectedInputs':protected,'originalArchiveSHA256Matches':archive_match,'originalFilesStable':files==files_after,'originalFileSHA256':files,'journalRows':len(j),'journalSequenceValid':[x['sequence'] for x in j]==list(range(1,len(j)+1)),'journalLastEvent':j[-1]['event'],'fixtureAndPrivateLedger':'Driver reports successful fixture removal; original private fixture path is not retained in public evidence, so private ledger contents and exact removed directory are not independently inspectable after run. No ledger mutation or guessed path.','driverAssertions':{k:r.get(k) for k in ['fixtureRemoved','cleanup','dockerRequestsDrained','dockerMutationStateCertain','dockerTransportNotStarted','runtimeCleanup','providerCalls','externalModelCalls']},'limits':'Only three browser identities captured at setup; no claim that every later Chromium child is enumerated. Literal zombies are reported and not killed.'}
(HERE/'independent-closure.json').write_text(json.dumps(out,indent=2)+'\n')
print(json.dumps({k:out[k] for k in ['identities','containers','volumes','engineIdentityMatches','imageIdentityMatches','protectedInputsAllMatch','originalArchiveSHA256Matches','originalFilesStable','journalRows','journalSequenceValid']}))
