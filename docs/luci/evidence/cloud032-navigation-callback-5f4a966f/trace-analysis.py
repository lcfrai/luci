from pathlib import Path
import json,hashlib,zipfile,statistics,math,collections
HERE=Path(__file__).resolve().parent;ROOT=Path('/workspace/luci/artifacts/quality-swarm/idle-5f4a966f-navigation-callback-attempt1/driver');archive=ROOT/'action-only-trace.zip'
with zipfile.ZipFile(archive) as z:events=[json.loads(x) for x in z.read('trace.trace').splitlines()]
by_call={}
for x in events:
 if 'callId' in x:by_call.setdefault(x['callId'],[]).append(x)
labels={'attempting click action':'attempt-click','  waiting for element to be visible, enabled and stable':'wait-actionability','  element is visible, enabled and stable':'actionable','  scrolling into view if needed':'scroll-start','  done scrolling':'scroll-end','  performing click action':'perform-start','  click action done':'perform-end','  waiting for scheduled navigations to finish':'navigation-wait','  navigations have finished':'navigation-end'}
rows=[];unknown=collections.Counter();inputs=[]
for group in by_call.values():
 before=next((e for e in group if e['type']=='before'),None)
 if not before or before.get('method')!='click':continue
 selector=before.get('params',{}).get('selector','')
 kind='open-project' if 'Open project ' in selector else 'all-projects' if 'All projects' in selector else 'discard-edits' if 'Discard edits and continue' in selector else 'define' if 'Define' in selector else 'design' if 'Design' in selector else 'other'
 assert kind!='other';after=next(e for e in group if e['type']=='after');marks=[]
 for e in group:
  if e['type']=='log':
   message=e['message'];label=labels.get(message)
   if label:marks.append({'label':label,'time':e['time'],'offsetMs':e['time']-before['startTime']})
   elif message.startswith('waiting for '):marks.append({'label':'locator-wait','time':e['time'],'offsetMs':e['time']-before['startTime']})
   elif message.strip().startswith('locator resolved to '):marks.append({'label':'locator-resolved','time':e['time'],'offsetMs':e['time']-before['startTime']})
   else:unknown[message.split('<',1)[0][:80]]+=1
  if e['type']=='input':inputs.append({'callOrdinal':len(rows),'kind':kind,'keys':list(e),'point':e.get('point')})
 times={m['label']:m['time'] for m in marks};needed=['locator-wait','locator-resolved',*labels.values()];assert set(needed).issubset(times);assert len(marks)==len(needed)
 regions={'lookup':times['locator-resolved']-before['startTime'],'resolvedToAttempt':times['attempt-click']-times['locator-resolved'],'actionability':times['actionable']-times['wait-actionability'],'scroll':times['scroll-end']-times['scroll-start'],'scrollToPerform':times['perform-start']-times['scroll-end'],'perform':times['perform-end']-times['perform-start'],'scheduledNavigation':times['navigation-end']-times['navigation-wait'],'afterNavigationReturn':after['endTime']-times['navigation-end']}
 row={'ordinal':len(rows),'traceCallID':before['callId'],'kind':kind,'traceStartTime':before['startTime'],'traceEndTime':after['endTime'],'elapsedMs':after['endTime']-before['startTime'],'marks':marks,'regionsMs':regions,'attempts':sum(m['label']=='attempt-click' for m in marks),'actionabilityWaits':sum(m['label']=='wait-actionability' for m in marks),'errors':bool(after.get('error'))};rows.append(row)
assert len(rows)==40 and len(unknown)==0
kinds=sorted(set(x['kind'] for x in rows))
def dist(v):
 s=sorted(v);return {'n':len(s),'median':statistics.median(s),'p95':s[math.ceil(len(s)*.95)-1],'max':s[-1]}
summary={'traceArchiveSHA256':hashlib.sha256(archive.read_bytes()).hexdigest(),'scope':'Existing trace follow-up only;40 recorded clicks in8 extra rounds. No primary-action trace exists. Safe enum/time/point projection; locator strings/DOM excerpts/params are not published.','clicks':len(rows),'singleAttemptClicks':sum(x['attempts']==1 for x in rows),'singleSuccessfulActionabilityWaits':sum(x['actionabilityWaits']==1 for x in rows),'retryOrInstabilityLogs':0,'clickErrors':sum(x['errors'] for x in rows),'byKind':{kind:{'elapsed':dist([x['elapsedMs'] for x in rows if x['kind']==kind]),**{key:dist([x['regionsMs'][key] for x in rows if x['kind']==kind]) for key in rows[0]['regionsMs']}} for kind in kinds},'longestOpen':max((x for x in rows if x['kind']=='open-project'),key=lambda x:x['elapsedMs']),'longestAllProjects':max((x for x in rows if x['kind']=='all-projects'),key=lambda x:x['elapsedMs']),'limits':['Logs establish no retry, invisible/disabled/unstable refusal, interception or overlay recovery in these40 clicks; they do not measure every frame of geometry or rule out concurrent rendering.','Perform interval includes Playwright mouse/input protocol work and browser event processing; no internal app CPU attribution.','Trace clock is analyzed internally only; no subtraction against browser readiness clock or original full action metrics.']}
(HERE/'trace-click-projection.json').write_text(json.dumps({'rows':rows,'inputMetadata':inputs},indent=2)+'\n');(HERE/'trace-analysis.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary))
