"""Portable read-only checksum and receipt assertions; no external/private reads."""
from pathlib import Path
import hashlib,json
ROOT=Path(__file__).resolve().parent;count=0
def check(ok,label):
 global count
 count+=1
 if not ok:raise AssertionError(label)
def data(n):return json.loads((ROOT/n).read_bytes())
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def main():
 manifest=data('publication-manifest.json')
 for row in manifest['members']:
  p=ROOT/row['path'];check(p.is_file()and not p.is_symlink(),'member type:'+row['path']);check(p.stat().st_size==row['bytes']and sha(p)==row['sha256'],'member bytes:'+row['path'])
 for row in data('original-bindings.json')['bindings']:
  p=ROOT/row['path'];check(p.stat().st_size==row['bytes']and sha(p)==row['sha256'],'original binding:'+row['path'])
 pm=data('preparation/source-only-manifest-v2.json')
 for row in pm['members']:
  p=ROOT/'preparation'/row['path'];check(p.stat().st_size==row['bytes']and sha(p)==row['sha256'],'preparation pin:'+row['path'])
 old=data('preparation/initial-manifest-including-runtime-sink.json');excluded={x['path']for x in old['members']}-{x['path']for x in pm['members']};check(excluded=={'place-watchdog-305863734286611/stdout.log','place-watchdog-305863734286611/stderr.log','place-watchdog-305863734286611/wrapper.json'},'initial manifest scope correction')
 r=data('archive/receipt.json');image='sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8';check(r['status']=='published'and r['candidateImage']==image,'published image');check(r['sourceCommit']=='29931afc5b31ae9b9bd7b8973a7e3fdb840609a3','source299');v=r['archiveVerification'];check(v['status']=='passed'and v['gzipCRCVerified']is True,'archive verification status');check(v['archiveSHA256']=='637d1d02c701be5f4327bb76dcb58335e1665b27945e3f1cb08bb825c5fd96fd'and v['archiveBytes']==1013403794,'declared archive subject');check(r['dockerLoadVerified']is False,'original load limitation retained');check(r['budget']['perFilesystemExportFloorBytes']==3905250841 and r['budget']['gzipLimitBytes']==1288490188,'unchanged floor and outputcap')
 for n,h in r['newPins'].items():check(sha(ROOT/'archive'/('new-'+n))==h,'new pin:'+n)
 cr=data('subjects/runtime-build-and-export-receipt.json');check(cr['sourceCommit']=='5f4a966f6d70264c92ee2176dde912e1b43fc3c7'and cr['candidateImage']==image,'original runtime build');check(len(cr['sourcesBefore'])==106 and cr['sourcesBefore']==cr['sourcesAfter'],'captured runtime inputs');check(sha(ROOT/'subjects/runtime-build-and-export-receipt.json')==r['candidateReceiptSHA256']==r['reviewedRuntimeEvidence']['sha256'],'buildcontext only')
 config=ROOT/'archive/metadata-76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8.raw';check('sha256:'+sha(config)==image,'raw image config identity');env=json.loads(config.read_bytes())['config'].get('Env',[]);check({x.split('=',1)[0]for x in env}=={'PATH','NODE_VERSION','YARN_VERSION'},'finite environment names only')
 for n in ['entrypoint.sh','sandboxd']:
  info=v['verifiedFiles']['usr/local/bin/'+n];check(info==cr['inputFiles'][n],'exact runtime file:'+n)
 restored=data('restore-check/result.json');check(restored['passed']is True and restored['exitCode']==0 and restored['restoredImage']==image,'currentdaemon tag restoration');check(restored['scriptSHA256']==r['newPins']['install.sh'],'tested installer subject');check('existing daemon' in restored['scope']and 'Not a fresh-daemon' in restored['scope'],'restore limit')
 configsave=data('configuration/save-public.json');check(configsave['status']=='saved'and configsave['requires_publish']is True,'save versus publish');check(configsave['requestFields']==['install_script','start_skill']and configsave['repoMembershipChangeRequested']is False and configsave['networkSecretEnvironmentChangeRequested']is False,'save scope');check(configsave['preservedRepositories'][0]['ref']==r['sourceCommit'],'repository preservation');check(configsave['scripts']=={n:r['newPins'][n]for n in ['install.sh','start.md']},'saved script subjects')
 note=data('saved-field-export/representation-note.json');check(note['savedInstallCharacters']==11169 and note['savedStartCharacters']==20525,'original saved character counts');check(len((ROOT/'saved-field-export/install.sh').read_text())==11170 and len((ROOT/'saved-field-export/start.md').read_text())==20526,'export representations extra newline');check(len((ROOT/'archive/new-start.md').read_bytes())==12852 and len((ROOT/'archive/new-start.md').read_text())==12848,'new startup UTF8 distinction')
 for p in (ROOT/'operations/watchdogs').glob('*/wrapper.json'):
  d=json.loads(p.read_bytes());check(d['passed']is True and d['exitCode']==0,'completed archive/placement wrapper:'+p.parent.name)
 for p in (ROOT/'operations/tool-watchdogs').glob('*/wrapper.json'):
  d=json.loads(p.read_bytes());check(d['exitCode']==0 and not d['timedOut'],'completed tool wrapper:'+p.parent.name)
 for p in (ROOT/'operations/tools').glob('*/*-finished.json'):check(json.loads(p.read_bytes())['passed']is True,'completed optional tool:'+p.parent.name)
 finals=[]
 for p in (ROOT/'operations').glob('runtime760-placement-*/*-finished.json'):
  d=json.loads(p.read_bytes());check(d['passed']is True,'completed placement:'+p.parent.name);finals.append(d)
 f=next(x for x in finals if x['mode']=='333-to-tmp');check(f['freeAfter']=={'/workspace':7339724800,'/tmp':1247367168},'original final observation');check(f['freeAfter']['/workspace']>=6736759858 and f['freeAfter']['/tmp']<3905250841,'runtime and export floors separate')
 tests=data('preparation/final-controls-result.json');check(tests['exitCode']==0 and tests['expectedTests']==13,'final controlled count');failed=data('preparation/control-attempt1/controls-result.json');check(failed['exitCode']==1 and failed['expectedTests']==8,'original fixture failure retained')
 print(json.dumps({'passed':True,'checks':count,'scope':'Portable public small-byte/receipt consistency only; excluded archives/tools/private fields not fetched or rehashed; no runtime actions.'},indent=2))
if __name__=='__main__':main()
