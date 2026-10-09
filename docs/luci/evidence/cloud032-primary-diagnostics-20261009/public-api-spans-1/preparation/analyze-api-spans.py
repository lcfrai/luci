#!/usr/bin/env python3
"""Analyze allowlisted public callback pairs in one Node clock; no raw app data."""
from pathlib import Path
import argparse,json,hashlib,math
NAMES={'project-switch':['locator.click','locator.waitFor','locator.click','locator.waitFor','locator.evaluate','locator.inputValue','page.evaluate'],'input':['locator.press','page.waitForFunction','page.evaluate'],'draft-reset':['locator.click','locator.click','locator.click','locator.inputValue','page.evaluate']}
LABELS={'project-switch':['all-projects-click','overview-heading-wait','open-project-click','idea-visible-wait','idea-value-evaluation','idea-value-read','two-frame-witness'],'input':['key-press','expected-value-driver-wait','two-frame-witness'],'draft-reset':['design-click','discard-click','define-click','idea-value-read','two-frame-witness']}
def main():
 p=argparse.ArgumentParser();p.add_argument('--receipt',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args();raw=a.receipt.read_bytes();assert len(raw)<=64*1024**2;d=json.loads(raw);s=d['publicAPISpans'];assert s['complete'] and not s['overflow'] and not s['problems']
 items=[x for x in d['interactions'] if x['phase']=='idle-interactive-calibration'];assert len(items)==len(s['actions'])==400;assert len(s['entries'])==3600
 out=[];cursor=0
 for ordinal,(item,action) in enumerate(zip(items,s['actions']),1):
  assert action['ordinal']==ordinal and action['kind']==item['kind'] and action['complete'];assert action['firstEntry']==cursor
  kind=item['kind'];rows=s['entries'][cursor:cursor+action['entryCount']];cursor+=len(rows);assert len(rows)==len(NAMES[kind])*2
  calls=[];previous=0
  for i,(api,label) in enumerate(zip(NAMES[kind],LABELS[kind])):
   begin,end=rows[2*i:2*i+2]
   assert begin['ordinal']==end['ordinal']==ordinal and begin['api']==end['api']==api and begin['state']=='started' and end['state']=='succeeded'
   left,right=begin['nodeOffsetMs'],end['nodeOffsetMs'];assert isinstance(left,(int,float)) and isinstance(right,(int,float)) and math.isfinite(left) and math.isfinite(right)
   assert previous<=left<=right<=item['elapsedMs'];calls.append({'api':api,'meaning':label,'startOffsetMs':left,'endOffsetMs':right,'callbackSpanMs':right-left,'unattributedGapBeforeMs':left-previous});previous=right
  out.append({'ordinal':ordinal,'kind':kind,'originalElapsedMs':item['elapsedMs'],'success':item['success'],'calls':calls,'unattributedTailMs':item['elapsedMs']-previous})
 assert cursor==len(s['entries'])
 result={'version':1,'scope':'Public API callback spans in same Node clock as original400 actions; not app CPU/render/retry/physical-input causality','receiptSHA256':hashlib.sha256(raw).hexdigest(),'primaryActions':400,'publicCalls':1800,'callbackEntries':3600,'actions':out,'limitations':['The public Logger API is deprecated/incomplete; exact expected observed sequences validated, no claim that all internal work is logged','No args/hints/values/selectors/results are stored','No browser clock subtraction or overhead correction','Public locator.waitFor timing is driver waiting; it does not prove first renderer-ready time or count its internal retries','Callback span and inter-call gaps are diagnostic components of the unchanged original elapsed duration; never substitute them for acceptance measurements']}
 text=json.dumps(result,indent=2)+'\n';assert len(text.encode())<=2*1024**2
 with a.output.open('x') as f:f.write(text)
 a.output.chmod(0o600)
if __name__=='__main__':main()
