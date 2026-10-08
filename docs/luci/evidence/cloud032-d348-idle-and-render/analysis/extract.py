#!/usr/bin/env python3
"""Read existing numerical collectors; never opens the private browser trace."""
import hashlib,json,math
from collections import defaultdict
from pathlib import Path
BASE=Path('/workspace/.luci-tools/cloud032-critical-path-d348')
RUN=Path('/workspace/.luci-tools/cloud032-idle-d348f012/runs/actual-idle-after-1')
EXPECTED_SOURCE='d348f0127a920aab05b54a4192baa8ed3800fe54'
EXPECTED_DRIVER='d7fb7586e5b4a2639da9abb6c2e1c6a1296d2161ea7cf2ec393a8f1d3ab1b016'
EXPECTED_INTERACTION='ec0bc9d06fda6636cbba6975e05b9c66979ca189b59aa38c373b37a318b79502'
sha=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
j=json.loads((RUN/'result.json').read_text())
assert j['sourceCommit']==EXPECTED_SOURCE and j['sourceDirty'] is False
assert j['driverSHA256']==EXPECTED_DRIVER
assert j['helperSHA256']['interaction-metrics.mjs']==EXPECTED_INTERACTION
assert sha(RUN/'accepted-harness/sustained.mjs')==EXPECTED_DRIVER
assert sha(RUN/'accepted-harness/interaction-metrics.mjs')==EXPECTED_INTERACTION
assert j['latencyTargets']['projectSwitchP95Ms']==300 and j['latencyTargets']['inputP95Ms']==100
assert j['idleDiagnosticCompleted'] and j['originalReceiptComplete']

def stats(vs):
    vs=sorted(vs)
    if not vs:return {'count':0}
    return {'count':len(vs),'p50Ms':vs[math.ceil(len(vs)*.5)-1],'p95Ms':vs[math.ceil(len(vs)*.95)-1],'p99Ms':vs[math.ceil(len(vs)*.99)-1],'maxMs':vs[-1]}

def spans(r):
    marks={m['stage']:m['offsetMs']for m in r['diagnostics']['nodeStages']}
    return {label:marks[end]-marks[start]for label,start,end in [('action','action-start','action-end'),('verify','verify-start','verify-end'),('finalFrameRPC','frames-start','frames-end'),('allProjectsClick','action-start','all-projects-click-end'),('overviewHeading','all-projects-click-end','overview-heading-ready'),('openProjectClick','overview-heading-ready','open-project-click-end')]if start in marks and end in marks}

def event_projection(e,t):
    return {'durationMs':e['duration'],'startRelativeDispatchMs':e['startTime']-t,'queueMs':e['processingStart']-e['startTime'],'handlerMs':e['processingEnd']-e['processingStart']}

events=defaultdict(list)
for w in j['browserEventWindows']:events[w['timeOrigin']].extend(w['events'])
primary=[r for r in j['interactions']if r['phase']=='idle-interactive-calibration']
assert len(primary)==400 and all(r['success']for r in primary)
assert [r['kind']for r in primary]==['project-switch','input','input','draft-reset']*100
# The immutable collector gives these deterministic positional input meanings.
input_rows=[]
for index,r in enumerate(r for r in primary if r['kind']=='input'):
    refs=r['diagnostics']['browserFrames']['references'];t=refs['input']['at']
    # A captured input listener runs within the PerformanceEventTiming dispatch.
    # +/-0.2 ms permits browser clock quantization; require one unique match.
    matches=[e for e in events[refs['timeOrigin']]if e['name']=='input'and e['processingStart']-.2<=t<=e['processingEnd']+.2]
    input_rows.append({'index':index,'actionLabel':'type-x'if index%2==0 else 'backspace','elapsedMs':r['elapsedMs'],'nodeSpansMs':spans(r),'dispatchToExpectedDOMMs':refs['expectedDOM']['at']-t,'eventMatchCount':len(matches),'event':event_projection(matches[0],t)if len(matches)==1 else None})
input_groups={}
for label in ['type-x','backspace']:
    rows=[r for r in input_rows if r['actionLabel']==label];matched=[r for r in rows if r['event']]
    input_groups[label]={'primaryElapsed':stats([r['elapsedMs']for r in rows]),'nodeAction':stats([r['nodeSpansMs']['action']for r in rows]),'uniqueEventMatches':len(matched),'unmatchedIndices':[r['index']for r in rows if not r['event']],'inputEventHandler':stats([r['event']['handlerMs']for r in matched]),'inputEventQueue':stats([r['event']['queueMs']for r in matched])}
compact_rows=[]
for index,r in enumerate(primary):
    compact_rows.append({'index':index,'kind':r['kind'],'success':r['success'],'elapsedMs':r['elapsedMs'],'nodeSpansMs':spans(r),'browserFrames':r['diagnostics']['browserFrames']})
out={'version':1,'scope':'Read-only numerical extraction from the existing d348 diagnostic; no runtime, browser, tests, or measurement rerun. No private raw trace read/copied.','limitations':['Original 100-switch/200-input primary totals and thresholds unchanged. Draft-reset100 retained separately.','Node spans include transport and scheduling. Browser and Node clocks are not subtracted. Quantiles are not additive.','EventTiming is thresholded at16ms and browser duration is rounded; absent matches are not zero. Nested events and handlers are not additive.','Input-event matching associates dispatch by one same-document handler interval, not React function attribution.','No longtask or React profiler data exist in this run. Source-supported optimizations have no measured latency attribution.','Different original4df and d348 runs are sequential diagnostics; no causal subtraction or claim of matched external load.'], 'binding':{'sourceCommit':j['sourceCommit'],'resultSHA256':sha(RUN/'result.json'),'driverSHA256':j['driverSHA256'],'interactionSHA256':j['helperSHA256']['interaction-metrics.mjs'],'latencyHelperSHA256':j['helperSHA256']['latency-diagnostics.mjs'],'binarySHA256':j['binarySHA256'],'webIndexSHA256':j['webIndexSHA256'],'packageArchiveSHA256':j['packageIntegrity']['archiveSHA256'],'browserVersion':j['browserVersion']},'targets':j['latencyTargets'],'targetComparison':j['targetComparison'],'primarySummary':{k:v for k,v in j['interactionSummary'].items()if k.startswith('idle-interactive-calibration:')},'inputMatching':{'rule':'Input handler interval contains captured trusted-input dispatch timestamp, with0.2ms clock tolerance; require exactly one match.','groups':input_groups,'rows':input_rows},'primaryRows':compact_rows,'originalRunOutcome':{'passed':j['passed'],'meaning':'Diagnostic completion only; both latency targets failed.','diagnosticOnly':j['diagnosticOnly'],'fullSustainedAcceptance':j['fullSustainedAcceptance'],'runtimeCleanup':j['runtimeCleanup'],'fixtureRemoved':j['fixtureRemoved'],'dockerRequestsDrained':j['dockerRequestsDrained'],'dockerMutationStateCertain':j['dockerMutationStateCertain']}}
(BASE/'critical-stage-evidence.json').write_text(json.dumps(out,indent=2)+'\n')
print(json.dumps({'output':str(BASE/'critical-stage-evidence.json'),'sha256':sha(BASE/'critical-stage-evidence.json'),'primaryActions':len(primary),'inputGroups':input_groups},indent=2))
