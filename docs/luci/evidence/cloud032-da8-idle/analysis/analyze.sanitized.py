#!/usr/bin/env python3
"""Allowlisted numerical extraction; actual trace stays private regardless of scan."""
from pathlib import Path
from collections import defaultdict,Counter
import hashlib,json,math,re,zipfile,datetime

BASE=Path('<PRIVATE_ORIGINAL_DIAGNOSTIC_DIRECTORY>')
OUT=Path('<PRIVATE_ANALYSIS_OUTPUT_DIRECTORY>');OUT.mkdir(exist_ok=True,mode=0o700)
ARMS={'actual':Path('<PRIVATE_DA8_RUN_DIRECTORY>')}
def sha(data):return hashlib.sha256(data).hexdigest()
def stats(values):
    values=sorted(values)
    if not values:return {'count':0}
    return {'count':len(values),'p50Ms':values[math.ceil(len(values)*.5)-1],'p95Ms':values[math.ceil(len(values)*.95)-1],'p99Ms':values[math.ceil(len(values)*.99)-1],'maxMs':values[-1]}
def click_kind(selector):
    if 'All projects' in selector:return 'all-projects'
    if 'Open project mixed-project-' in selector:return 'open-project'
    if 'Discard edits and continue' in selector:return 'discard'
    if 'name="Design"s' in selector:return 'design'
    if 'name="Define"s' in selector:return 'define'
    return 'other-click'
def retry_class(message):
    for needle,name in [('retrying click action','retry'),('element is not stable','unstable'),('intercepts pointer events','intercepted'),('element was detached','detached'),('Timeout','timeout')]:
        if needle in message:return name
    return None

output={'version':1,'scope':'Original immutable primary metrics retained. One actual da8 packaged diagnostic arm without the full Linux sampler; original browser collectors unchanged. No subtraction or causal comparison claim. Trace follow-up is 8 extra rounds, not the primary 100.','arms':{}}
for arm,path in ARMS.items():
    data=json.loads((path/'result.json').read_text());result={'resultSHA256':sha((path/'result.json').read_bytes()),'groups':{},'trace':{}}
    grouped=defaultdict(list)
    for row in data['interactions']:grouped[row['phase']+':'+row['kind']].append(row)
    for key,rows in grouped.items():
        spans=defaultdict(list);largest=[]
        for index,row in enumerate(rows):
            marks={m['stage']:m['offsetMs'] for m in row['diagnostics']['nodeStages']};parts={}
            for label,start,end in [('action','action-start','action-end'),('verify','verify-start','verify-end'),('framesRPC','frames-start','frames-end'),('allProjectsClick','action-start','all-projects-click-end'),('overviewHeading','all-projects-click-end','overview-heading-ready'),('openProjectClick','overview-heading-ready','open-project-click-end')]:
                if start in marks and end in marks:parts[label]=marks[end]-marks[start];spans[label].append(parts[label])
            frames=row['diagnostics']['browserFrames']
            if frames:
                spans['browserFinalFrames'].append(frames['secondFrameAt']-frames['requestedAt'])
                refs=frames.get('references')or{};expected=refs.get('expectedDOM');prior=refs.get('input' if row['kind']=='input' else 'pointer')
                if expected and prior and expected['actionKind']==row['kind']:spans['browserLatestDispatchToExpectedDOM'].append(expected['at']-prior['at'])
            largest.append({'index':index,'at':row['at'],'elapsedMs':row['elapsedMs'],'nodeSpansMs':parts})
        result['groups'][key]={'originalSummary':data['interactionSummary'][key],'distributions':{k:stats(v) for k,v in spans.items()},'fiveLargest':sorted(largest,key=lambda r:r['elapsedMs'],reverse=True)[:5]}
    tracepath=path/'action-only-trace.zip'  # Read-only; retained raw trace stays private.
    with zipfile.ZipFile(tracepath) as archive:
        members={row.filename:row.file_size for row in archive.infolist()};assert set(members)=={'trace.trace','trace.network','trace.stacks'}
        assert sum(members.values())<8*1024*1024,'Bounded trace extraction'
        rows=[json.loads(line) for line in archive.read('trace.trace').splitlines()]
        indicators={name:sum(len(re.findall(pattern,archive.read(member),re.I)) for member in members) for name,pattern in [('authorization-field',rb'authorization'),('bearer-scheme',rb'bearer\s'),('password-field',rb'password'),('bootstrap-token-field',rb'bootstrap.?token'),('api-key-field',rb'api.?key'),('access-refresh-token-field',rb'(?:access|refresh)[_-]?token')]}
    trace={'privateRaw':arm=='actual','rawSHA256':sha(tracepath.read_bytes()),'archiveMemberBytes':members,'credentialIndicatorScan':indicators,'scanLimitation':'Indicator scan only; absence does not establish absence of credentials. Actual raw archive is private. Output contains only categorical operation names, numeric durations/boxes and counts.','eventTypes':dict(Counter(row['type'] for row in rows)),'clickKinds':{}}
    calls=defaultdict(list)
    for row in rows:
        if row.get('callId'):calls[row['callId']].append(row)
    click_groups=defaultdict(list);method_durations=defaultdict(list)
    for events in calls.values():
        before=next((r for r in events if r['type']=='before'),None);after=next((r for r in events if r['type']=='after'),None)
        if not before or not after:continue
        # Protocol method names only; parameters and return values never leave.
        method=before['class']+'.'+before['method'];method_durations[method].append(after['endTime']-before['startTime'])
        if before['method']!='click':continue
        logs=[r for r in events if r['type']=='log'];kind=click_kind(before['params'].get('selector',''));assert kind!='other-click'
        item={'totalMs':after['endTime']-before['startTime'],'attempts':sum(r['message']=='attempting click action' for r in logs),'retries':dict(Counter(value for r in logs if (value:=retry_class(r['message']))))}
        stages={}
        for needle,label in [('waiting for element to be visible, enabled and stable','stable-wait'),('element is visible, enabled and stable','stable'),('scrolling into view if needed','scroll-start'),('done scrolling','scroll-end'),('performing click action','perform-start'),('click action done','perform-end'),('waiting for scheduled navigations to finish','navigation-start'),('navigations have finished','navigation-end')]:
            matches=[r['time'] for r in logs if r['message'].strip()==needle]
            if len(matches)==1:stages[label]=matches[0]
        item['segmentsMs']={}
        locator_wait=next((r['time']for r in logs if r['message'].startswith('waiting for ')),None)
        locator_ready=next((r['time']for r in logs if 'locator resolved to'in r['message']),None)
        if locator_wait is not None and locator_ready is not None:item['segmentsMs']['locatorResolution']=locator_ready-locator_wait
        for label,start,end in [('stableWait','stable-wait','stable'),('scroll','scroll-start','scroll-end'),('performClick','perform-start','perform-end'),('navigationWait','navigation-start','navigation-end')]:
            if start in stages and end in stages:item['segmentsMs'][label]=stages[end]-stages[start]
        if 'stable-wait' in stages:item['segmentsMs']['beforeStableWait']=stages['stable-wait']-before['startTime']
        if 'navigation-end'in stages:item['segmentsMs']['afterNavigationWait']=after['endTime']-stages['navigation-end']
        action=next((r for r in events if r['type']=='input'),None)
        if action:item['box']={k:action['box'][k] for k in ['x','y','width','height']}
        click_groups[kind].append(item)
    for kind,items in click_groups.items():
        segments=defaultdict(list)
        for item in items:
            for key,value in item['segmentsMs'].items():segments[key].append(value)
        trace['clickKinds'][kind]={'clicks':len(items),'attempts':sum(r['attempts'] for r in items),'retryCounts':dict(sum((Counter(r['retries'])for r in items),Counter())),'total':stats([r['totalMs']for r in items]),'segmentDistributions':{k:stats(v)for k,v in segments.items()},'observations':items}
    trace['protocolMethodDistributions']={method:stats(values)for method,values in method_durations.items()}
    result['trace']=trace
    resources=defaultdict(list);last={}
    for window in data['browserEventWindows']:
        latency=window['latency'];last[latency['timeOrigin']]=latency
        for row in latency['rows']:
            if row['kind']=='resource':resources[row['route']].append(row['duration'])
    result['resourceClassDurations']={k:stats(v)for k,v in resources.items()}
    documents=defaultdict(dict)
    for window in data['browserEventWindows']:
        for row in window['latency']['rows']:documents[window['timeOrigin']][row['sequence']]=row
    primary=[r for r in data['interactions']if r['kind']=='project-switch'and'followup'not in r['phase']]
    windows=[]
    for index,row in sorted(enumerate(primary),key=lambda item:item[1]['elapsedMs'],reverse=True)[:5]:
        refs=row['diagnostics']['browserFrames']['references'];document=documents[refs['timeOrigin']];opened=refs['pointer'];expected=refs['expectedDOM']
        pointers=sorted((r for r in document.values()if r['kind']=='pointer-dispatch'and r['sequence']<=opened['sequence']),key=lambda r:r['sequence']);first=pointers[-2]
        observed=[r for r in document.values()if r['kind']=='resource'and first['at']<=r['at']<=expected['at']]
        windows.append({'index':index,'elapsedMs':row['elapsedMs'],'browserAllPointerToOpenPointerMs':opened['at']-first['at'],'browserOpenPointerToExpectedDOMMs':expected['at']-opened['at'],'resources':[{'routeClass':r['route'],'durationMs':r['duration'],'status':r['responseStatus'],'completedRelativeToExpectedDOMMs':r['responseEnd']-expected['at']}for r in observed]})
    result['largestPrimaryNetworkWindows']={'scope':'Known completed resources starting between the two latest pointer events and expected DOM, same browser clock only. Sequence follows unchanged two-click action, without explicit causal IDs or aborted/inflight coverage.','windows':windows}
    result['browserCollectorPartialCost']={'documents':len(last),'callbacks':sum(r['callbacks']for r in last.values()),'callbackMs':sum(r['callbackMs']for r in last.values()),'scope':'Final cumulative per-document callback body self-time. Not total collector/transport/observer overhead.'}
    output['arms'][arm]=result
(OUT/'comparison.json').write_text(json.dumps(output,indent=2)+'\n')
print(json.dumps({'output':str(OUT/'comparison.json'),'sha256':sha((OUT/'comparison.json').read_bytes()),'actualTracePrivate':True,'runtimeExecuted':False}))
