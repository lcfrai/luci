from pathlib import Path
import json,hashlib
base=Path(__file__).resolve().parent
original=Path('/workspace/luci/artifacts/quality-swarm/idle-d20ebd29-autonomous-attempt2/driver/result.json')
bytes_=original.read_bytes();assert len(bytes_)<64*1024**2
raw=json.loads(bytes_);projection=json.loads((base/'actual/readiness.projection.json').read_text());rows={r['index']:r for r in projection['switches']}
selected=[32,104,156,232];output=[]
routes={'project-overview','project-specification','project-orchestrator','project-swarm-queue','project-runs','swarm-scheduler'}
events={'click','pointerdown','pointerup','mousedown','mouseup'}
for index in selected:
 row=rows[index];start=row['home']['clickAt'];end=row['driverExpectedDOMAt'];origin=row['home']['timeOrigin'];resources=[];clicks=[]
 for window in raw['browserEventWindows']:
  latency=window.get('latency') or {}
  if latency.get('timeOrigin')==origin:
   for event in latency.get('rows',[]):
    if event.get('kind')=='resource' and event.get('route') in routes and start-10<=event.get('at',-1)<=end:
     resources.append({key:event.get(key) for key in ['route','at','duration','responseStatus','requestStart','responseStart','responseEnd','transferSize','encodedBodySize','decodedBodySize']})
  if window.get('timeOrigin')==origin:
   for event in window.get('events',[]):
    if event.get('name') in events and start-10<=event.get('startTime',-1)<=end:
     clicks.append({key:event.get(key) for key in ['name','startTime','duration','processingStart','processingEnd','interactionId']})
 output.append({'index':index,'browserClockTimeOrigin':origin,'anchors':{'homeClickAt':start,'homeObservedAt':row['home']['observedAt'],'openClickAt':row['define']['clickAt'],'defineObservedAt':row['define']['observedAt'],'driverExpectedDOMAt':end},'resources':resources,'eventTiming':clicks})
assert original.read_bytes()==bytes_
value={'originalResultSHA256':hashlib.sha256(bytes_).hexdigest(),'scope':'Four preselected primary rows: two large observed-to-driver gaps, one late sample with blind gap, and largest Home-observed-to-Open interval. No raw URL/project ID/body or inferred request identity.','limitations':['Resource route classes do not independently identify a specific project request; concurrent/cancelled reads may be present.','Event Timing collection uses a16ms threshold; missing entries are not proof of no event work. Duration is not physical paint or exclusive CPU time.','No primary Playwright actionability trace or long-task collector exists in this run; followup trace is a different cohort.'], 'rows':output}
(base/'stage-markers.json').write_text(json.dumps(value,indent=2)+'\n')
print(json.dumps([{'index':r['index'],'anchors':r['anchors'],'resources':[{'route':v['route'],'at':v['at'],'duration':v['duration'],'status':v['responseStatus']} for v in r['resources']],'events':r['eventTiming']} for r in output],indent=2))
