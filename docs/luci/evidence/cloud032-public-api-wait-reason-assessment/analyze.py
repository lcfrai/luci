#!/usr/bin/env python3
"""Offline coverage inspection only; never recover/guess omitted API log text."""
import argparse, collections, gzip, hashlib, io, json
from pathlib import Path

PINS={
 'analysis/public-api-spans.original-projection.json':'83e88086cc7745c67014c95092970bf57dd9c224f1378ed5320c7b655fbbda5b',
 'result.projection.json.gz':'429bee86031eb1321f54e196e14f210cf79a15a572102bb6336d92ac72a89846',
 'sources/accepted/public-api-spans.mjs.gz':'7b8647f92afcf7feffdbc05004af3fb6eff33904dd531dd00168080c233e60bb',
}

def unzip(value):
 with gzip.GzipFile(fileobj=io.BytesIO(value)) as stream:value=stream.read(2*1024**2+1)
 assert len(value)<=2*1024**2
 return value

def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument('--input',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
 assert not a.output.exists(),'Fresh output required'
 raw={};bindings=[]
 for n,pin in PINS.items():
  f=a.input/n;assert f.stat().st_size<=1024**2
  b=f.read_bytes();assert hashlib.sha256(b).hexdigest()==pin
  raw[n]=b;bindings.append({'path':n,'bytes':len(b),'sha256':pin})
 logger=unzip(raw['sources/accepted/public-api-spans.mjs.gz'])
 assert hashlib.sha256(logger).hexdigest()=='994468a342b44b4bb41294242ccfcad1281331d5f59229e1cd0f05bc9e5a09f9'
 document=json.loads(raw['analysis/public-api-spans.original-projection.json']);spans=document['value']
 result=json.loads(unzip(raw['result.projection.json.gz']));actions=result['interactions']
 assert len(actions)==len(spans['actions'])==400 and len(spans['entries'])==137
 assert not spans['complete'] and spans['overflow'] and spans['problems']==['unrecognized-api-message']*100
 assert all(set(e)=={'ordinal','api','state','nodeOffsetMs'} for e in spans['entries'])
 rows=[]
 for ordinal,(action,coverage) in enumerate(zip(actions,spans['actions']),1):
  assert coverage['ordinal']==ordinal and coverage['kind']==action['kind']
  if action['kind']!='project-switch':continue
  assert action['originalIndex']==ordinal-1
  entries=[e for e in spans['entries'] if e['ordinal']==ordinal]
  assert len(entries)==coverage['entryCount']
  stages=action['diagnostics']['nodeStages']
  assert [s['stage'] for s in stages]==['action-start','all-projects-click-end','overview-heading-ready','open-project-click-end','action-end','verify-start','verify-end','frames-start','frames-end']
  rows.append({'originalIndex':ordinal-1,'ordinal':ordinal,'success':action['success'],'elapsedMs':action['elapsedMs'],
               'over300':action['elapsedMs']>300,'originalNodeStages':stages,'loggerActionComplete':coverage['complete'],
               'loggerEntryCount':len(entries),'originalPublicAPIEntries':entries,
               'internalWaitReason':'unavailable-not-recorded','absenceDoesNotProveNoRetry':True})
 assert len(rows)==100
 slow=[r for r in rows if r['over300']]
 output={'scope':'Existing failed public API logger, lifecycle coverage only; no duration or retry-absence attribution.',
         'inputBindings':bindings,'originalResult':document['originalResult'],'acceptedLoggerDecodedSHA256':hashlib.sha256(logger).hexdigest(),
         'originalFailedCapture':{'complete':spans['complete'],'overflow':spans['overflow'],'limits':spans['limits'],
                                 'entries':len(spans['entries']),'entryBytes':spans['bytes'],'actions':len(spans['actions']),
                                 'problemCodes':spans['problems'],'problemCounts':dict(collections.Counter(spans['problems']))},
         'coverage':{'completeActionOrdinals':[r['ordinal'] for r in spans['actions'] if r['complete']],
                     'partialActions':[r for r in spans['actions'] if not r['complete'] and r['entryCount']],
                     'zeroEntryActions':sum(r['entryCount']==0 for r in spans['actions']),
                     'switches':100,'completeSwitches':sum(r['loggerActionComplete'] for r in rows),'switchesOver300':len(slow),
                     'slowSwitchesWithAnyLoggerEntries':sum(r['loggerEntryCount']>0 for r in slow),
                     'slowSwitchOrdinals':[r['ordinal'] for r in slow]},
         'all100SwitchRows':rows,
         'limitations':['Logger accepted lifecycle method names only; no internal visibility/stability/interception/locator/dispatch reasons retained.',
                        'All100 unknown-message codes remain unclassified; original text was never retained.',
                        'No Playwright actionability trace was enabled in this attempt. CDP/CPU capture is not its substitute.',
                        'Other follow-up actionability traces belong to different episodes and cannot fill the absent primary logs.',
                        'All original spans, failures and 300/100 targets remain unchanged; no causal inference, subtraction or new harness.']}
 a.output.mkdir(parents=True)
 (a.output/'assessment.json').write_text(json.dumps(output,indent=2)+'\n')
 (a.output/'original-logger-projection.json').write_bytes(raw['analysis/public-api-spans.original-projection.json'])
 (a.output/'accepted-public-api-spans.mjs').write_bytes(logger)
 print(json.dumps(output['coverage']))

if __name__=='__main__':main()
