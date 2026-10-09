#!/usr/bin/env python3
"""Prepare only bounded Node command markers, never launch an app or mutate repo."""
from pathlib import Path
import hashlib,json,difflib
HERE=Path(__file__).resolve().parent
BASE=Path('/workspace/.luci-tools/cloud055-readiness-stage-gate-v2/capsule/actual')
EXPECTED={'interaction-metrics.mjs':'ec0bc9d06fda6636cbba6975e05b9c66979ca189b59aa38c373b37a318b79502','latency-diagnostics.mjs':'a2d787493f0eeaeb95570c7f211ee69c649ff1b7247feec171f2118ac3a42c96'}
def sha(b):return hashlib.sha256(b).hexdigest()
def replace_once(source,before,after):
 assert source.count(before)==1,before
 return source.replace(before,after)
def main():
 for directory in ['original','candidate']:(HERE/directory).mkdir(exist_ok=False)
 records=[];patch=[]
 for name,expected in EXPECTED.items():
  data=(BASE/name).read_bytes();assert sha(data)==expected
  before=data.decode();after=before
  if name=='interaction-metrics.mjs':
   after=replace_once(after,'await verify();', 'await verify(diagnostic.mark);')
   after=replace_once(after,"mark('overview-heading-ready');await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();", "mark('overview-heading-ready');mark('open-project-click-start');await page.getByRole('button',{name:'Open project '+owner.owner.name,exact:true}).click();")
   after=replace_once(after,"mark('open-project-click-end');},async()=>{", "mark('open-project-click-end');},async mark=>{")
   after=replace_once(after,"exact:true});await field.waitFor();", "exact:true});await field.waitFor();mark('idea-field-wait-end');")
   after=replace_once(after,"}),values.get(owner.owner.id));\n   assert.equal(await field.inputValue()", "}),values.get(owner.owner.id));mark('idea-context-evaluate-end');\n   assert.equal(await field.inputValue()")
  else:
   after=replace_once(after,"'overview-heading-ready','open-project-click-end'].includes(stage)", "'overview-heading-ready','open-project-click-end','open-project-click-start','idea-field-wait-end','idea-context-evaluate-end'].includes(stage)")
  (HERE/'original'/name).write_bytes(data);(HERE/'candidate'/name).write_text(after)
  records.append({'path':name,'beforeSHA256':expected,'afterSHA256':sha(after.encode()),'beforeBytes':len(data),'afterBytes':len(after.encode())})
  patch.extend(difflib.unified_diff(before.splitlines(keepends=True),after.splitlines(keepends=True),fromfile='original/'+name,tofile='candidate/'+name))
 (HERE/'candidate.patch').write_text(''.join(patch))
 (HERE/'source-binding.json').write_text(json.dumps({'schemaVersion':1,'baseManifestSHA256':'789ecae04845d42c32378143e3013fac778d3d91c963899c2219e60cc656c9e0','baseObservedProduct':'d20ebd292f3f1de04d6726bab0998c210987bcb5','baseObservedHelper':'edd0cb121ff6847f681ab6bf41509a497f043d41','prospectiveRunProduct':None,'prospectiveRunHelper':None,'scope':'External source-only delta pending parent integration/reseal/review. Exactly three synchronous Node marks, no extra browser RPC or primary action. No installed run authorized by this script.','files':records,'nodeStageCap':12,'newClockReadsPerSuccessfulSwitch':3,'newClockReadsPerInputOrReset':0,'browserAndActionLimitsUnchanged':True},indent=2)+'\n')
 print(json.dumps({'prepared':True,'files':records,'installedRun':False}))
if __name__=='__main__':main()
