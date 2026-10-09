"""Prepared ONLY. Selectively retire two old image tags; never prune or remove archives."""
from pathlib import Path
import argparse,json,os,re,subprocess,sys
from operation_common import ROOT,Journal,need,read_bounded,pinned_json,hash_file,identity,quiescence,process_scan,require_idle,Refusal
OLD='sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98'
BASE='sha256:9e0fd9f731408c4405c4b9e97308bf807c8088cf521a138c0b1921667a831351'
TAGS=('luci/sandbox:dev','luci/sandbox:verified-candidate-20261008t140025z-a4a2e82b')
ARCHIVE=Path('/workspace/.luci-tools/runtime-archive-1f845cfd2b45/verified-luci-sandbox-dev.tar.gz')
ARCHIVE_SHA='7caf7aab012cd4a5bf2ace5c8a2e12e617b4afbaef20f6b3db64586a24e9fcf3'
HIST=ARCHIVE.parent/'receipt.json';HIST_SHA='a8e164024cf84d8bfcb90f8693964ba8ee5b53dddb8f02bc857ede707a6d5860'
PROOF=Path('/workspace/.luci-tools/cloud054-runtime-space-plan/retained-333eff-archive-inspection.json');PROOF_SHA='aebb6e154ce4f367f245c7b36fb11cff8fb9d94ba3405bce3e5c3a32ca521774'
def removal_command(tag):
 need(tag in TAGS,'unapproved-removal-tag');return ['image','rm','--no-prune',tag]
def validate_archive_proof(historical,proof):
 need(historical['candidateImage']==OLD and historical['archiveVerification']['candidateImage']==OLD,'archive-image-mismatch');v=historical['archiveVerification']
 need(v['status']=='passed' and v['gzipCRCVerified'] and v['archiveSHA256']==ARCHIVE_SHA and v['archiveBytes']==1013367768,'archive-historical-proof-incomplete')
 need(proof['archiveCRCAndFullReadPassed'] and proof['sourceArchiveUnchanged'] and proof['archiveSHA256']==ARCHIVE_SHA and proof['archiveBytes']==v['archiveBytes'],'archive-proof-incomplete')
 need(proof['image']==OLD and proof['configSHA256']==OLD.split(':')[1] and proof['manifest']['RepoTags']==['luci/sandbox:dev'],'archive-config-or-tags-mismatch')
 need(proof['config']['rootfs']['diff_ids']==v['layerDiffIDs'] and not v['configComparison']['differences'],'archive-config-drift');return v
def validate_inventory(inventory,new_image,remaining=TAGS):
 need(new_image not in (OLD,BASE) and re.fullmatch(r'sha256:[a-f0-9]{64}',new_image),'new-image-pin-invalid');need(BASE in inventory and new_image in inventory,'protected-image-missing')
 need(OLD in inventory and set(inventory[OLD])==set(remaining),'old-tag-set-drift')
 for image,tags in inventory.items():
  if image!=OLD:need(not(set(tags)&set(TAGS)),'old-tag-reassigned')
def validate_preserved(before,after,remaining):
 expected={k:sorted(v) for k,v in before.items() if k!=OLD};actual={k:sorted(v) for k,v in after.items() if k!=OLD};need(actual==expected,'unrelated-image-or-tag-changed')
 if remaining:need(set(after.get(OLD,[]))==set(remaining),'unexpected-old-tags-after-removal')
 else:need(OLD not in after,'old-image-remains-no-prune-conflict')
class Docker:
 def __init__(self,journal):
  config=journal.path/'docker-config';config.mkdir(mode=0o700);home=journal.path/'home';home.mkdir(mode=0o700)
  self.env={k:os.environ[k] for k in ['PATH','LANG','LC_ALL','TZ'] if k in os.environ};self.env.update(HOME=str(home),DOCKER_CONFIG=str(config));self.prefix=['docker','--host','unix:///var/run/docker.sock']
 def __call__(self,*args):
  p=subprocess.run(self.prefix+list(args),env=self.env,capture_output=True,timeout=30)
  need(len(p.stdout)<=4*1024*1024 and len(p.stderr)<=65536,'docker-response-bound');need(p.returncode==0,'docker-command-failed-or-uncertain');return p.stdout.decode().strip()
 def inventory(self):
  rows=self('image','ls','--all','--no-trunc','--format','{{json .}}').splitlines();need(len(rows)<=4096,'image-count-bound');out={}
  for line in rows:
   o=json.loads(line);image=o['ID'];need(re.fullmatch(r'sha256:[a-f0-9]{64}',image),'image-id-unavailable');tags=out.setdefault(image,set())
   if o['Repository']!='<none>' and o['Tag']!='<none>':tags.add(o['Repository']+':'+o['Tag'])
  return {k:sorted(v) for k,v in out.items()}
def main(argv=None):
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--execute-retirement',action='store_true');p.add_argument('--new-image',required=True);p.add_argument('--candidate-receipt',type=Path,required=True);p.add_argument('--candidate-receipt-sha256',required=True);p.add_argument('--quiescence',type=Path,required=True);p.add_argument('--quiescence-sha256',required=True);p.add_argument('--receipt',type=Path,required=True);a=p.parse_args(argv)
 need(a.execute_retirement,'explicit-execute-retirement-required');need(a.receipt.parent==ROOT and not os.path.lexists(a.receipt),'fresh-receipt-under-operation-root-required')
 j=Journal(a.receipt,'retire-exact-old333eff-tags',[Path(__file__).resolve(),ROOT/'operation_common.py'])
 try:
  historical=pinned_json(HIST,HIST_SHA);proof=pinned_json(PROOF,PROOF_SHA);v=validate_archive_proof(historical,proof)
  archive_sha,archive_stat=hash_file(ARCHIVE);need(archive_sha==ARCHIVE_SHA and archive_stat.st_size==v['archiveBytes'],'preserved-archive-current-bytes-mismatch')
  candidate=pinned_json(a.candidate_receipt,a.candidate_receipt_sha256);need(candidate.get('status')=='passed' and candidate.get('candidateImage')==a.new_image and candidate.get('baseImage')==BASE,'new-candidate-receipt-contract');need(candidate.get('sourceStable') is True,'new-candidate-source-unverified')
  coordination=quiescence(a.quiescence,a.quiescence_sha256,'cloud054-exclusive-docker');scan=process_scan(docker=True);j.save('preflight-observations',archive={'path':str(ARCHIVE),'sha256':archive_sha,'bytes':archive_stat.st_size,'dockerLoadVerified':False},coordination=coordination,processObservation=scan);require_idle(scan)
  docker=Docker(j);engine=docker('info','--format','{{.ID}}');need(bool(engine),'engine-id-unavailable');need(docker('ps','--all','--quiet','--no-trunc')=='','container-present');before=docker.inventory();validate_inventory(before,a.new_image);j.save('ready',beforeImages=before,engineID=engine,protectedImages=[BASE,a.new_image])
  remaining=list(TAGS)
  for tag in TAGS:
   quiescence(a.quiescence,a.quiescence_sha256,'cloud054-exclusive-docker');fresh=process_scan(docker=True);require_idle(fresh);need(docker('info','--format','{{.ID}}')==engine,'daemon-changed');need(docker('ps','--all','--quiet','--no-trunc')=='','container-present');current=docker.inventory();validate_preserved(before,current,remaining);validate_inventory(current,a.new_image,remaining)
   need(docker('image','inspect','--format','{{.Id}}',tag)==OLD,'tag-id-changed');command=removal_command(tag);j.save('removal-pending',pendingCommand=command,completedRemovals=[x for x in TAGS if x not in remaining],lastProcessObservation=fresh)
   docker(*command);remaining.remove(tag);after=docker.inventory();validate_preserved(before,after,remaining);j.save('removal-observed',pendingCommand=None,completedRemovals=[x for x in TAGS if x not in remaining],afterImages=after)
  need(identity(ARCHIVE.lstat())==identity(archive_stat),'archive-metadata-changed');need(docker('ps','--all','--quiet','--no-trunc')=='','container-appeared');j.finish(True,finalImages=docker.inventory(),archiveUnmodifiedMetadata=True,scope='Exact two non-force no-prune tag removals only; no fixture/ledger mutation, no product charge release, no fresh Docker-load proof');return 0
 except Exception as e:
  j.save('failed',failureType=type(e).__name__,failureCode=str(e) if isinstance(e,Refusal) else 'operation-failed-or-uncertain',partialMutationPossible=bool(j.value.get('pendingCommand') or j.value.get('completedRemovals')));return 1
if __name__=='__main__':
 try:sys.exit(main())
 except Exception as e:print(json.dumps({'passed':False,'code':str(e) if isinstance(e,Refusal) else 'operation-preflight-failed'}));sys.exit(1)
